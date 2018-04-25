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

package sibra

import (
	"github.com/scionproto/scion/go/lib/common"
)

const (
	// offerFieldLen is the SIBRA offer field length.
	offerFieldLen = common.LineLen
)

// OfferField is the SIBRA offer Field.
// In steady requests:
//  - alloc BW is the BW class the AS has allocated for this request
//  - min BW is the minimum BW class the AS is willing to grant
//    (i.e. when shrinking reservation).
//  - max BW is the maximum BW class the AS is willing to grant.
// In ephemeral failed request:
//  - max BW is the maximum BW class the AS is willing to grant.
//  - alloc BW + min BW is ignored
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |  Padding                                   |alloc BW| min BW | max BW |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type OfferField struct {
	// AllocBw is the allocated bandwidth class.
	AllocBw BwClass
	// MinBw is the minimum bandwidth class.
	MinBw BwClass
	// MaxBw is the maximum bandwidth class.
	MaxBw BwClass
}

func NewOfferFieldFromRaw(raw common.RawBytes) *OfferField {
	return &OfferField{
		AllocBw: BwClass(raw[5]),
		MinBw:   BwClass(raw[6]),
		MaxBw:   BwClass(raw[7]),
	}
}

func (o *OfferField) Write(b common.RawBytes) error {
	if len(b) < o.Len() {
		return common.NewBasicError(BufferToShort, nil, "method", "SIBRAOfferField.Write",
			"min", o.Len(), "actual", len(b))
	}
	b[5] = uint8(o.AllocBw)
	b[6] = uint8(o.MinBw)
	b[7] = uint8(o.MaxBw)
	return nil
}

func (o *OfferField) Len() int {
	return offerFieldLen
}
