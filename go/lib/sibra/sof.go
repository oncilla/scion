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
)

const (
	// opFieldLen is the SIBRA opaque field length.
	opFieldLen = common.LineLen
)

// OpField is the SIBRA Opqaue Field. This is used for routing SIBRA packets.
// It describes the ingress/egress interfaces, and has a MAC to authenticate
// that it was issued for this reservation.
//
// Whether the previous or the next OpField is used as input for the mac
// depends on the path type.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Ingress IF      | Egress IF       | MAC(IFs, res info, pathIDs, sof)  |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type OpField struct {
	// Ingress is the ingress interface.
	Ingress Interface
	// Egress is the egress interface.
	Egress Interface
	// Mac is the MAC over (IFs, res info, pathIDs, prev).
	Mac common.RawBytes
}

func NewOpFieldFromRaw(raw common.RawBytes) *OpField {
	return &OpField{
		Ingress: Interface(common.Order.Uint16(raw[:2])),
		Egress:  Interface(common.Order.Uint16(raw[2:4])),
		Mac:     append(common.RawBytes(nil), raw[4:8]...),
	}
}

func (o *OpField) Write(b common.RawBytes) error {
	if len(b) < o.Len() {
		return common.NewBasicError(BufferToShort, nil, "method", "SIBRAopField.Write",
			"min", o.Len(), "actual", len(b))
	}
	common.Order.PutUint16(b[:2], uint16(o.Ingress))
	common.Order.PutUint16(b[2:4], uint16(o.Egress))
	copy(b[4:8], o.Mac)
	return nil
}

func (o *OpField) CalcMac(mac hash.Hash, info *Info, ids []*ResvID,
	sof common.RawBytes) (common.RawBytes, error) {

	//	all := make(common.RawBytes, macInputLen)
	//	common.Order.PutUint16(all[:2], uint16(o.Ingress))
	//	common.Order.PutUint16(all[2:4], uint16(o.Egress))
	//	off, end := 4, 4+info.Len()
	//	info.writeToBuff(all[off:end], true)
	//	for i := range ids {
	//		off, end = end, end+len(ids[i])
	//		copy(all[off:end], ids[i])
	//	}
	//	off = 4 + info.Len() + maxPathIDsLen
	//	if prev != nil {
	//		copy(all[off:off+MacLen], prev)
	//	}
	//	tag, err := util.Mac(mac, all)
	//	return tag[:MacLen], err
	return nil, nil // TODO(roosd): implement
}

func (o *OpField) Len() int {
	return opFieldLen
}
