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

package sbresv

import (
	"fmt"

	"github.com/scionproto/scion/go/lib/common"
)

const (
	// InfoLen is the reservation info length.
	InfoLen = common.LineLen
)

// Info is the SIBRA reservation info field. It stores information about
// a reservation.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Expiration time (4B)              |  BwCls | RTT Cls|Idx|Type|Fail hop|
// +--------+--------+--------+--------+--------+--------+--------+--------+
//
// The bandwidth class (BwCls) indicates the reserved bandwidth in an active
// reservation. In a steady request, it indicates the minimal bandwidth class
// reserved so far. In a ephemeral request, it indicates the bandwidth class
// that the source end host is seeking to reserve.
//
// The round trip class (RTT Cls) allows for more granular control in the
// pending request garbage collection.
//
// The reservation index (Idx) is used to allow for multiple overlapping
// reservations within a single path, which enables renewal and changing the
// bandwidth requested.
//
// Type indicates which path type of the reservation.
//
// The fail hop field is normally set to 0, and ignored unless this reservation
// info is part of a denied request, in which case it is set to the number of
// the first hop to reject the reservation.
type Info struct {
	// ExpTick is the SIBRA tick when the reservation expires.
	ExpTick Tick
	// BwCls is the bandwidth class.
	BwCls BwCls
	// RttCls is the round trip class.
	RttCls RttCls
	// Index is the reservation index.
	Index Index
	// PathType is the path type.
	PathType PathType
	// FailHop is the fail hop. It is unset in an active reservation.
	FailHop uint8
}

func NewInfoFromRaw(raw common.RawBytes) *Info {
	return &Info{
		ExpTick:  Tick(common.Order.Uint32(raw[:4])),
		BwCls:    BwCls(raw[4]),
		RttCls:   RttCls(raw[5]),
		Index:    parseIdx(raw[6]),
		PathType: parsePathType(raw[6]),
		FailHop:  raw[7],
	}
}

func parseIdx(field uint8) Index {
	return Index(field >> 4)
}

func parsePathType(field uint8) PathType {
	return PathType(field & 0x7)
}

func (i *Info) Len() int {
	return InfoLen
}

func (i *Info) Write(b common.RawBytes) error {
	if len(b) < i.Len() {
		return common.NewBasicError("Buffer to short", nil, "method",
			"sbresv.Info.Write", "min", i.Len(), "actual", len(b))
	}
	common.Order.PutUint32(b[:4], uint32(i.ExpTick))
	b[4] = uint8(i.BwCls)
	b[5] = uint8(i.RttCls)
	b[6] = i.packIdxPathType()
	b[7] = i.FailHop
	return nil
}

func (i *Info) packIdxPathType() uint8 {
	return (uint8(i.Index) << 4) | uint8(i.PathType)
}

func (i *Info) Copy() *Info {
	return &Info{
		ExpTick:  i.ExpTick,
		BwCls:    i.BwCls,
		RttCls:   i.RttCls,
		Index:    i.Index,
		PathType: i.PathType,
		FailHop:  i.FailHop,
	}
}

func (i *Info) String() string {
	return fmt.Sprintf("Exp: %s BwCls: %v RttCls: %v Idx: %d Path: %s FailHop: %d",
		i.ExpTick.Time(), i.BwCls, i.RttCls, i.Index, i.PathType, i.FailHop)
}

func (i *Info) Eq(o *Info) bool {
	return *i == *o
}
