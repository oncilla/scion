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

	"github.com/scionproto/scion/go/lib/common"
)

var _ common.Extension = (*SteadyExtn)(nil)

const (
	InvalidPathLens = "Invalid steady path lengths"
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
}

func SteadyExtnFromRaw(raw common.RawBytes) (*SteadyExtn, error) {
	base, err := BaseExtnFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return SteadyExtnFromBase(base)
}

func SteadyExtnFromBase(base *BaseExtn) (*SteadyExtn, error) {
	s := &SteadyExtn{base}
	off, end := common.ExtnFirstLineLen, common.ExtnFirstLineLen+SteadyIDLen
	s.PathIDs = append(s.PathIDs, PathID(s.raw[off:end]))
	if s.PathLens[1] != 0 || s.PathLens[2] != 0 {
		return nil, common.NewBasicError(InvalidPathLens, nil, "expected", "(x,0,0)",
			"actual", fmt.Sprintf("(x,%d,%d)", s.PathLens[1], s.PathLens[2]))
	}
	if err := s.updateIndices(); err != nil {
		return nil, err
	}
	off = end
	// There is exactly one active block, if this is not a setup request.
	if !s.Setup {
		end += calcResvBlockLen(s.totalHops)
		if err := s.parseActiveResvBlock(s.raw[off:end], s.totalHops); err != nil {
			return nil, err
		}
	}
	return s, s.parseEnd(s.raw[end:])
}

func NewSteadySetup(exp Tick, bw BWPair, pathID PathID, numHops uint8) (*SteadyExtn, error) {
	if len(pathID) != SteadyIDLen {
		return nil, common.NewBasicError("Invalid Steady PathID length", nil,
			"expected", SteadyIDLen, "actual", len(pathID))
	}
	resvBlockLen := calcResvBlockLen(int(numHops))
	l := common.ExtnFirstLineLen + SteadyIDLen + resvBlockLen
	s := &SteadyExtn{
		&BaseExtn{
			raw:        make(common.RawBytes, l),
			SOFIndex:   0,
			PathLens:   []uint8{numHops, 0, 0},
			PathIDs:    make([]PathID, 1),
			ResvActive: make([]*ResvBlock, 0),
			Setup:      true,
			Request:    true,
			Accepted:   true,
			Error:      false,
			Steady:     true,
			Forward:    true,
			Version:    Version}}
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
}

func (s *SteadyExtn) Copy() common.Extension {
	raw := make(common.RawBytes, len(s.raw))
	s.Write(raw)
	e, _ := SteadyExtnFromRaw(raw)
	return e
}

func (s *SteadyExtn) String() string {
	return fmt.Sprintf("sibra.SteadyExtn (%dB)", s.Len())
}
