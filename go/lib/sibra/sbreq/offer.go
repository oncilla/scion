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

const (
	offerFieldLen = 3

	offsetOfferAlloc = 0
	offsetOfferMin   = 1
	offsetOfferMax   = 2
)

// Offer is the SIBRA offer Field.
// In steady requests:
//  - AllocBw is the bandwidth class the AS has allocated for this request
//  - MinBw is the minimum bandwidth class the AS is willing to grant
//    (i.e. when shrinking reservation).
//  - MaxBw is the maximum bandwidth class the AS is willing to grant.
// In ephemeral failed request:
//  - MaxBw is the maximum bandwidth class the AS is willing to grant.
//  - AllocBw + MinBw is unset.
type Offer struct {
	// AllocBw is the allocated bandwidth class.
	AllocBw sbresv.BwCls
	// MinBw is the minimum bandwidth class.
	MinBw sbresv.BwCls
	// MaxBw is the maximum bandwidth class.
	MaxBw sbresv.BwCls
}

func NewOfferFromRaw(raw common.RawBytes) *Offer {
	return &Offer{
		AllocBw: sbresv.BwCls(raw[offsetOfferAlloc]),
		MinBw:   sbresv.BwCls(raw[offsetOfferMin]),
		MaxBw:   sbresv.BwCls(raw[offsetOfferMax]),
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
	b[offsetOfferMin] = uint8(o.MinBw)
	b[offsetOfferMax] = uint8(o.MaxBw)
	return nil
}

func (o *Offer) String() string {
	return fmt.Sprintf("Alloc: %v Min: %v Max: %v", o.AllocBw, o.MinBw, o.MaxBw)
}
