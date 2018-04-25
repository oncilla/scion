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

package sibra_mgmt

import (
	"fmt"

	"crypto/sha256"
	"encoding/binary"
	"strconv"
	"strings"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/proto"
)

type PathInterface struct {
	RawIsdas addr.IAInt `capnp:"isdas"`
	IfID     common.IFIDType
}

func (iface *PathInterface) ISD_AS() addr.IA {
	return iface.RawIsdas.IA()
}

func (iface PathInterface) String() string {
	return fmt.Sprintf("%s#%d", iface.ISD_AS(), iface.IfID)
}

var _ proto.Cerealizable = (*BlockMeta)(nil)

type BlockMeta struct {
	Id            sbresv.ID
	RawBlock      common.RawBytes `capnp:"block"`
	Block         *sbresv.Block   `capnp:"-"`
	RawCreation   uint32          `capnp:"creation"`
	RawInterfaces common.RawBytes `capnp:"interfaces"`
	Interfaces    []PathInterface `capnp:"-"`
	Signature     common.RawBytes
	WhiteList     common.RawBytes
	Mtu           uint16
}

func (m *BlockMeta) ParseRaw() error {
	var err error
	m.Block, err = parseBlock(m.RawBlock)
	if err != nil {
		return common.NewBasicError("Unable to parse block", err)
	}
	m.Interfaces, err = parseIntfs(m.RawInterfaces)
	if err != nil {
		return common.NewBasicError("Unable to parse interfaces", err)
	}
	return nil
}

func (m *BlockMeta) SetBlock(block *sbresv.Block) error {
	p, err := packBlock(block)
	if err != nil {
		return err
	}
	m.RawBlock = p
	m.Block, err = parseBlock(m.RawBlock)
	return err
}

func (m *BlockMeta) SetInterfaces(intfs []PathInterface) error {
	var err error
	m.RawInterfaces = packIntfs(intfs)
	m.Interfaces, err = parseIntfs(m.RawInterfaces)
	return err
}

func (m *BlockMeta) Creation() time.Time {
	return time.Unix(int64(m.RawCreation), 0)
}

// InitIA returns the initiator IA.
func (m *BlockMeta) InitIA() addr.IA {
	if m.Block == nil {
		return addr.IA{}
	}
	if m.Block.Info.PathType.Reversed() {
		return m.EndIA()
	}
	return m.StartIA()
}

func (m *BlockMeta) StartIA() addr.IA {
	if m.Interfaces == nil || len(m.Interfaces) == 0 {
		return addr.IA{}
	}
	return m.Interfaces[0].ISD_AS()
}

func (m *BlockMeta) EndIA() addr.IA {
	if m.Interfaces == nil || len(m.Interfaces) == 0 {
		return addr.IA{}
	}
	return m.Interfaces[len(m.Interfaces)-1].ISD_AS()
}

func (m *BlockMeta) Expiry() time.Time {
	if m.Block == nil {
		return time.Time{}
	}
	return m.Block.Info.ExpTick.Time()
}

func (m *BlockMeta) SegID() common.RawBytes {
	return PathToSegID(m.Interfaces)
}

func (m *BlockMeta) String() string {
	hops := m.fmtIfaces()
	var idx = "na"
	if m.Block != nil && m.Block.Info != nil {
		idx = strconv.Itoa(int(m.Block.Info.Index))
	}
	return fmt.Sprintf("ResvID %s idx %s: Hops: [%s] Mtu: %d",
		m.Id, idx, strings.Join(hops, ">"), m.Mtu)
}

func (m *BlockMeta) ProtoId() proto.ProtoIdType {
	return proto.SibraBlockMeta_TypeID
}

func PathToSegID(intfs []PathInterface) common.RawBytes {
	h := sha256.New()
	for _, intf := range intfs {
		binary.Write(h, common.Order, intf.RawIsdas)
		binary.Write(h, common.Order, intf.IfID)
	}
	return h.Sum(nil)
}

func (m *BlockMeta) fmtIfaces() []string {
	var hops []string
	if len(m.Interfaces) == 0 {
		return hops
	}
	intf := m.Interfaces[0]
	hops = append(hops, fmt.Sprintf("%s %d", intf.ISD_AS(), intf.IfID))
	for i := 1; i < len(m.Interfaces)-1; i += 2 {
		inIntf := m.Interfaces[i]
		outIntf := m.Interfaces[i+1]
		hops = append(hops, fmt.Sprintf("%d %s %d", inIntf.IfID, inIntf.ISD_AS(), outIntf.IfID))
	}
	intf = m.Interfaces[len(m.Interfaces)-1]
	hops = append(hops, fmt.Sprintf("%d %s", intf.IfID, intf.ISD_AS()))
	return hops
}

func packBlock(block *sbresv.Block) (common.RawBytes, error) {
	packed := make(common.RawBytes, block.Len()+1)
	packed[0] = uint8(block.NumHops())
	return packed, block.Write(packed[1:])
}

func parseBlock(raw common.RawBytes) (*sbresv.Block, error) {
	if len(raw) < 2 {
		return nil, common.NewBasicError("Invalid raw block size", nil, "len", len(raw))
	}
	return sbresv.BlockFromRaw(common.RawBytes(raw[1:]), int(raw[0]))
}

func packIntfs(intfs []PathInterface) common.RawBytes {
	p := make(common.RawBytes, len(intfs)*(addr.IABytes+common.IFIDBytes))
	off, end := 0, addr.IABytes
	for i := range intfs {
		common.Order.PutUint64(p[off:end], uint64(intfs[i].RawIsdas))
		off, end = end, end+addr.IABytes
		common.Order.PutUint64(p[off:end], uint64(intfs[i].IfID))
		off, end = end, end+common.IFIDBytes
	}
	return p
}

func parseIntfs(raw common.RawBytes) ([]PathInterface, error) {
	if len(raw)%(addr.IABytes+common.IFIDBytes) != 0 {
		return nil, common.NewBasicError("Invalid raw interfaces size", nil, "len", len(raw))
	}
	intfs := make([]PathInterface, len(raw)/(addr.IABytes+common.IFIDBytes))
	off, end := 0, addr.IABytes
	for i := range intfs {
		intfs[i].RawIsdas = addr.IAInt(common.Order.Uint64(raw[off:end]))
		off, end = end, end+addr.IABytes
		intfs[i].IfID = common.IFIDType(common.Order.Uint64(raw[off:end]))
		off, end = end, end+common.IFIDBytes
	}
	return intfs, nil
}
