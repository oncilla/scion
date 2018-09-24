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

var _ Data = (*SteadySucc)(nil)

// SteadySucc is the response for a successful steady reservation request.
// It contains the reservation block.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation Info                                                      |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | SIBRA Opaque Field                                                    |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |...                                                                    |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type SteadySucc struct {
	// Block is the reservation block.
	Block *sbresv.Block
	// DataType is the data type.
	DataType DataType
}

func SteadySuccFromRaw(raw common.RawBytes, t DataType, numHops int) (*SteadySucc, error) {
	if len(raw) <= common.LineLen {
		return nil, common.NewBasicError("Invalid steady reservation response length", nil,
			"min", common.LineLen, "actual", len(raw))
	}
	block, err := sbresv.BlockFromRaw(raw, numHops)
	if err != nil {
		return nil, err
	}
	s := &SteadySucc{
		Block:    block,
		DataType: t,
	}
	return s, nil
}

func (r *SteadySucc) SetSOF(mac hash.Hash, ids []sibra.ID, plens []uint8,
	inIFID, egIFID common.IFIDType, sofIdx int) error {

	r.Block.SOFields[sofIdx].Ingress = inIFID
	r.Block.SOFields[sofIdx].Egress = egIFID
	return r.Block.SetMac(mac, sofIdx, ids, plens)
}

func (r *SteadySucc) Steady() bool {
	return true
}

func (r *SteadySucc) NumHops() int {
	return len(r.Block.SOFields)
}

func (r *SteadySucc) Len() int {
	return r.Block.Len()
}

func (r *SteadySucc) Type() DataType {
	return r.DataType
}

func (r *SteadySucc) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.SteadySucc.Write",
			"min", r.Len(), "actual", len(b))
	}
	return r.Block.Write(b)
}

func (r *SteadySucc) Reverse() (Data, error) {
	return nil, common.NewBasicError("Cannot reverse steady success", nil)
}

func (r *SteadySucc) String() string {
	return fmt.Sprintf("Block: [%s]", r.Block)

}
