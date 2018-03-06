// Copyright 2018 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sibra

import "github.com/scionproto/scion/go/lib/common"

const (
	SetupAndNoReq      = "Setup flag is set without request flag"
	UnsupportedVersion = "Unsupported SIBRA version"

	flagSetup   = 0x80
	flagRequest = 0x40
	flagAccept  = 0x20
	flagError   = 0x10
	flagSteady  = 0x08
	flagForward = 0x04
	flagVersion = 0x03
)

// BaseExtn is the basis for SIBRA packet extension. This class isn't used directly, but
// via EphemeralExtn and SteadyExtn.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | xxxxxxxxxxxxxxxxxxxxxxxx | Flags  |SOF idx |P0 hops |P1 hops |P2 hops |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | <Path IDs>                                                            |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |...                                                                    |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation block                                                     |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...                                                                   |
// +                                                                       +
//
// The first byte contains the flag field. Its bits are allocated as follows:
// - (MSB) path setup flag:
//      Set if packet is setting up a new SIBRA path.
// - request flag:
//      Set if packet is requesting a reservation, i.e. setup or renewal.
// - accepted flag:
//      Set if the reservation request has been accepted so far.
// - error flag:
//      Set if an error has occurred.
// - steady flag:
//      Set if this is a steady path, unset if this is an ephemeral path.
// - forward flag:
//      Set if packet is travelling src->dest.
// - version (2b): SIBRA version, to be used as (SCION ver, SIBRA ver).
//
// - SOF idx indicates which is the current Sibra Opaque Field. The current hop
// location can be derived from this.
// - P* hops indicate how long each active reservation block is. Summed, they
// indicate the total number of hops in the path.
// - 1-4 Path IDs are used to identify the current path at the current point on
// its travel.
// - There can be multiple reservation blocks - between 0 and 3 active blocks,
// that are used to route the packet, and an optional request block.
type BaseExtn struct {
	// raw is the underlying buffer.
	raw common.RawBytes
	// Setup indicates if this packet is setting up a new SIBRA path.
	Setup bool
	// Request indicates if this packet requests a reservation (both setup and renewal)
	Request bool
	// Accepted indicates if the reservation request has been accepted so far.
	Accepted bool
	// Error indicates if an error has occurred.
	Error bool
	// Steady indicates if this is a steady or an ephemeral path.
	Steady bool
	// Forward indicates if packet is travelling src->dst.
	Forward bool
	// Version is the SIBRA version. It is used as (SCION ver, SIBRA ver).
	Version uint8
	// SOFIndex indicates the current Sibra Opaque Field.
	SOFIndex uint8
	// PathLens indicates how long each active reservation block is.
	PathLens []uint8
	// PathIDs holds up to 4 path IDs. They are directly mapped to raw.
	PathIDs []PathID
	// ResvActive holds up to 3 active reservations blocks.
	ResvActive []*ResvBlock
	// ResvRequest is an optional reservation request.
	ResvRequest *ResvBlock
	// ResvOffer is an optional reservation offer.
	ResvOffer *ResvOffer
	// currHop is the current hop.
	currHop int
	// blockIdx is the index of the current block.
	blockIndex int
	// relSOFIndex is the index of the current SOF inside the current block.
	relSOFIndex int
	// totalHops is the number of all hops.
	totalHops int
}

func BaseExtnFromRaw(raw common.RawBytes) (*BaseExtn, error) {
	b := &BaseExtn{raw: raw}
	if err := b.parseFlags(raw[0]); err != nil {
		return nil, err
	}
	b.SOFIndex = raw[1]
	b.PathLens = make([]uint8, 3)
	copy(b.PathLens, raw[2:5])
	b.PathIDs = make([]PathID, 0, 4)
	b.ResvActive = make([]*ResvBlock, 0, 3)
	return b, nil
}

func (e *BaseExtn) parseFlags(flags uint8) error {
	e.Setup = (flags & flagSetup) != 0
	e.Request = (flags & flagRequest) != 0
	e.Accepted = (flags & flagAccept) != 0
	e.Error = (flags & flagError) != 0
	e.Steady = (flags & flagSteady) != 0
	e.Forward = (flags & flagForward) != 0
	e.Version = flags & flagVersion
	if e.Setup && !e.Request {
		return common.NewBasicError(SetupAndNoReq, nil)
	}
	if e.Version != Version {
		return common.NewBasicError(UnsupportedVersion, nil, "expected",
			Version, "actual", e.Version)
	}
	return nil
}

func (e *BaseExtn) parseActiveResvBlock(raw common.RawBytes, numHops int) error {
	block, err := ResvBlockFromRaw(raw, numHops)
	if err != nil {
		return err
	}
	e.ResvActive = append(e.ResvActive, block)
	return nil
}

func (e *BaseExtn) parseEnd(raw common.RawBytes) error {
	var err error
	if e.Request {
		if e.Accepted {
			e.ResvRequest, err = ResvBlockFromRaw(raw, e.totalHops)
		} else {
			e.ResvOffer, err = ResvOfferFromRaw(raw, e.totalHops)
		}
	} else {
		if len(raw) != 0 {
			err = common.NewBasicError("SIBRA header not parsed completely", nil, "len", len(raw))
		}
	}
	return err
}

func (e *BaseExtn) updateIndices() error {
	e.currHop = int(e.SOFIndex)
	e.relSOFIndex = int(e.SOFIndex)
	e.totalHops = int(e.PathLens[0])
	if int(e.SOFIndex) >= e.totalHops {
		return common.NewBasicError("Invalid SOFIndex", nil, "expected<", e.totalHops,
			"actual", e.SOFIndex)
	}
	return nil
}

func (e *BaseExtn) Pack() (common.RawBytes, error) {
	b := make(common.RawBytes, e.Len())
	if err := e.Write(b); err != nil {
		return nil, err
	}
	return b, nil
}

func (e *BaseExtn) Write(b common.RawBytes) error {
	if len(b) < e.Len() {
		return common.NewBasicError("Buffer too short", nil,
			"method", "SIBRAExtn.Write", "expected min", e.Len(), "actual", len(b))
	}
	e.write()
	copy(b, e.raw)
	return nil
}

// write writes all updates to the underlying buffer.
func (e *BaseExtn) write() {
	e.raw[0] = e.packFlags()
	e.raw[1] = e.SOFIndex
	off, end := 2, 5
	copy(e.raw[off:end], e.PathLens)
	for i := range e.PathIDs {
		off, end = end, end+len(e.PathIDs[i])
	}
	for i := range e.ResvActive {
		off, end = end, end+e.ResvActive[i].Len()
		e.ResvActive[i].write()
	}
	if e.ResvRequest != nil {
		e.ResvRequest.write()
	} else if e.ResvOffer != nil {
		e.ResvOffer.write()
	}
}

func (e *BaseExtn) packFlags() uint8 {
	var flags uint8 = 0
	if e.Setup {
		flags |= flagSetup
	}
	if e.Request {
		flags |= flagRequest
	}
	if e.Accepted {
		flags |= flagAccept
	}
	if e.Error {
		flags |= flagError
	}
	if e.Steady {
		flags |= flagSteady
	}
	if e.Forward {
		flags |= flagForward
	}
	flags |= flagVersion & Version

	return flags
}

func (e *BaseExtn) Len() int {
	return len(e.raw)
}

func (e *BaseExtn) Class() common.L4ProtocolType {
	return common.HopByHopClass
}

func (e *BaseExtn) Type() common.ExtnType {
	return common.ExtnSIBRAType
}

func (e *BaseExtn) Reverse() (bool, error) {
	return true, nil // TODO(roosd): implement
}
