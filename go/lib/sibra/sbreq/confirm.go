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

var _ Request = (*ConfirmIndex)(nil)

// ConfirmIndex is a request to send a index confirmation. The index can either
// be confirmed to an pending index or to an active index.
type ConfirmIndex struct {
	*Base
	// Idx is the index to be confirmed.
	Idx sibra.Index
	// State is the state which the index shall be confirmed to.
	State sibra.State
	// numHops keeps track of how many hops there are.
	numHops int
}

func ConfirmIndexFromRaw(raw common.RawBytes, numHops int) (*ConfirmIndex, error) {
	b, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return ConfirmIndexFromBase(b, raw, numHops)
}

func ConfirmIndexFromBase(b *Base, raw common.RawBytes, numHops int) (*ConfirmIndex, error) {
	if b.Type != RSteadyConfIndex {
		return nil, common.NewBasicError("Invalid request type", nil, "type", b.Type)
	}
	c := &ConfirmIndex{
		Base:    b,
		Idx:     sibra.Index(raw[BaseLen]),
		State:   sibra.State(raw[BaseLen+1]),
		numHops: numHops,
	}
	return c, nil
}

func NewConfirmIndex(numHops int, idx sibra.Index, state sibra.State) (*ConfirmIndex, error) {
	if state != sibra.StatePending && state != sibra.StateActive {
		return nil, common.NewBasicError("Invalid confirm index state", nil, "state", state)
	}
	c := &ConfirmIndex{
		Base: &Base{
			Type:     RSteadyConfIndex,
			Accepted: true,
		},
		Idx:     idx,
		State:   state,
		numHops: numHops,
	}
	return c, nil
}

func (c *ConfirmIndex) Steady() bool {
	return true
}

func (c *ConfirmIndex) NumHops() int {
	return c.numHops
}

func (c *ConfirmIndex) Len() int {
	return common.LineLen
}

func (c *ConfirmIndex) Write(raw common.RawBytes) error {
	if len(raw) < c.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "SIBRAConfirmIndex.Write",
			"min", c.Len(), "actual", len(raw))
	}
	raw.Zero()
	raw[BaseLen] = byte(c.Idx)
	raw[BaseLen+1] = byte(c.State)
	return c.Base.Write(raw)
}

func (c *ConfirmIndex) Reverse() (Request, error) {
	if c.Response {
		return nil, common.NewBasicError("Reversing not supported", nil,
			"response", c.Response, "accepted", c.Accepted)
	}
	c.Response = true
	return c, nil
}

func (c *ConfirmIndex) String() string {
	return fmt.Sprintf("Base: [%s] Idx: %d State: %s", c.Base, c.Idx, c.State)

}
