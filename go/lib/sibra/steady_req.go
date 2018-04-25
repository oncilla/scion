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

import "github.com/scionproto/scion/go/lib/common"

// SteadyReqBlock is the SIBRA request block for steady reservations.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation Info                                                      |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | min rBW| max rBW|          Padding         |all1 BW |min1 BW |max1 BW |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |  Padding                                   |all2 BW |min2 BW |max2 BW |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |...										             |
// +--------+--------+--------+--------+--------+--------+--------+--------+
//
// A reservation block is made up of a reservation info field, minimum requested BW
// class, maximum requested BW class, and a list of offer fields.
type SteadyReqBlock struct {
	// Info is the reservation info field.
	Info *Info
	// MinBw is the minimum bandwidth class requested by the reservation initiator.
	MinBw BwClass
	// MaxBw is the maximum bandwidth class requested by the reservation initiator.
	MaxBw BwClass
	// OfferFields are the SIBRA offer fields.
	OfferFields []*OfferField
}

func SteadyReqBlockFromRaw(raw common.RawBytes, numHops int) (*SteadyReqBlock, error) {
	if calcSteadyReqBlockLen(numHops) != len(raw) {
		return nil, common.NewBasicError("Invalid request block length", nil, "numHops",
			numHops, "expected", calcResvBlockLen(numHops), "actual", len(raw))
	}
	off, end := 0, infoLen
	block := &SteadyReqBlock{
		Info:        NewInfoFromRaw(raw[:end]),
		MinBw:       BwClass(raw[end]),
		MaxBw:       BwClass(raw[end+1]),
		OfferFields: make([]*OfferField, numHops),
	}
	for i := 0; i < numHops; i++ {
		off, end = end, end+offerFieldLen
		block.OfferFields[i] = NewOfferFieldFromRaw(raw[off:end])
	}
	return block, nil
}

func (r *SteadyReqBlock) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError(BufferToShort, nil, "method", "SIBRASteadyReqBlock.Write",
			"min", r.Len(), "actual", len(b))
	}
	off, end := 0, r.Info.Len()
	if err := r.Info.Write(b[off:end]); err != nil {
		return err
	}
	b[end] = uint8(r.MinBw)
	b[end+1] = uint8(r.MaxBw)
	for _, op := range r.OfferFields {
		off, end := end, end+op.Len()
		if err := op.Write(b[off:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r *SteadyReqBlock) Len() int {
	return calcResvBlockLen(len(r.OfferFields))
}

func calcSteadyReqBlockLen(numHops int) int {
	return infoLen + numHops*opFieldLen
}
