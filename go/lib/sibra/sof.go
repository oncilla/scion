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

import (
	"hash"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/util"
)

// OpField is the SIBRA Opqaue Field. This is used for routing SIBRA packets.
// It describes the ingress/egress interfaces, and has a MAC to authenticate
// that it was issued for this reservation.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Ingress IF      | Egress IF       | MAC(IFs, res info, pathIDs, prev) |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type OpField struct {
	// raw is the underlying buffer.
	raw common.RawBytes
	// Ingress is the ingress interface.
	Ingress Interface
	// Egress is the egress interface.
	Egress Interface
	// Mac is the MAC over (IFs, res info, pathIDs, prev). It is mapped directly to raw.
	Mac common.RawBytes
}

func NewOpFieldFromRaw(raw common.RawBytes) *OpField {
	return &OpField{
		raw:     raw,
		Ingress: Interface(common.Order.Uint16(raw[:2])),
		Egress:  Interface(common.Order.Uint16(raw[2:4])),
		Mac:     raw[4:8],
	}
}

// write writes all updates to the underlying buffer.
func (o *OpField) write() {
	common.Order.PutUint16(o.raw[:2], uint16(o.Ingress))
	common.Order.PutUint16(o.raw[2:4], uint16(o.Egress))
}

func (o *OpField) CalcMac(mac hash.Hash, info *Info, ids []PathID,
	prev common.RawBytes) (common.RawBytes, error) {

	all := make(common.RawBytes, macInputLen)
	common.Order.PutUint16(all[:2], uint16(o.Ingress))
	common.Order.PutUint16(all[2:4], uint16(o.Egress))
	off, end := 4, 4+info.Len()
	info.writeToBuff(all[off:end], true)
	for i := range ids {
		off, end = end, end+len(ids[i])
		copy(all[off:end], ids[i])
	}
	off = 4 + info.Len() + maxPathIDsLen
	if prev != nil {
		copy(all[off:off+MacLen], prev)
	}
	tag, err := util.Mac(mac, all)
	return tag[:MacLen], err
}

func (o *OpField) Len() int {
	return opFieldLen
}
