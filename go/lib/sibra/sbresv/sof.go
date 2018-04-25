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
	"bytes"
	"fmt"
	"hash"

	"crypto/aes"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/util"
)

const (
	// maxPathIDsLen is the maximum space required to write all path ids.
	maxPathIDsLen = 3*SteadyIDLen + EphemIDLen
	// upadded is the unpadded input length for SIBRA opaque field MAC
	// computation. Sum of len(Ingress), len(Egress), len(Info), maxPathIDsLen,
	// len(pathLens), len(prev sof).
	unpadded = 3 + InfoLen + maxPathIDsLen + 3 + SOFieldLen
	// padding is the padding to make macInputLen a multiple of aes.BlockSize.
	padding = (aes.BlockSize - unpadded%aes.BlockSize) % aes.BlockSize
	// macInputLne is the input length for SIBRA opaque field MAC computation.
	macInputLen = unpadded + padding
	// MacLen is the SIBRA opaque field MAC length.
	MacLen = 4
	// SOFieldLen is the length of a SIBRA opaque field.
	SOFieldLen = common.LineLen
	// SOFieldLines is the number of lines a SOField spans.
	SOFieldLines = SOFieldLen / common.LineLen

	ErrorSOFBadMac   = "Bad SOF MAC"
	ErrorSOFTooShort = "SOF too short"
)

// SOField is the SIBRA Opqaue Field. It is used for routing SIBRA packets and
// describes the ingress/egress interfaces. A MAC is used to authenticate
// that it was issued for this reservation.
//
// Whether the previous or the next SOField is used as input for the mac
// depends on the path type specified in the reservation info field.
//
// When calculating the mac for stitched steady paths, only the reservation id
// and path length of the current steady block must be provided.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// |c|      | Ingress IF | Egress IF   | MAC(IFs,resInfo,resIDs,pLens,SOF) |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type SOField struct {
	// Mac is the MAC over (IFs, res info, pathIDs, prev).
	Mac common.RawBytes
	// Ingress is the ingress interface.
	Ingress common.IFIDType
	// Egress is the egress interface.
	Egress common.IFIDType
	// Continue indicates if the SOF spans multiple lines.
	Continue bool
}

func NewSOFieldFromRaw(b common.RawBytes) (*SOField, error) {
	if len(b) < SOFieldLen {
		return nil, common.NewBasicError(ErrorSOFTooShort, nil,
			"min", SOFieldLen, "actual", len(b))
	}
	sof := &SOField{
		Continue: b[0]&0x80 != 1,
		Ingress:  common.IFIDType(int(b[1])<<4 | int(b[2])>>4),
		Egress:   common.IFIDType((int(b[2])&0xF)<<8 | int(b[3])),
		Mac:      b[4 : 4+MacLen],
	}
	return sof, nil
}

func (s *SOField) Verify(mac hash.Hash, info *Info, ids []ID, pLens []uint8,
	sof common.RawBytes) error {

	if mac, err := s.CalcMac(mac, info, ids, pLens, sof); err != nil {
		return err
	} else if !bytes.Equal(s.Mac, mac) {
		return common.NewBasicError(ErrorSOFBadMac, nil, "expected", s.Mac, "actual", mac)
	}
	return nil
}

func (s *SOField) SetMac(mac hash.Hash, info *Info, ids []ID, pLens []uint8,
	sof common.RawBytes) error {

	tag, err := s.CalcMac(mac, info, ids, pLens, sof)
	if err != nil {
		return err
	}
	copy(s.Mac, tag)
	return nil
}

func (s *SOField) CalcMac(mac hash.Hash, info *Info, ids []ID, pLens []uint8,
	sof common.RawBytes) (common.RawBytes, error) {

	if info.FailHop != 0 {
		return nil, common.NewBasicError("Invalid fail hop", nil, "expected", 0,
			"actual", info.FailHop)
	}
	all := make(common.RawBytes, macInputLen)
	if err := s.writeIFIDs(all[:3]); err != nil {
		return nil, common.NewBasicError("Unable to write IFIDs", err)
	}
	off, end := 3, 3+info.Len()
	info.Write(all[off:end])
	for i := range ids {
		off, end = end, end+ids[i].Len()
		ids[i].Write(all[off:end])
	}
	off = 3 + info.Len() + maxPathIDsLen
	end = off + 3
	copy(all[off:end], pLens)
	if sof != nil {
		copy(all[end:end+len(sof)], sof)
	}
	tag, err := util.Mac(mac, all)
	if err != nil {
		return nil, err
	}
	return tag[:MacLen], nil
}

func (s *SOField) Len() int {
	return SOFieldLen
}

func (s *SOField) Pack() common.RawBytes {
	b := make(common.RawBytes, s.Len())
	s.Write(b)
	return b
}

func (s *SOField) Write(b common.RawBytes) error {
	if len(b) < s.Len() {
		return common.NewBasicError("Buffer to short", nil, "method",
			"sbresv.SOField.Write", "min", s.Len(), "actual", len(b))
	}
	b[0] = 0
	if s.Continue {
		b[0] |= 0x80
	}
	if err := s.writeIFIDs(b[1:4]); err != nil {
		return common.NewBasicError("Unable to write IFIDs", err, "method",
			"sbresv.SOField.Write")
	}
	copy(b[4:8], s.Mac)
	return nil
}

func (s *SOField) writeIFIDs(b common.RawBytes) error {
	if len(b) < 3 {
		return common.NewBasicError("Buffer to short", nil, "min", 3, "actual", len(b))
	}
	b[0] = byte(s.Ingress >> 4)
	b[1] = byte((s.Ingress&0x0F)<<4 | s.Egress>>8)
	b[2] = byte(s.Egress & 0XFF)
	return nil
}

func (s *SOField) String() string {
	return fmt.Sprintf("Ingress: %s Egress: %s Mac: %s", s.Ingress, s.Egress, s.Mac)
}
