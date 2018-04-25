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

package sbextn

import (
	"time"

	"hash"

	"fmt"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

const (
	// Version is the SIBRA Version number.
	// It is a 2-bit value to be used as (SCION ver, SIBRA ver).
	Version byte = 0

	InvalidExtnLength  = "Invalid extension length"
	InvalidSetupFlag   = "Setup must be steady and request"
	UnsupportedVersion = "Unsupported SIBRA version"
	UnableSetMac       = "Unable to set mac"

	flagSteady     = 0x80
	flagSetup      = 0x40
	flagForward    = 0x20
	flagBestEffort = 0x10
	flagRequest    = 0x08
	flagVersion    = 0x03

	offFlags    = 0
	offPathLens = 2
	OffSOFIndex = 1
)

// Base is the basis for steady and ephemeral extensions.
//
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
// - steady (MSB)
// - setup
// - forward
// - bestEffort
// - request
// - (reserved)
// - version (2 LSB)
type Base struct {
	// ReqID is set to the reservation id associated with the request, if any.
	ReqID sbresv.ID
	// CurrHop is the current hop on the path. Transfer ASes count as one hop.
	CurrHop int
	// TotalHops is the number of traversed ASes. Transfer ASes count as one hop.
	TotalHops int
	// CurrSteady indicates which steady reservation is associated with the current
	// hop. At transfer Hops, this is the smaller of the two indexes.
	CurrSteady int
	// TotalSteady is the number of steady reservations.
	TotalSteady int
	// RelSteadyHop is the index inside the current steady reservation.
	RelSteadyHop int
	// CurrBlock is the index of the current active reservation block.
	CurrBlock int
	// RelSOFIdx ist the index of the SOF in the current active reservation block
	RelSOFIdx int

	// PathLens indicates how long each steady reservation segment is. Ephemeral
	// reservation keep the same value as the stitched segments during setup.
	PathLens []uint8
	// IDs holds up to 4 path IDs. They are directly mapped to underlying buffer.
	// Steady setup and renewal requests contain only one id. Ephemeral setup
	// requests contain the steady reservation ids of the active reservation blocks.
	// Ephemeral extensions contain the ephemeral id plus the steady reservation ids.
	IDs []sbresv.ID
	// ActiveBlocks holds up to 3 active reservations blocks.
	// In case of an ephemeral extension, exactly one active block must be present.
	ActiveBlocks []*sbresv.Block
	// Request contains a request if the IsRequest flag is set.
	Request sbreq.Request

	// Version is the SIBRA version.
	Version uint8
	// SOFIndex indicates the current SIBRA Opaque Field.
	SOFIndex uint8
	// Steady indicates if this packet is sent on a steady or ephemeral reservation.
	// Ephemeral setup requests are sent using one or multiple steady reservations.
	Steady bool
	// Setup indicates if this is a steady setup request. If this is the case,
	// the packet travels on a regular spath and shall be used for forwarding.
	Setup bool
	// Forward indicates if the packet is travelling from reservation start to end.
	Forward bool
	// BestEffort indicates if the packet is considered best effort. In case forward
	// is set, this flag must be set as well.
	BestEffort bool
	// IsRequest indicates if the SIBRA header contains a request.
	IsRequest bool
}

// BaseFromRaw parses the first line of raw in order to distinguish if it is a
// steady or ephemeral extension.
func BaseFromRaw(raw common.RawBytes) (*Base, error) {
	b := &Base{}
	if err := b.parseFlags(raw[offFlags]); err != nil {
		return nil, err
	}
	if err := b.parsePathLens(raw[offPathLens : offPathLens+3]); err != nil {
		return nil, err
	}
	b.SOFIndex = raw[OffSOFIndex]
	if err := b.UpdateIndices(); err != nil {
		return nil, err
	}
	if err := b.checkMinLen(raw); err != nil {
		return nil, err
	}
	b.IDs = make([]sbresv.ID, 0, 4)
	b.ActiveBlocks = make([]*sbresv.Block, 0, 3)
	return b, nil
}

// parseFlags parses the flags.
func (e *Base) parseFlags(flags byte) error {
	e.Steady = (flags & flagSteady) != 0
	e.Setup = (flags & flagSetup) != 0
	e.Forward = (flags & flagForward) != 0
	e.BestEffort = (flags & flagBestEffort) != 0
	e.IsRequest = (flags & flagRequest) != 0
	e.Version = flags & flagVersion
	if e.Version != Version {
		return common.NewBasicError(UnsupportedVersion, nil, "expected", Version,
			"actual", e.Version)
	}
	if e.Setup && (!e.Steady || !e.IsRequest) {
		return common.NewBasicError(InvalidSetupFlag, nil, "steady", e.Steady,
			"isReq", e.IsRequest)
	}
	return nil
}

func (e *Base) parsePathLens(raw common.RawBytes) error {

	if raw[0] == 0 {
		return common.NewBasicError("PathLens of format (0xx) not allowed", nil,
			"P0", raw[0], "P1", raw[1], "P2", raw[2])
	}
	if raw[1] == 0 && raw[2] != 0 {
		return common.NewBasicError("PathLens of format (x0x) not allowed", nil,
			"P0", raw[0], "P1", raw[1], "P2", raw[2])
	}
	e.PathLens = raw
	return nil
}

// checkMinLen checks that the raw buffer is at least the required size for ids and
// active reservation blocks.
func (e *Base) checkMinLen(raw common.RawBytes) error {
	l := common.ExtnFirstLineLen
	if !e.Steady {
		l += sbresv.EphemIDLen
	}
	l += sbresv.SteadyIDLen * e.TotalSteady
	l += padding(l + common.ExtnSubHdrLen)
	if !e.Setup && e.Steady {
		l += sbresv.InfoLen * e.TotalSteady
		l += sbresv.SOFieldLen * int(e.PathLens[0]+e.PathLens[1]+e.PathLens[2])
	} else if !e.Steady {
		l += sbresv.InfoLen
		l += sbresv.SOFieldLen * e.TotalHops
	}
	if len(raw) < l {
		return common.NewBasicError("Raw is smaller than minimum length", nil,
			"plen", e.PathLens, "expected", l, "actual", len(raw))
	}
	return nil
}

// ParseID casts a reservation id from raw and appends it to the IDs slice.
func (e *Base) ParseID(raw common.RawBytes) {
	e.IDs = append(e.IDs, sbresv.ID(raw))
}

// parseActiveBlock parses an active reservation block and appends it to the
// ActiveBlocks slice.
func (e *Base) parseActiveBlock(raw common.RawBytes, numHops int) error {
	block, err := sbresv.BlockFromRaw(raw, numHops)
	if err != nil {
		return err
	}
	e.ActiveBlocks = append(e.ActiveBlocks, block)
	return nil
}

// parseRequest parses the request and sets ReqID. Requests are at the end,
// thus raw must be completely parsed, otherwise an error is thrown.
func (e *Base) parseRequest(raw common.RawBytes) error {
	r, err := sbreq.Parse(raw, e.TotalHops)
	if err != nil {
		return err
	}
	if r.Len() != len(raw) {
		return common.NewBasicError(InvalidExtnLength, nil,
			"len", len(raw), "unparsed", len(raw)-e.Request.Len())
	}
	e.Request = r
	e.ReqID = r.EphemID()
	if e.ReqID == nil {
		e.ReqID = e.IDs[0]
	}
	return nil
}

// FirstHop indicates if this is the first hop.
func (e *Base) FirstHop() bool {
	return (!e.Forward && e.CurrHop == e.TotalHops-1) || (e.Forward && e.CurrHop == 0)

}

// LastHop indicates if this is the last hop.
func (e *Base) LastHop() bool {
	return (e.Forward && e.CurrHop == e.TotalHops-1) || (!e.Forward && e.CurrHop == 0)
}

// IsTransfer indicates if active block is switch when NextSOFIdx is called.
// In ephemeral extensions, this is never the case.
func (e *Base) IsTransfer() bool {
	if !e.Steady {
		return false
	}
	if e.Forward {
		return e.CurrSteady < e.TotalSteady-1 && e.RelSteadyHop+1 == int(e.PathLens[e.CurrSteady])
	}
	return e.CurrSteady != 0 && e.RelSteadyHop == 0
}

// Expiry returns the earliest expiration time of all active reservation blocks.
func (e *Base) Expiry() time.Time {
	if len(e.ActiveBlocks) < 1 {
		return time.Time{}
	}
	exp := e.ActiveBlocks[0].Info.ExpTick.Time()
	for i := 1; i < len(e.ActiveBlocks); i++ {
		newExp := e.ActiveBlocks[i].Info.ExpTick.Time()
		if newExp.Before(exp) {
			exp = newExp
		}
	}
	return exp
}

func (e *Base) GetCurrBlock() *sbresv.Block {
	if len(e.ActiveBlocks) == 0 {
		return nil
	}
	return e.ActiveBlocks[e.CurrBlock]
}

func (e *Base) VerifySOF(mac hash.Hash, now time.Time) error {
	pLens := []uint8{e.PathLens[e.CurrSteady]}
	ids := []sbresv.ID{e.IDs[e.CurrSteady]}
	if !e.Steady {
		pLens = e.PathLens
		ids = e.IDs
	}
	return e.GetCurrBlock().Verify(mac, e.RelSOFIdx, ids, pLens, now)
}

func (e *Base) SetSOF(mac hash.Hash, inIFID, egIFID common.IFIDType) error {
	switch r := e.Request.(type) {
	case *sbreq.SteadySucc:
		sof := r.Block.SOFields[e.SOFIndex]
		sof.Ingress = inIFID
		sof.Egress = egIFID
		return r.Block.SetMac(mac, int(e.SOFIndex), e.IDs, e.PathLens)
	case *sbreq.EphemReq:
		sof := r.Block.SOFields[e.CurrHop]
		sof.Ingress = inIFID
		sof.Egress = egIFID
		ids := e.IDs
		if r.ReqID != nil {
			ids = append([]sbresv.ID(nil), r.ReqID)
			ids = append(ids, e.IDs...)
		}
		return r.Block.SetMac(mac, int(e.CurrHop), ids, e.PathLens)
	default:
		return common.NewBasicError(UnableSetMac, nil,
			"err", "packet does not contain reservation response", "request", r)
	}
}

// NextSOFIndex updates the SOFIndex based on the direction implied by Forward.
func (e *Base) NextSOFIndex() error {
	if e.Forward {
		e.SOFIndex += 1
	} else {
		e.SOFIndex -= 1
	}
	return e.UpdateIndices()
}

// UpdateIndices the helper indices based on the SOFIndex field.
func (e *Base) UpdateIndices() (err error) {
	if e.Steady {
		return e.updateIndexesSteady()
	}
	return e.updateIndexesEphem()
}

func (e *Base) updateIndexesSteady() error {
	if err := e.setSteadyIndexes(false); err != nil {
		return err
	}
	e.CurrHop = int(e.SOFIndex) - e.CurrSteady
	e.CurrBlock = e.CurrSteady
	e.RelSOFIdx = e.RelSteadyHop
	return nil
}

func (e *Base) updateIndexesEphem() error {
	if err := e.setSteadyIndexes(true); err != nil {
		return err
	}
	e.CurrHop = int(e.SOFIndex)
	e.CurrBlock = 0
	e.RelSOFIdx = int(e.SOFIndex)
	return nil
}

func (e *Base) setSteadyIndexes(ephem bool) error {
	sumLens := 0
	numBlocks := 0
	for _, l := range e.PathLens {
		sumLens += int(l)
		if l > 0 {
			numBlocks++
		}
	}
	// There are (numBlocks - 1) switches between active reservation blocks.
	e.TotalHops = sumLens - numBlocks + 1
	// Find the current block and the relative SOF index inside that block
	blockIdx, relIdx := 0, int(e.SOFIndex)
	for ; blockIdx < numBlocks && relIdx >= int(e.PathLens[blockIdx]); blockIdx++ {
		relIdx -= int(e.PathLens[blockIdx])
		if ephem {
			relIdx += 1
		}
	}
	if blockIdx >= numBlocks {
		return common.NewBasicError("Invalid SOF index", nil,
			"expected<", sumLens, "actual", e.SOFIndex)
	}
	e.CurrSteady = blockIdx
	e.RelSteadyHop = relIdx
	e.TotalSteady = numBlocks
	return nil
}

// ActiveBlockOff returns the offset of the first active block. This is right after
// the reservation ids.
func (e *Base) ActiveBlockOff() int {
	off := common.ExtnFirstLineLen
	for i := range e.IDs {
		off += e.IDs[i].Len()
	}
	return off + padding(off+common.ExtnSubHdrLen)
}

// padding calculates the padding to the next multiple of common.LineLen.
// WARNING: make sure to account for common.ExtnFirstLineLen.
func padding(bytes int) int {
	return (common.LineLen - bytes%common.LineLen) % common.LineLen
}

func (e *Base) Len() int {
	l := e.ActiveBlockOff()
	for _, resvBlock := range e.ActiveBlocks {
		l += resvBlock.Len()
	}
	if e.Request != nil {
		l += e.Request.Len()
	}
	return l
}

func (e *Base) Class() common.L4ProtocolType {
	return common.HopByHopClass
}

func (e *Base) Type() common.ExtnType {
	return common.ExtnSIBRAType
}

func (e *Base) Pack() (common.RawBytes, error) {
	b := make(common.RawBytes, e.Len())
	return b, e.Write(b)
}

func (e *Base) Write(b common.RawBytes) error {
	if len(b) < e.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "SIBRABaseExtn.Write",
			"min", e.Len(), "actual", len(b))
	}
	b[offFlags] = e.packFlags()
	b[OffSOFIndex] = e.SOFIndex
	off, end := offPathLens, offPathLens+3
	copy(b[off:end], e.PathLens)
	for i := range e.IDs {
		off, end = end, end+e.IDs[i].Len()
		if err := e.IDs[i].Write(b[off:end]); err != nil {
			return err
		}
	}
	// add padding after reservation IDs.
	end = end + padding(end+common.ExtnSubHdrLen)
	for i := range e.ActiveBlocks {
		off, end = end, end+e.ActiveBlocks[i].Len()
		if err := e.ActiveBlocks[i].Write(b[off:end]); err != nil {
			return err
		}
	}
	if e.Request != nil {
		return e.Request.Write(b[end:])
	}
	return nil
}

func (e *Base) packFlags() byte {
	flags := Version & flagVersion
	if e.Steady {
		flags |= flagSteady
	}
	if e.Setup {
		flags |= flagSetup
	}
	if e.Forward {
		flags |= flagForward
	}
	if e.BestEffort {
		flags |= flagBestEffort
	}
	if e.IsRequest {
		flags |= flagRequest
	}
	return flags
}

func (e *Base) Reverse() (bool, error) {
	e.Forward = !e.Forward
	if !e.Forward {
		e.BestEffort = true
	}
	return true, nil
}

func (e *Base) String() string {
	return fmt.Sprintf("IDs %s Steady %t Setup %t Forward %t BestEffort %t "+
		"IsRequest %t Version %d SOFIdx %d PLens %s CurrHop %d TotalHop %d "+
		"CurrSteady %d RelSOFIdx %d TotalSteady %d Blocks %s Request %s",
		e.IDs, e.Steady, e.Setup, e.Forward, e.BestEffort, e.IsRequest, e.Version,
		e.SOFIndex, e.PathLens, e.CurrHop, e.TotalHops, e.CurrSteady, e.RelSteadyHop,
		e.TotalSteady, e.ActiveBlocks, e.Request)
}
