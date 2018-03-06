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

// ResvBlock is the SIBRA reservation block. This can either be an active
// block, in which case it's used for routing the packet; or a request block,
// in which case it's evaluated and filled in by each hop on the path. If any
// hop rejects the request, then this block will be replaced by an offers
// block.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Reservation Info                                                      |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | SIBRA Opaque Field (8B)                                               |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |...                                                                    |
// +--------+--------+--------+--------+--------+--------+--------+--------+
//
// A reservation block is made up of a reservation info field, and a list of
// SIBRA opaque fields.
type ResvBlock struct {
	// raw is the underlying buffer
	raw common.RawBytes
	// Info is the reservation info field.
	Info *Info
	// OpFields are the SIBRA opaque fields.
	OpFields []*OpField
}

func ResvBlockFromRaw(raw common.RawBytes, numHops int) (*ResvBlock, error) {
	if calcResvBlockLen(numHops) != len(raw) {
		return nil, common.NewBasicError("Invalid reservation block length", nil, "numHops",
			numHops, "expected", calcResvBlockLen(numHops), "actual", len(raw))
	}
	off, end := 0, infoLen
	block := &ResvBlock{
		raw:      raw,
		Info:     NewInfoFromRaw(raw[:end]),
		OpFields: make([]*OpField, numHops),
	}
	for i := 0; i < numHops; i++ {
		off, end = end, end+opFieldLen
		block.OpFields[i] = NewOpFieldFromRaw(raw[off:end])
	}
	return block, nil
}

func (r *ResvBlock) write() {
	r.Info.write()
	for _, op := range r.OpFields {
		op.write()
	}
}

func (r *ResvBlock) Len() int {
	return calcResvBlockLen(len(r.OpFields))
}

func calcResvBlockLen(numHops int) int {
	return infoLen + numHops*opFieldLen
}
