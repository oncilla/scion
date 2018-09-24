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
	"hash"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

var _ Data = (*EphemReq)(nil)

// EphemReq is the SIBRA request block for ephemeral reservations. It contains
// an ephemeral id, if it is a setup request.
//
// 0B       1        2        3        4        5        6        7
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
	// ID is the requested ephemeral reservation id set in setup requests.
	ID sibra.ID
	// Block is the reservation block.
	Block *sbresv.Block
}

func EphemReqFromRaw(raw common.RawBytes, setup bool, numHops int) (*EphemReq, error) {
	min := sbresv.InfoLen
	if setup {
		min += sibra.EphemIDLen
	}
	if len(raw) < min {
		return nil, common.NewBasicError("Invalid ephemeral reservation request length", nil,
			"min", min, "actual", len(raw))
	}
	resvReq := &EphemReq{}
	end := 0
	if setup {
		end = sibra.EphemIDLen
		resvReq.ID = sibra.ID(raw[:end])
	}
	var err error
	resvReq.Block, err = sbresv.BlockFromRaw(raw[end:], numHops)
	if err != nil {
		return nil, err
	}
	return resvReq, nil
}

func (r *EphemReq) SetSOF(mac hash.Hash, ids []sibra.ID, plens []uint8,
	inIFID, egIFID common.IFIDType, sofIdx int) error {
	r.Block.SOFields[sofIdx].Ingress = inIFID
	r.Block.SOFields[sofIdx].Egress = egIFID
	return r.Block.SetMac(mac, sofIdx, ids, plens)
}

func (r *EphemReq) Fail(code FailCode, maxBw sibra.BwCls, failHop int) *EphemFailed {
	rep := &EphemFailed{
		ID:       r.ID,
		Info:     r.Block.Info.Copy(),
		DataLen:  uint16(r.Len()),
		FailCode: code,
		FailHop:  uint8(failHop),
		Offers:   make([]sibra.BwCls, r.NumHops()),
	}
	rep.Info.FailHop = uint8(failHop)
	rep.Info.BwCls = maxBw
	for i := 0; i < int(failHop); i++ {
		rep.Offers[i] = r.Block.Info.BwCls
	}
	return rep
}

func (r *EphemReq) Steady() bool {
	return false
}

func (r *EphemReq) NumHops() int {
	return r.Block.NumHops()
}

func (r *EphemReq) Len() int {
	return r.ID.Len() + r.Block.Len()
}

func (r *EphemReq) Type() DataType {
	if r.ID != nil {
		return REphmSetup
	}
	return REphmRenewal
}

func (r *EphemReq) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.EphemReq.Write",
			"min", r.Len(), "actual", len(b))
	}
	end := 0
	if r.ID != nil {
		end = sibra.EphemIDLen
		r.ID.Write(b[:end])
	}
	if err := r.Block.Write(b[end:]); err != nil {
		return err
	}
	return nil
}

func (r *EphemReq) Reverse() (Data, error) {
	c := &EphemReq{
		ID:    r.ID,
		Block: r.Block,
	}
	return c, nil
}

func (r *EphemReq) String() string {
	return fmt.Sprintf("ID: [%s] Block: %s", r.ID, r.Block)

}
