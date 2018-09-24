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
	offerFieldLen = 3

	offsetOfferAlloc   = 0
	offsetOfferMax     = 1
	offsetOfferLineLen = 2
)

// Offer is the SIBRA offer Field.
type Offer struct {
	// AllocBw is the allocated bandwidth class.
	AllocBw sibra.BwCls
	// MaxBw is the maximum bandwidth class.
	MaxBw sibra.BwCls
	// LineLen is the line length of the SOField
	LineLen uint8
}

func NewOfferFromRaw(raw common.RawBytes) *Offer {
	return &Offer{
		AllocBw: sibra.BwCls(raw[offsetOfferAlloc]),
		MaxBw:   sibra.BwCls(raw[offsetOfferMax]),
		LineLen: raw[offsetOfferLineLen],
	}
}

func (o *Offer) Len() int {
	return offerFieldLen
}

func (o *Offer) Write(b common.RawBytes) error {
	if len(b) < o.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "SIBRAOfferField.Write",
			"min", o.Len(), "actual", len(b))
	}
	b[offsetOfferAlloc] = uint8(o.AllocBw)
	b[offsetOfferMax] = uint8(o.MaxBw)
	b[offsetOfferLineLen] = o.LineLen
	return nil
}

func (o *Offer) String() string {
	return fmt.Sprintf("Alloc: %v Max: %v LineLen: %d", o.AllocBw, o.MaxBw, o.LineLen)
}
