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
	offSteadyReqFail   = 0
	offSteadyReqMin    = 1
	offSteadyReqMax    = 2
	offSteadyReqAcc    = 3
	offSteadyReqSplit  = 4
	offSteadyReqProps  = 5
	offSteadyReqBaseID = 6
)

var _ Data = (*SteadyReq)(nil)

// SteadyReq is the SIBRA request for a steady reservations. It can
// contain a reservation request, or the response for a failed request.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |FailHop | Min rBW| Max rBW| AccBW  | Split  |End Prop| Base ID         |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ... (only for telescope request)									   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Info 																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |All 1 BW|Max 1 BW|Lines 1 |All 2 BW|Max 2 BW|Lines 2 | ...             |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type SteadyReq struct {
	// BaseID is the base ID in a telescoping request.
	BaseID sibra.ID
	// Info is the reservation info field.
	Info *sbresv.Info
	// OfferFields are the SIBRA offer fields.
	OfferFields []*Offer
	// MinBw is the minimum bandwidth class requested by the reservation initiator.
	MinBw sibra.BwCls
	// MaxBw is the maximum bandwidth class requested by the reservation initiator.
	MaxBw sibra.BwCls
	// AccBw is the minimum of the so far allocated bandwidth classes.
	AccBw sibra.BwCls
	// FailHop is the first hop that failed the request.
	FailHop uint8
	// Split is the bandwidth split class.
	Split sibra.SplitCls
	// EndProps are the reservation end properties.
	EndProps sibra.EndProps
	// DataType indicates whether this is for setup, telescope setup or renewal.
	DataType DataType
}

func SteadyReqFromRaw(raw common.RawBytes, t DataType, numHops int) (*SteadyReq, error) {
	if len(raw) < calcSteadyResvReqLen(t, numHops) {
		return nil, common.NewBasicError("Invalid steady reservation request length", nil,
			"numHops", numHops, "min", calcSteadyResvReqLen(t, numHops), "actual", len(raw))
	}
	block := &SteadyReq{
		FailHop:     raw[offSteadyReqFail],
		MinBw:       sibra.BwCls(raw[offSteadyReqMin]),
		MaxBw:       sibra.BwCls(raw[offSteadyReqMax]),
		AccBw:       sibra.BwCls(raw[offSteadyReqAcc]),
		Split:       sibra.SplitCls(raw[offSteadyReqSplit]),
		EndProps:    sibra.EndProps(raw[offSteadyReqProps]),
		OfferFields: make([]*Offer, numHops),
	}
	off := offSteadyReqBaseID
	if t == RSteadySetupTelescope {
		block.BaseID = sibra.ID(raw[off : off+sibra.SteadyIDLen])
		off = off + sibra.SteadyIDLen
	}
	block.Info = sbresv.NewInfoFromRaw(raw[off : off+sbresv.InfoLen])
	off = off + sbresv.InfoLen
	for i := 0; i < numHops; i++ {
		block.OfferFields[i] = NewOfferFromRaw(raw[off:])
		off += block.OfferFields[i].Len()
	}
	return block, nil
}

func (r *SteadyReq) Steady() bool {
	return true
}

func (r *SteadyReq) NumHops() int {
	return len(r.OfferFields)
}

func (r *SteadyReq) Len() int {
	return calcSteadyResvReqLen(r.DataType, r.NumHops())
}

func (r *SteadyReq) Type() DataType {
	return r.DataType
}

func calcSteadyResvReqLen(t DataType, numHops int) int {
	min := offSteadyReqBaseID + sbresv.InfoLen + numHops*(offerFieldLen)
	if t == RSteadySetupTelescope {
		return min + sibra.SteadyIDLen
	}
	return min
}

func (r *SteadyReq) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "SIBRASteadyResvReq.Write",
			"min", r.Len(), "actual", len(b))
	}
	b[offSteadyReqFail] = byte(r.FailHop)
	b[offSteadyReqMin] = byte(r.MinBw)
	b[offSteadyReqMax] = byte(r.MaxBw)
	b[offSteadyReqAcc] = byte(r.AccBw)
	b[offSteadyReqSplit] = byte(r.Split)
	b[offSteadyReqProps] = byte(r.EndProps)

	off, end := offSteadyReqBaseID, offSteadyReqBaseID
	if r.DataType == RSteadySetupTelescope {
		end += sibra.SteadyIDLen
		r.BaseID.Write(b[off:end])
	}
	off, end = end, end+sbresv.InfoLen
	if err := r.Info.Write(b[off:end]); err != nil {
		return err
	}
	for _, op := range r.OfferFields {
		off, end = end, end+op.Len()
		if err := op.Write(b[off:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r *SteadyReq) Reverse() (Data, error) {
	c := &SteadyReq{
		BaseID:      r.BaseID,
		DataType:    r.DataType,
		EndProps:    r.EndProps,
		Split:       r.Split,
		AccBw:       r.AccBw,
		FailHop:     r.FailHop,
		Info:        r.Info,
		OfferFields: r.OfferFields,
		MaxBw:       r.MaxBw,
		MinBw:       r.MinBw,
	}
	return c, nil
}

func (r *SteadyReq) String() string {
	return fmt.Sprintf("Info: [%s] Max: %v Min: %v AccBw: %v FailHop: %d Split: %v "+
		"EndProps: %v OfferFields: %s", r.Info, r.MaxBw, r.MinBw, r.AccBw, r.FailHop, r.Split,
		r.EndProps, r.OfferFields)

}
