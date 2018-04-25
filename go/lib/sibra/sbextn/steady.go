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
	"fmt"

	"github.com/scionproto/scion/go/lib/assert"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

var _ common.Extension = (*Steady)(nil)

const InvalidSteadyIdLen = "Invalid steady reservation id length"

// Steady is the SIBRA steady reservation extension header.
type Steady struct {
	*Base
}

func SteadyFromRaw(raw common.RawBytes) (*Steady, error) {
	base, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return SteadyFromBase(base, raw)
}

func SteadyFromBase(base *Base, raw common.RawBytes) (*Steady, error) {
	s := &Steady{Base: base}
	off, end := 0, common.ExtnFirstLineLen
	for i := 0; i < s.TotalSteady; i++ {
		off, end = end, end+sbresv.SteadyIDLen
		s.ParseID(raw[off:end])
	}
	off = end + padding(end+common.ExtnSubHdrLen)
	if !s.Setup {
		for i := 0; i < s.TotalSteady; i++ {
			if err := s.parseActiveBlock(raw[off:], int(s.PathLens[i])); err != nil {
				return nil, err
			}
			off += s.ActiveBlocks[i].Len()
		}
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	switch {
	case s.IsRequest:
		if err := s.parseRequest(raw[off:]); err != nil {
			return nil, common.NewBasicError("Unable to parse request", err,
				"raw", raw, "off", off, "len", len(raw)-off)
		}
		return s, nil
	case s.BestEffort:
		if off != len(raw) {
			return nil, common.NewBasicError(InvalidExtnLength, nil,
				"extn", s, "expected", off, "actual", len(raw))
		}
		return s, nil
	default:
		return nil, common.NewBasicError("Steady traffic must be request or best effort", nil)
	}
}

func (s *Steady) validate() error {
	if !s.Steady {
		return common.NewBasicError("Base not steady", nil)
	}
	if err := s.ValidatePath(); err != nil {
		return err
	}
	return nil
}

// ValidatePath validates the the path types are compatible at the transfer hops.
func (s *Steady) ValidatePath() error {
	if len(s.ActiveBlocks) == 0 && s.Setup {
		return nil
	}
	if len(s.ActiveBlocks) < 1 || len(s.ActiveBlocks) > 3 {
		return common.NewBasicError("Invalid number of active blocks", nil,
			"num", len(s.ActiveBlocks))
	}
	prevPT := sbresv.PathTypeNone
	for i, v := range s.ActiveBlocks {
		if !v.Info.PathType.ValidAfter(prevPT) {
			return common.NewBasicError("Incompatible path types", nil, "blockIdx", i,
				"prev", prevPT, "curr", v.Info.PathType)
		}

		prevPT = v.Info.PathType
	}
	return nil
}

// ToRequest modifies the steady extension and adds the provided request.
func (s *Steady) ToRequest(r sbreq.Request) error {
	if s.Steady && s.Setup {
		return common.NewBasicError("Steady setup requests cannot be transformed", nil)
	}
	if r.Steady() && r.NumHops() != s.ActiveBlocks[0].NumHops() {
		return common.NewBasicError("NumHops in SOFields and request mismatch", nil,
			"offer", r.NumHops(), "sof", s.ActiveBlocks[0].NumHops())
	}
	if !r.Steady() && r.NumHops() != s.TotalHops {
		return common.NewBasicError("NumHops in SOFields and request mismatch", nil,
			"offer", r.NumHops(), "sof", s.TotalHops)
	}
	s.IsRequest = true
	s.Request = r
	s.BestEffort = false
	return nil
}

func (s *Steady) Copy() common.Extension {
	raw, err := s.Pack()
	if assert.On {
		assert.Must(err == nil, "Packing must not fail")
	}
	c, err := SteadyFromRaw(raw)
	if assert.On {
		assert.Must(err == nil, "Parsing must not fail")
	}
	return c
}

func (s *Steady) Reverse() (bool, error) {
	if s.Request != nil {
		rev, err := s.Request.Reverse()
		if err != nil {
			return false, common.NewBasicError("Unable to reverse steady extension", err)
		}
		s.Request = rev
	}
	return s.Base.Reverse()
}

func (s *Steady) String() string {
	return fmt.Sprintf("sbextn.Steady (%dB): %s", s.Len(), s.IDs)
}
