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

// Info is the SIBRA reservation info field. It stores information about
// a (requested or active) reservation.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Expiration time (4B)              | BW fwd | BW rev |Idx|F|xx|Fail hop|
// +--------+--------+--------+--------+--------+--------+--------+--------+
//
// The reservation index (Idx) is used to allow for multiple overlapping
// reservations within a single path, which enables renewal and changing the
// bandwidth requested.
//
// The F(orward) flag is used when registering a steady path, to indicate which
// direction it will traverse the path. E.g. a steady path registered with a
// local path server will have the forward flag set, as anything using that
// path will traverse it in the direction it was created. A steady path
// registered with a core path server will have the forward flag unset, as
// anything using the path from that direction will traverse the path in the
// opposite direction to creation.
//
// The fail hop field is normally set to 0, and ignored unless this reservation
// info is part of a denied request, in which case it is set to the number of
// the first hop to reject the reservation.
type Info struct {
	// raw is the underlying buffer.
	raw common.RawBytes
	// ExpTick is the SIBRA tick when the reservation expires.
	ExpTick Tick
	// BwPair is the requested bandwidth class pair.
	BwPair BWPair
	// Index is the reservation index.
	Index Index
	// FailHop is the fail hop. It is only considered in a ResvOffer.
	FailHop uint8
	// Forward indicates in what direction the path is traversed.
	Forward bool
}

func NewInfoFromRaw(raw common.RawBytes) *Info {
	return &Info{
		raw:     raw,
		ExpTick: Tick(common.Order.Uint32(raw[:4])),
		BwPair:  BWPairFromRaw(raw[4:6]),
		Index:   IndexFromUint8(raw[6]),
		Forward: (raw[6] & 0x8) != 0,
		FailHop: raw[7],
	}
}

// write writes all updates to the underlying buffer.
func (i *Info) write() {
	i.writeToBuff(i.raw, false)
}

// writeToBuff writes the Info to a buffer. If the info is used to compute the mac of
// a OpField, the fwd flag is ignored.
func (i *Info) writeToBuff(b common.RawBytes, mac bool) {
	common.Order.PutUint32(b[:4], uint32(i.ExpTick))
	b[4], b[5] = uint8(i.BwPair.Fwd), uint8(i.BwPair.Rev)
	b[6], b[7] = i.Index.ToUint8(), i.FailHop
	if i.Forward && !mac {
		b[6] |= 0x8
	}
}

func (i *Info) Len() int {
	return infoLen
}
