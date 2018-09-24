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
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
)

const InvalidEphemIdLen = "Invalid ephemeral reservation id length"

var _ common.Extension = (*Ephemeral)(nil)

// Ephemeral is the SIBRA ephemeral reservation extension header.
type Ephemeral struct {
	*Base
}

func EphemeralFromRaw(raw common.RawBytes) (*Ephemeral, error) {
	base, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return EphemeralFromBase(base, raw)
}

func EphemeralFromBase(base *Base, raw common.RawBytes) (*Ephemeral, error) {
	e := &Ephemeral{base}
	off, end := common.ExtnFirstLineLen, common.ExtnFirstLineLen+sibra.EphemIDLen
	e.ParseID(raw[off:end])
	for i := 0; i < e.TotalSteady; i++ {
		off, end = end, end+sibra.SteadyIDLen
		e.ParseID(raw[off:end])
	}
	off = end + padding(end+common.ExtnSubHdrLen)
	if err := e.parseActiveBlock(raw[off:], e.TotalHops); err != nil {
		return nil, err
	}
	off += e.ActiveBlocks[0].Len()
	switch {
	case e.IsRequest:
		if err := e.parseRequest(raw[off:]); err != nil {
			return nil, common.NewBasicError("Unable to parse request", err,
				"raw", raw, "off", off, "len", len(raw)-off)
		}
		return e, nil
	default:
		if off != len(raw) {
			return nil, common.NewBasicError(InvalidExtnLength, nil,
				"extn", e, "expected", off, "actual", len(raw))
		}
		return e, nil
	}
}

// SteadyIds returns the steady reservation ids in the reservation direction.
func (e *Ephemeral) SteadyIds() []sibra.ID {
	return e.IDs[1:]
}

// IsSteadyTransfer indicates if the current hop is a transfer hop between two steady reservations.
func (e *Ephemeral) IsSteadyTransfer() bool {
	transFwd := e.CurrSteady < e.TotalSteady-1 && e.RelSteadyHop+1 == int(e.PathLens[e.CurrSteady])
	transRev := e.CurrSteady != 0 && e.RelSteadyHop == 0
	return transFwd || transRev
}

// ToRequest modifies the ephemeral extension and adds the provided request.
func (e *Ephemeral) ToRequest(r sbreq.Request) error {
	if r.Steady() {
		return common.NewBasicError("Steady request not supported", nil, "req", r)
	}
	if !r.Steady() && r.NumHops() != e.TotalHops {
		return common.NewBasicError("NumHops in SOFields and request mismatch", nil,
			"offer", r.NumHops(), "sof", e.TotalHops)
	}
	e.IsRequest = true
	e.Request = r
	e.BestEffort = false
	return nil
}

func (e *Ephemeral) Copy() common.Extension {
	raw, err := e.Pack()
	if assert.On {
		assert.Must(err == nil, "Packing must not fail")
	}
	c, err := EphemeralFromRaw(raw)
	if assert.On {
		assert.Must(err == nil, "Parsing must not fail")
	}
	return c
}

// Reverse reverses the reservation and the request if present.
func (e *Ephemeral) Reverse() (bool, error) {
	if e.Request != nil {
		rev, err := e.Request.Reverse()
		if err != nil {
			return false, common.NewBasicError("Unable to reverse steady extension", err)
		}
		e.Request = rev
	}
	return e.Base.Reverse()
}

func (e *Ephemeral) String() string {
	return fmt.Sprintf("sbextn.Ephemeral (%dB): %s", e.Len(), e.IDs)
}
