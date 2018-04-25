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

// Package sbreq provides a framework for SIBRA request data.
package sbreq

import (
	"fmt"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

const (
	BaseLen = 1

	flagAccepted = 0x10
	flagResponse = 0x08
	flagType     = 0x07
)

// Base is the basis for SIBRA request. It can either be a request
// or a response for a request.
//
// 0B             1
// +-----------------+
// |r|r|r|A|R|ReqType|
// +-----------------+
//
// The 3 MSB are reserved.
// The 5 LSB are allocated as follows:
//  - A: Set if the request has been accepted so far.
// 	- R: Set if this is a response.
// 	- RequestType(3 bit): indicates the type of request.
type Base struct {
	// Type indicates the type of request.
	Type RequestType
	// Response indicates if this is a response.
	Response bool
	// Accepted indicates if the request is accepted so far.
	Accepted bool
}

func BaseFromRaw(raw common.RawBytes) (*Base, error) {
	if len(raw) < BaseLen {
		return nil, common.NewBasicError("Invalid SIBRA request hdr length", nil,
			"min", BaseLen, "actual", len(raw))
	}
	b := &Base{
		Type:     RequestType(raw[0] & flagType),
		Response: (raw[0] & flagResponse) != 0,
		Accepted: (raw[0] & flagAccepted) != 0,
	}
	return b, nil
}

func (b *Base) EphemID() sbresv.ID {
	return nil
}

func (b *Base) GetBase() *Base {
	return b
}

func (b *Base) Write(raw common.RawBytes) error {
	if len(raw) < BaseLen {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.Base.Write",
			"min", BaseLen, "actual", len(raw))
	}
	raw[0] = byte(b.Type)
	if b.Response {
		raw[0] |= flagResponse
	}
	if b.Accepted {
		raw[0] |= flagAccepted
	}
	return nil
}

func (b *Base) String() string {
	return fmt.Sprintf("RequestType: %s Response: %t Accepted: %t",
		b.Type, b.Response, b.Accepted)
}
