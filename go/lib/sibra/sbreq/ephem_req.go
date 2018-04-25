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

package sbreq

import (
	"fmt"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

const offEphemId = common.LineLen

var _ Request = (*EphemReq)(nil)

// EphemReq is the SIBRA request block for ephemeral reservations. It contains
// an ephemeral id, if it is a setup request.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | base   |          padding                                             |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Ephemeral ID (opt)													   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation Block													   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type EphemReq struct {
	*Base
	// ReqID is the requested ephemeral reservation id set in setup requests.
	ReqID sbresv.ID
	// Block is the reservation block.
	Block *sbresv.Block
}

func EphemReqFromRaw(raw common.RawBytes, numHops int) (*EphemReq, error) {
	b, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return EphemReqFromBase(b, raw, numHops)
}

func EphemReqFromBase(b *Base, raw common.RawBytes, numHops int) (*EphemReq, error) {
	if b.Type != REphmSetup && b.Type != REphmRenewal {
		return nil, common.NewBasicError("Invalid request type", nil, "type", b.Type)
	}
	if !b.Accepted {
		return nil, common.NewBasicError("Must be accepted", nil)
	}
	// Make sure we don't panic when accessing raw[end:]
	min := offEphemId + common.LineLen
	if b.Type == REphmSetup {
		min += sbresv.EphemIDLen
	}
	if len(raw) < min {
		return nil, common.NewBasicError("Invalid ephemeral reservation request length", nil,
			"min", min, "actual", len(raw))
	}
	resvReq := &EphemReq{
		Base: b,
	}
	off, end := 0, offEphemId
	if b.Type == REphmSetup {
		off, end = end, end+sbresv.EphemIDLen
		resvReq.ReqID = sbresv.ID(raw[off:end])
	}
	var err error
	resvReq.Block, err = sbresv.BlockFromRaw(raw[end:], numHops)
	if err != nil {
		return nil, err
	}
	return resvReq, nil
}

func NewEphemReq(t RequestType, id sbresv.ID, info *sbresv.Info, numhops int) *EphemReq {
	c := &EphemReq{
		Base: &Base{
			Type:     t,
			Accepted: true,
		},
		ReqID: id,
		Block: sbresv.NewBlock(info, numhops),
	}
	return c
}

func (r *EphemReq) Fail(code FailCode, maxBw sbresv.BwCls, failHop int) *EphemFailed {
	rep := &EphemFailed{
		Base: &Base{
			Type:     r.Type,
			Accepted: false,
		},
		FailCode: code,
		LineLen:  uint8(r.Len() / common.LineLen),
		ReqID:    r.ReqID,
		Info:     r.Block.Info.Copy(),
		Offers:   make([]sbresv.BwCls, r.NumHops()),
	}
	rep.Info.FailHop = uint8(failHop)
	rep.Info.BwCls = maxBw
	for i := 0; i < failHop; i++ {
		rep.Offers[i] = r.Block.Info.BwCls
	}
	return rep
}

func (r *EphemReq) EphemID() sbresv.ID {
	return r.ReqID
}

func (r *EphemReq) Steady() bool {
	return false
}

func (r *EphemReq) NumHops() int {
	return r.Block.NumHops()
}

func (r *EphemReq) Len() int {
	l := offEphemId
	if r.Type == REphmSetup {
		l += sbresv.EphemIDLen
	}
	return l + r.Block.Len()
}

func (r *EphemReq) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.EphemReq.Write",
			"min", r.Len(), "actual", len(b))
	}
	if err := r.Base.Write(b); err != nil {
		return err
	}
	off, end := 0, offEphemId
	if r.Type == REphmSetup {
		off, end = end, end+sbresv.EphemIDLen
		r.ReqID.Write(b[off:end])
	}
	if err := r.Block.Write(b[end:]); err != nil {
		return err
	}
	return nil
}

func (r *EphemReq) Reverse() (Request, error) {
	if r.Response {
		return nil, common.NewBasicError("Reversing not supported", nil,
			"response", r.Response, "accepted", r.Accepted)
	}
	r.Response = true
	return r, nil
}

func (r *EphemReq) String() string {
	return fmt.Sprintf("Base: [%s] ReqID: [%s] Block: %s", r.Base, r.ReqID, r.Block)

}
