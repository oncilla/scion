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
	"github.com/scionproto/scion/go/lib/sibra"
)

const (
	ConfirmIndexLen = 3

	offConfIdxFlags    = 0
	offConfIdxFailHop  = 1
	offConfIdxFailCode = 2
)

var _ Data = (*ConfirmIndex)(nil)

// ConfirmIndex is a request to send a index confirmation. The index can either
// be confirmed to an pending index or to an active index.
type ConfirmIndex struct {
	// Idx is the index to be confirmed.
	Idx sibra.Index
	// State is the state which the index shall be confirmed to.
	State sibra.State
	// FailHop indicates the first hop that failed the request.
	FailHop uint8
	// FailCode indicates the fail code.
	FailCode FailCode
}

func ConfirmIndexFromRaw(raw common.RawBytes) (*ConfirmIndex, error) {
	if len(raw) < ConfirmIndexLen {
		return nil, common.NewBasicError("Invalid confirm index length", nil,
			"min", ConfirmIndexLen, "actual", len(raw))
	}
	c := &ConfirmIndex{
		Idx:      sibra.Index(raw[offConfIdxFlags] >> 4),
		State:    sibra.State(raw[offConfIdxFlags] & 0x0F),
		FailHop:  raw[offConfIdxFailHop],
		FailCode: FailCode(raw[offConfIdxFailCode]),
	}
	return c, nil
}

func (c *ConfirmIndex) Steady() bool {
	return true
}

func (c *ConfirmIndex) Len() int {
	return ConfirmIndexLen
}

func (c *ConfirmIndex) Type() DataType {
	return RSteadyConfIndex
}

func (c *ConfirmIndex) Write(raw common.RawBytes) error {
	if len(raw) < c.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "SIBRAConfirmIndex.Write",
			"min", c.Len(), "actual", len(raw))
	}
	raw[offConfIdxFlags] = uint8(c.State) & 0x0F
	raw[offConfIdxFlags] |= (uint8(c.Idx) << 4) & 0xF0
	raw[offConfIdxFailHop] = c.FailHop
	raw[offConfIdxFailCode] = uint8(c.FailCode)
	return nil
}

func (c *ConfirmIndex) Reverse() (Data, error) {
	r := &ConfirmIndex{
		Idx:      c.Idx,
		State:    c.State,
		FailHop:  c.FailHop,
		FailCode: c.FailCode,
	}
	return r, nil
}

func (c *ConfirmIndex) String() string {
	return fmt.Sprintf("Idx: %d State: %s FailHop %d FaildCode: %s",
		c.Idx, c.State, c.FailHop, c.FailCode)

}
