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

var _ Request = (*SteadySucc)(nil)

// SteadySucc is the response for a successful steady reservation request.
// It contains the reservation block.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | base   |          padding                                             |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation Info                                                      |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | SIBRA Opaque Field                                                    |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |...                                                                    |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type SteadySucc struct {
	*Base
	// Block is the reservation block.
	Block *sbresv.Block
}

func SteadySuccFromRaw(raw common.RawBytes, numHops int) (*SteadySucc, error) {
	b, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return SteadySuccFromBase(b, raw, numHops)
}

func SteadySuccFromBase(b *Base, raw common.RawBytes, numHops int) (*SteadySucc, error) {
	if b.Type != RSteadySetup && b.Type != RSteadyRenewal {
		return nil, common.NewBasicError("Invalid request type", nil, "type", b.Type)
	}
	if !b.Response {
		return nil, common.NewBasicError("Response flag must be set", nil)
	}
	if !b.Accepted {
		return nil, common.NewBasicError("Accepted flag must be set", nil)
	}
	if len(raw) <= common.LineLen {
		return nil, common.NewBasicError("Invalid steady reservation response length", nil,
			"min", common.LineLen, "actual", len(raw))
	}
	block, err := sbresv.BlockFromRaw(raw[common.LineLen:], numHops)
	if err != nil {
		return nil, err
	}
	return &SteadySucc{Base: b, Block: block}, nil
}

func NewSteadySuccFromReq(r *SteadyReq) (*SteadySucc, error) {
	if !r.Accepted {
		return nil, common.NewBasicError("Unable to create steady reservation "+
			"response from failed request", nil)
	}
	s := &SteadySucc{
		Block: sbresv.NewBlock(r.Info, r.NumHops()),
		Base: &Base{
			Type:     r.Base.Type,
			Accepted: true,
			Response: true,
		},
	}
	return s, nil
}

func (r *SteadySucc) Steady() bool {
	return true
}

func (r *SteadySucc) NumHops() int {
	return len(r.Block.SOFields)
}

func (r *SteadySucc) Len() int {
	return common.LineLen + r.Block.Len()
}

func (r *SteadySucc) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.SteadySucc.Write",
			"min", r.Len(), "actual", len(b))
	}
	if err := r.Base.Write(b); err != nil {
		return err
	}
	return r.Block.Write(b[common.LineLen:])
}

func (r *SteadySucc) Reverse() (Request, error) {
	return nil, common.NewBasicError("Reversing not supported", nil,
		"response", r.Response, "accepted", r.Accepted)
}

func (r *SteadySucc) String() string {
	return fmt.Sprintf("Base: [%s] Block: [%s]", r.Base, r.Block)

}
