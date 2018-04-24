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
	// Version is the SIBRA Version number.
	// It is a 2bit value to be used as (SCION ver, SIBRA ver).
	Version = 0

	InvalidExtnLength    = "Invalid extension length"
	TeardownAndEphemeral = "Teardown unsupported for ephemeral reservation"
	UnsupportedVersion   = "Unsupported SIBRA version"

	flagSteady  = 0x80
	flagForward = 0x40
	flagAccept  = 0x20
	flagDown    = 0x10
	flagType    = 0x0c
	flagVersion = 0x03

	offsetType = 2
)

// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | xxxxxxxxxxxxxxxxxxxxxxxx | Flags  |SOF Idx |P0 hops |P1 hops |P2 hops |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation IDs (1-4)                                                 |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Active Reservation Tokens (0-3)                                       |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |...                                                                    |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation Request/Response                                          |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...                                                                   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
//
//
// Flags are allocated as follows:
// - (MSB) steady flag:
//    Set if steady reservation, unset if ephemeral.
// - forward flag:
//    Set if packet is travelling src -> dest. (if unset, packet is considered
//    best effort). A request for reservation of a Down or Peering-Down path has
//    this flag unset, since the request is traveling in the reverse direction
//    of the reservation.
// - accepted flag:
//    Set if the reservation request has been accepted so far.
// - Down flag:
//    Set during a steady request if reservation is for a Down or Peering-Down path.
// - type (2b):
//    00 := data packet
//    01 := setup
//    10 := renewal
//    11 := tear down (only allowed if steady flag set)
// - version (2b):
//    SIBRA version, to be used as (SCION ver, SIBRA ver).
//
// SOF Idx indicates which sibra opaque field shall be used forwarding.
//
// P* hops indicate how long each path of the active reservation blocks is.
// Summed, they indicate the total number of hops in the resulting path. If
// this is a steady Reservation (request or usage) P1 and P2 must be 0.
//
// Reservation IDs is a list of multiple Reservation IDs (between 1 and 4).
// The ordering corresponds to that of the active reservation blocks. If this
// is a setup request, the last ID is the Reservation ID associated with the
// reservation request.
//
// Active Reservations Tokens (between 0 and 3) are contain the reservation
// info and the hop fields. If 0 are present, this packet must be a steady
// reservation request.
//
// Reservation Request can either be a request or response block. The format
// is implied by the flags.
type BaseExtn struct {
	// Steady indicates if this is a steady or an ephemeral path.
	Steady bool
	// Forward indicates if packet is travelling reservation src->dst.
	Forward bool
	// Accepted indicates if the reservation request has been accepted so far.
	Accepted bool
	// Down indicates if an error has occurred.
	Down bool
	// PacketType indicates if this is a data, setup, renewal or teardown packet.
	PacketType PacketType
	// Version is the SIBRA version.
	Version uint8
	// SOFIndex indicates the current Sibra Opaque Field.
	SOFIndex uint8
	// PathLens indicates how long each active reservation block is.
	PathLens []uint8
	// ResvIDs holds up to 4 path IDs. They are directly mapped to raw.
	ResvIDs []ResvID
	// ActiveResvs holds up to 3 active reservations blocks.
	ActiveResvs []*ResvBlock
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
	b := &BaseExtn{}
	if err := b.parseFlags(raw[0]); err != nil {
		return nil, err
	}
	b.SOFIndex = raw[1]
	b.PathLens = append([]uint8(nil), raw[2:5]...)
	b.ResvIDs = make([]ResvID, 0, 4)
	b.ActiveResvs = make([]*ResvBlock, 0, 3)
	return b, nil
}

// parseFlags parses the flags
func (e *BaseExtn) parseFlags(flags uint8) error {
	e.Steady = (flags & flagSteady) != 0
	e.Forward = (flags & flagForward) != 0
	e.Accepted = (flags & flagAccept) != 0
	e.Down = (flags & flagDown) != 0
	e.PacketType = PacketType((flags & flagType) >> offsetType)
	e.Version = flags & flagVersion
	if e.Steady && (e.PacketType == PacketTypeTearDown) {
		return common.NewBasicError(TeardownAndEphemeral, nil)
	}
	if e.Version != Version {
		return common.NewBasicError(UnsupportedVersion, nil, "expected", Version,
			"actual", e.Version)
	}
	return nil
}

// parseActiveResvBlock parses an active reservation block and appends it to the ActiveResvs list.
func (e *BaseExtn) parseActiveResvBlock(raw common.RawBytes, numHops int) error {
	block, err := ResvBlockFromRaw(raw, numHops)
	if err != nil {
		return err
	}
	e.ActiveResvs = append(e.ActiveResvs, block)
	return nil
}

// updateIndices updates the currHop, relSOFIndex and total hops for the simple case with
// one active reservation block.
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

func (e *BaseExtn) Write(b common.RawBytes) error {
	if len(b) < e.Len() {
		return common.NewBasicError(BufferToShort, nil, "method", "SIBRABaseExtn.Write",
			"min", e.Len(), "actual", len(b))
	}
	b[0] = e.packFlags()
	b[1] = e.SOFIndex
	off, end := 2, 5
	copy(b[off:end], e.PathLens)
	for i := range e.ResvIDs {
		off, end = end, end+e.ResvIDs[i].Len()
		if err := e.ResvIDs[i].Write(b[off:end]); err != nil {
			return err
		}
	}
	for i := range e.ActiveResvs {
		off, end = end, end+e.ActiveResvs[i].Len()
		if err := e.ActiveResvs[i].Write(b[off:end]); err != nil {
			return err
		}
	}
	return nil
}

func (e *BaseExtn) packFlags() uint8 {
	var flags uint8 = 0
	if e.Steady {
		flags |= flagSteady
	}
	if e.Forward {
		flags |= flagForward
	}
	if e.Accepted {
		flags |= flagAccept
	}
	if e.Down {
		flags |= flagDown
	}
	flags |= uint8(e.PacketType) << offsetType
	flags |= Version
	return flags
}

func (e *BaseExtn) Len() int {
	l := common.ExtnFirstLineLen
	for _, resvID := range e.ResvIDs {
		l += resvID.Len()
	}
	for _, resvBlock := range e.ActiveResvs {
		l += resvBlock.Len()
	}
	return l
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
