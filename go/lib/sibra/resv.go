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

// ResvBlock is the SIBRA reservation block.
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
		Info:     NewInfoFromRaw(raw[:end]),
		OpFields: make([]*OpField, numHops),
	}
	for i := 0; i < numHops; i++ {
		off, end = end, end+opFieldLen
		block.OpFields[i] = NewOpFieldFromRaw(raw[off:end])
	}
	return block, nil
}

func (r *ResvBlock) Write(b common.RawBytes) error {
	if len(b) < r.Len() {
		return common.NewBasicError(BufferToShort, nil, "method", "SIBRAResvBlock.Write",
			"min", r.Len(), "actual", len(b))
	}
	off, end := 0, infoLen
	if err := r.Info.Write(b[off:end]); err != nil {
		return err
	}
	for _, op := range r.OpFields {
		off, end := end, end+opFieldLen
		if err := op.Write(b[off:end]); err != nil {
			return err
		}
	}
	return nil
}

func (r *ResvBlock) Len() int {
	return calcResvBlockLen(len(r.OpFields))
}

func calcResvBlockLen(numHops int) int {
	return infoLen + numHops*opFieldLen
}
