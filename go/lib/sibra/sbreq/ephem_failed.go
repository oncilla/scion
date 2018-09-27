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
	offEphemFailedDataLen = sbresv.InfoLen
	offEphemFailedHop     = offEphemFailedDataLen + 2
	offEphemFailedCode    = offEphemFailedHop + 1
	offEphemFailedOffers  = offEphemFailedCode + 1
)

var _ Data = (*EphemFailed)(nil)

// EphemFailed holds a failed SIBRA ephemeral reservation requests.
// In case it is for a setup request, it contains the reservation id.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Ephemeral ID (opt)													   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Info												                   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | DataLen         | FailHop|FailCode| MaxBw 1| MaxBW 2| ...			   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | padding															   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type EphemFailed struct {
	// ID is the requested ephemeral reservation id set in setup requests.
	ID sibra.ID
	// Info is the requested reservation info.
	Info *sbresv.Info
	// Offers contains the offered bandwidth classes.
	Offers []sibra.BwCls
	// DataLen contains the byte length. This is done to avoid
	// resizing the packet. Thus, the response will keep the same size as the
	// request.
	DataLen uint16
	// FailHop indicates the first hop that failed the request.
	FailHop uint8
	// FailCode indicates why the reservation failed.
	FailCode FailCode
}

func EphemFailedFromRaw(raw common.RawBytes, setup bool, numHops int) (*EphemFailed, error) {
	min := offEphemFailedCode
	off := 0
	if setup {
		min += sibra.EphemIDLen
		off += sibra.EphemIDLen
	}
	if len(raw) < min {
		return nil, common.NewBasicError("Invalid ephemeral failed length", nil,
			"min", min, "actual", len(raw))
	}
	e := &EphemFailed{
		DataLen:  common.Order.Uint16(raw[off+offEphemFailedDataLen : off+offEphemFailedDataLen+2]),
		FailHop:  raw[off+offEphemFailedHop],
		FailCode: FailCode(raw[off+offEphemFailedCode]),
		Offers:   make([]sibra.BwCls, numHops),
	}
	if len(raw) < int(e.DataLen) {
		return nil, common.NewBasicError("Invalid ephemeral failed length", nil,
			"expected", int(e.DataLen), "actual", len(raw))
	}
	off, end := 0, 0
	if setup {
		end = sibra.EphemIDLen
		e.ID = sibra.ID(raw[off:end])
	}
	off, end = end, end+sbresv.InfoLen
	e.Info = sbresv.NewInfoFromRaw(raw[off:end])
	off += offEphemFailedOffers
	for i := 0; i < numHops; i++ {
		e.Offers[i] = sibra.BwCls(raw[off+i])
	}
	return e, nil
}

func (r *EphemFailed) MinOffer() sibra.BwCls {
	if len(r.Offers) < 1 {
		return 0
	}
	offer := r.Offers[0]
	for _, v := range r.Offers {
		if offer > v {
			offer = v
		}
	}
	return offer
}

func (r *EphemFailed) Steady() bool {
	return false
}

func (r *EphemFailed) Len() int {
	return int(r.DataLen)
}

func (r *EphemFailed) Type() DataType {
	if r.ID != nil {
		return REphmSetup
	}
	return REphmRenewal
}

func (r *EphemFailed) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.EphemFailed.Write",
			"min", r.Len(), "actual", len(b))
	}
	off, end := 0, 0
	if r.ID != nil {
		off, end = end, end+sibra.EphemIDLen
		r.ID.Write(b[off:end])
	}
	off, end = end, end+offEphemFailedDataLen
	if err := r.Info.Write(b[off:end]); err != nil {
		return err
	}
	common.Order.PutUint16(b[end:end+2], r.DataLen)
	b[off+offEphemFailedCode] = uint8(r.FailCode)
	b[off+offEphemFailedHop] = r.FailHop
	off += offEphemFailedOffers
	for i := 0; i < len(r.Offers); i++ {
		b[off+i] = uint8(r.Offers[i])
	}
	return nil
}

func (r *EphemFailed) Reverse() (Data, error) {
	c := &EphemFailed{
		ID:       r.ID,
		FailHop:  r.FailHop,
		FailCode: r.FailCode,
		DataLen:  r.DataLen,
		Offers:   r.Offers,
		Info:     r.Info,
	}
	return c, nil
}

func (r *EphemFailed) String() string {
	return fmt.Sprintf("Info: [%s] Len: %d Hop %d Code: %s Offers: [%s]",
		r.Info, r.DataLen, r.FailHop, r.FailCode, r.Offers)

}
