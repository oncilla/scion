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
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

const (
	offSteadyResvMin  = 6
	offSteadyResvMax  = 7
	offSteadyResvInfo = common.LineLen
)

var _ Request = (*SteadyReq)(nil)

// SteadyReq is the SIBRA request for a steady reservations. It can
// contain a reservation request, or the response for a failed request.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | base   |          padding                           | min rBW| max rBW|
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Info																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |all1 BW |min1 BW |max1 BW | lines1 | ...                               |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |allN BW |minN BW |maxN BW | linesN |             padding               |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type SteadyReq struct {
	*Base
	// Info is the reservation info field.
	Info *sbresv.Info
	// OfferFields are the SIBRA offer fields.
	OfferFields []*Offer
	// Lines is a list of line lengths for the SOFields.
	Lines []byte
	// MinBw is the minimum bandwidth class requested by the reservation initiator.
	MinBw sibra.BwCls
	// MaxBw is the maximum bandwidth class requested by the reservation initiator.
	MaxBw sibra.BwCls
}

func SteadyReqFromRaw(raw common.RawBytes, numHops int) (*SteadyReq, error) {
	b, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return SteadyReqFromBase(b, raw, numHops)
}

func SteadyReqFromBase(b *Base, raw common.RawBytes, numHops int) (*SteadyReq, error) {
	if b.Type != RSteadySetup && b.Type != RSteadyRenewal {
		return nil, common.NewBasicError("Invalid request type", nil, "type", b.Type)
	}
	if b.Response && b.Accepted {
		return nil, common.NewBasicError("Response must not be accepted", nil)
	}
	if len(raw) < calcSteadyResvReqLen(numHops) {
		return nil, common.NewBasicError("Invalid steady reservation request length", nil,
			"numHops", numHops, "min", calcSteadyResvReqLen(numHops), "actual", len(raw))
	}
	off, end := offSteadyResvInfo, offSteadyResvInfo+sbresv.InfoLen
	block := &SteadyReq{
		Base:        b,
		Info:        sbresv.NewInfoFromRaw(raw[off:end]),
		MinBw:       sibra.BwCls(raw[offSteadyResvMin]),
		MaxBw:       sibra.BwCls(raw[offSteadyResvMax]),
		OfferFields: make([]*Offer, numHops),
		Lines:       make([]byte, numHops),
	}
	for i := 0; i < numHops; i++ {
		off, end = end, end+offerFieldLen
		block.OfferFields[i] = NewOfferFromRaw(raw[off:end])
		block.Lines[i] = raw[end]
		end += 1
	}
	return block, nil
}

func NewSteadyReq(t RequestType, info *sbresv.Info,
	min, max sibra.BwCls, numHops uint8) *SteadyReq {

	base := &Base{
		Type:     t,
		Accepted: true,
	}
	// Create request block.
	req := &SteadyReq{
		Base:        base,
		Info:        info,
		MinBw:       min,
		MaxBw:       max,
		OfferFields: make([]*Offer, numHops),
		Lines:       make([]byte, numHops),
	}
	// Initialize the offer fields.
	for i := range req.OfferFields {
		req.OfferFields[i] = &Offer{}
	}
	// Set allocated bandwidth in own offer field.
	if req.Info.PathType.Reversed() {
		req.OfferFields[len(req.OfferFields)-1].AllocBw = max
	} else {
		req.OfferFields[0].AllocBw = max
	}
	return req
}

func (r *SteadyReq) Steady() bool {
	return true
}

func (r *SteadyReq) NumHops() int {
	return len(r.OfferFields)
}

func (r *SteadyReq) Len() int {
	return calcSteadyResvReqLen(r.NumHops())
}

func calcSteadyResvReqLen(numHops int) int {
	l := offSteadyResvInfo + sbresv.InfoLen + numHops*(offerFieldLen+1)
	padding := (common.LineLen - l%common.LineLen) % common.LineLen
	return l + padding
}

func (r *SteadyReq) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "SIBRASteadyResvReq.Write",
			"min", r.Len(), "actual", len(b))
	}
	if err := r.Base.Write(b); err != nil {
		return err
	}
	b[offSteadyResvMin] = byte(r.MinBw)
	b[offSteadyResvMax] = byte(r.MaxBw)
	off, end := offSteadyResvInfo, offSteadyResvInfo+sbresv.InfoLen
	if err := r.Info.Write(b[off:end]); err != nil {
		return err
	}
	for i, op := range r.OfferFields {
		off, end = end, end+op.Len()
		if err := op.Write(b[off:end]); err != nil {
			return err
		}
		b[end] = r.Lines[i]
		end += 1
	}
	return nil
}

func (r *SteadyReq) Reverse() (Request, error) {
	if r.Response {
		return nil, common.NewBasicError("Reversing not supported", nil,
			"response", r.Response, "accepted", r.Accepted)
	}
	if r.Accepted {
		rep, err := NewSteadySuccFromReq(r)
		return rep, err
	}
	r.Response = true
	return r, nil
}

func (r *SteadyReq) String() string {
	return fmt.Sprintf("Base: [%s] Info: [%s] Max: %d Min: %d OfferFields: %s Lines: %s",
		r.Base, r.Info, r.MaxBw, r.MinBw, r.OfferFields, r.Lines)

}
