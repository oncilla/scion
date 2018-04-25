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

import (
	"fmt"

	"github.com/scionproto/scion/go/lib/assert"
	"github.com/scionproto/scion/go/lib/common"
)

var _ common.Extension = (*SteadyExtn)(nil)

const (
	InvalidPathLens = "Invalid steady path lengths"

	// SteadyIDLen is the length of the steady reservation id.
	SteadyIDLen = 8
)

// SteadyExtn is the SIBRA Steady path extension header.
//
// Steady paths are long-lived reservations setup by ASes, to provide
// guarantees about bandwidth availability to their customers. The setup packet
// travels along a normal SCION path, and only after it's successful do the
// packets switch to using the (newly-created) SIBRA path. Steady paths only
// have a single path ID.
type SteadyExtn struct {
	*BaseExtn
	ResvBlock *ResvBlock
	ReqBlock  *SteadyReqBlock
}

func SteadyExtnFromRaw(raw common.RawBytes) (*SteadyExtn, error) {
	base, err := BaseExtnFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return SteadyExtnFromBase(base, raw)
}

func SteadyExtnFromBase(base *BaseExtn, raw common.RawBytes) (*SteadyExtn, error) {
	if base.PathLens[1] != 0 || base.PathLens[2] != 0 {
		return nil, common.NewBasicError(InvalidPathLens, nil, "expected", "(x,0,0)",
			"actual", fmt.Sprintf("(x,%d,%d)", base.PathLens[1], base.PathLens[2]))
	}
	s := &SteadyExtn{BaseExtn: base}
	off, end := common.ExtnFirstLineLen, common.ExtnFirstLineLen+SteadyIDLen
	s.ResvIDs = append(s.ResvIDs, append(ResvID(nil), raw[off:end]...))
	off = end
	if err := s.updateIndices(); err != nil {
		return nil, err
	}
	// If this is not a setup request there is one active reservation block.
	if s.PacketType != PacketTypeSetup {
		end += calcResvBlockLen(s.totalHops)
		if err := s.parseActiveResvBlock(raw[off:end], s.totalHops); err != nil {
			return nil, err
		}
	}
	// Parse the remains according to the packet type.
	switch s.PacketType {
	case PacketTypeData:
		return parseData(s, end, raw)
	case PacketTypeTearDown:
		return parseTeardown(s)
	default:
		return parseRequest(s, raw[end:])
	}
}

func parseData(s *SteadyExtn, end int, raw common.RawBytes) (*SteadyExtn, error) {
	if end != len(raw) {
		return nil, common.NewBasicError(InvalidExtnLength, nil, "type", s.PacketType,
			"expected", end, "actual", len(raw))
	}
	return s, nil
}

func parseTeardown(s *SteadyExtn) (*SteadyExtn, error) {
	return nil, common.NewBasicError("Not supported", nil, "type", s.PacketType)
}

func parseRequest(s *SteadyExtn, raw common.RawBytes) (*SteadyExtn, error) {
	var err error
	// If the request is accepted and on the way to the reservation initiator,
	// it contains a reservation block. I.e. fwd==true for Down and Peering-Down
	// paths and fwd==false for all other paths.
	if s.Accepted && (s.Forward == s.Down) {
		if s.ResvBlock, err = ResvBlockFromRaw(raw, s.totalHops); err != nil {
			return nil, err
		}
		return s, nil
	}
	// In any other case, it contains a steady request block
	if s.ReqBlock, err = SteadyReqBlockFromRaw(raw, s.totalHops); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SteadyExtn) Pack() (common.RawBytes, error) {
	return nil, nil
}

func (s *SteadyExtn) Copy() common.Extension {
	raw, err := s.Pack()
	if assert.On {
		assert.Must(err == nil, "Packing must not fail")
	}
	c, err := SteadyExtnFromRaw(raw)
	if assert.On {
		assert.Must(err == nil, "Parsing must not fail")
	}
	return c
}

func (s *SteadyExtn) String() string {
	return fmt.Sprintf("sibra.SteadyExtn (%dB)", s.Len())
}

/*
func NewSteadySetup(exp Tick, bw BWPair, pathID ResvID, numHops uint8) (*SteadyExtn, error) {
	if len(pathID) != SteadyIDLen {
		return nil, common.NewBasicError("Invalid Steady ResvID length", nil,
			"expected", SteadyIDLen, "actual", len(pathID))
	}
	resvBlockLen := calcResvBlockLen(int(numHops))
	l := common.ExtnFirstLineLen + SteadyIDLen + resvBlockLen
	s := &SteadyExtn{
		&BaseExtn{
			raw:         make(common.RawBytes, l),
			SOFIndex:    0,
			PathLens:    []uint8{numHops, 0, 0},
			ResvIDs:     make([]ResvID, 1),
			ActiveResvs: make([]*ResvBlock, 0),
			Setup:       true,
			Request:     true,
			Accepted:    true,
			Down:        false,
			Steady:      true,
			Forward:     true,
			Version:     Version}}
	s.updateIndices()
	off, end := common.ExtnFirstLineLen, common.ExtnFirstLineLen+SteadyIDLen
	copy(s.raw[off:end], pathID)
	off, end = end, end+resvBlockLen
	var err error
	if s.ResvRequest, err = ResvBlockFromRaw(s.raw[off:end], int(numHops)); err != nil {
		return nil, err
	}
	s.ResvRequest.Info.Forward = true
	s.ResvRequest.Info.BwPair = bw
	s.ResvRequest.Info.ExpTick = exp
	s.write()
	return s, nil
}*/
