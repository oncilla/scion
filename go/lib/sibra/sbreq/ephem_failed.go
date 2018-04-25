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
	offFailCode = common.LineLen - 2
	offEphmLen  = common.LineLen - 1
)

var _ Request = (*EphemReq)(nil)

// EphemFailed holds a failed SIBRA ephemeral reservation requests.
// In case it is for a setup request, it contains the reservation id.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | base   |          padding                           |  code  |  len   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Ephemeral ID (opt)													   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Info												                   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | max1 BW| max2 BW|...												   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | padding															   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type EphemFailed struct {
	*Base
	// ReqID is the requested ephemeral reservation id set in setup requests.
	ReqID sbresv.ID
	// Info is the requested reservation info.
	Info *sbresv.Info
	// Offers contains the offered bandwidth classes.
	Offers []sbresv.BwCls
	// FailCode indicates why the reservation failed.
	FailCode FailCode
	// LineLen contains the line length of the header. This is done to avoid
	// resizing the packet. Thus, the response will keep the same size as the
	// request.
	LineLen uint8
}

func EphemFailedFromRaw(raw common.RawBytes, numHops int) (*EphemFailed, error) {
	b, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return EphemFailedFromBase(b, raw, numHops)
}

func EphemFailedFromBase(b *Base, raw common.RawBytes, numHops int) (*EphemFailed, error) {
	if b.Type != REphmSetup && b.Type != REphmRenewal {
		return nil, common.NewBasicError("Invalid request type", nil, "type", b.Type)
	}
	if b.Accepted {
		return nil, common.NewBasicError("Must be failed", nil)
	}
	min := offEphemId + common.LineLen
	if b.Type == REphmSetup {
		min += sbresv.EphemIDLen
	}
	if len(raw) < min {
		return nil, common.NewBasicError("Invalid ephemeral reservation reply length", nil,
			"min", min, "actual", len(raw))
	}
	if len(raw) != int(raw[offEphmLen])*common.LineLen {
		return nil, common.NewBasicError("Invalid ephemeral reservation reply length", nil,
			"expected", int(raw[offEphmLen])*common.LineLen, "actual", len(raw))
	}
	var reqID sbresv.ID
	off, end := 0, offEphemId
	if b.Type == REphmSetup {
		off, end = end, end+sbresv.EphemIDLen
		reqID = sbresv.ID(raw[off:end])
	}
	off, end = end, end+sbresv.InfoLen
	rep := &EphemFailed{
		Base:     b,
		ReqID:    reqID,
		Info:     sbresv.NewInfoFromRaw(raw[off:end]),
		Offers:   make([]sbresv.BwCls, numHops),
		FailCode: FailCode(raw[offFailCode]),
		LineLen:  raw[offEphmLen],
	}
	for i := 0; i < numHops; i++ {
		rep.Offers[i] = sbresv.BwCls(raw[end+i])
	}
	return rep, nil
}

func (r *EphemFailed) EphemID() sbresv.ID {
	return r.ReqID
}

func (r *EphemFailed) Steady() bool {
	return false
}

func (r *EphemFailed) NumHops() int {
	return len(r.Offers)
}

func (r *EphemFailed) Len() int {
	return int(r.LineLen) * common.LineLen
}

func (r *EphemFailed) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.EphemFailed.Write",
			"min", r.Len(), "actual", len(b))
	}
	if err := r.Base.Write(b); err != nil {
		return err
	}
	b[offFailCode] = uint8(r.FailCode)
	b[offEphmLen] = r.LineLen
	off, end := 0, offEphemId
	if r.Type == REphmSetup {
		off, end = end, end+sbresv.EphemIDLen
		r.ReqID.Write(b[off:end])
	}
	off, end = end, end+r.Info.Len()
	if err := r.Info.Write(b[off:end]); err != nil {
		return err
	}
	for i := 0; i < len(r.Offers); i++ {
		b[end+i] = uint8(r.Offers[i])
	}
	return nil
}

func (r *EphemFailed) Reverse() (Request, error) {
	if r.Response {
		return nil, common.NewBasicError("Reversing not supported", nil,
			"response", r.Response, "accepted", r.Accepted)
	}
	r.Response = true
	return r, nil
}

func (r *EphemFailed) String() string {
	return fmt.Sprintf("Base: [%s] Code: %s Len: %d Info: [%s] Offers: [%s]",
		r.Base, r.FailCode, r.LineLen, r.Info, r.Offers)

}
