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
	"encoding/hex"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/util"
	"github.com/scionproto/scion/go/lib/xtest"
)

var rawBlock, _ = hex.DecodeString("16ebdb4f0d04240000001002010203040000300411121314000050062122232400007008313233340000900041424344")

func TestBlockFromRaw(t *testing.T) {
	Convey("Parse Block", t, func() {
		block := &Block{
			Info: &Info{
				BwCls:    13,
				PathType: sibra.PathTypeEphemeral,
				RLC:      4,
				Index:    2,
				ExpTick:  384555855,
			},
			SOFields: []*SOField{
				{Ingress: 1, Egress: 2, Mac: []byte{0x01, 0x02, 0x03, 0x04}},
				{Ingress: 3, Egress: 4, Mac: []byte{0x11, 0x12, 0x13, 0x14}},
				{Ingress: 5, Egress: 6, Mac: []byte{0x21, 0x22, 0x23, 0x24}},
				{Ingress: 7, Egress: 8, Mac: []byte{0x31, 0x32, 0x33, 0x34}},
				{Ingress: 9, Egress: 0, Mac: []byte{0x41, 0x42, 0x43, 0x44}},
			},
		}
		other, err := BlockFromRaw(rawBlock, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("info", other.Info.Eq(block.Info), ShouldBeTrue)
		SoMsg("sof", other.SOFields, ShouldResemble, block.SOFields)
	})
}

func TestNewBlock(t *testing.T) {
	Convey("New Block", t, func() {
		block := &Block{
			Info: &Info{
				BwCls:    13,
				PathType: sibra.PathTypeEphemeral,
				RLC:      4,
				Index:    2,
				ExpTick:  384555855,
			},
			SOFields: []*SOField{
				{Mac: make(common.RawBytes, 4)},
				{Mac: make(common.RawBytes, 4)},
				{Mac: make(common.RawBytes, 4)},
				{Mac: make(common.RawBytes, 4)},
				{Mac: make(common.RawBytes, 4)},
			},
		}
		other := NewBlock(block.Info, 5)
		SoMsg("info", other.Info.Eq(block.Info), ShouldBeTrue)
		SoMsg("sof", other.SOFields, ShouldResemble, block.SOFields)
	})
}

func TestBlock_Write(t *testing.T) {
	Convey("Write block correctly", t, func() {
		block := &Block{
			Info: &Info{
				BwCls:    13,
				PathType: sibra.PathTypeEphemeral,
				RLC:      4,
				Index:    2,
				ExpTick:  384555855,
			},
			SOFields: []*SOField{
				{Ingress: 1, Egress: 2, Mac: []byte{0x01, 0x02, 0x03, 0x04}},
				{Ingress: 3, Egress: 4, Mac: []byte{0x11, 0x12, 0x13, 0x14}},
				{Ingress: 5, Egress: 6, Mac: []byte{0x21, 0x22, 0x23, 0x24}},
				{Ingress: 7, Egress: 8, Mac: []byte{0x31, 0x32, 0x33, 0x34}},
				{Ingress: 9, Egress: 0, Mac: []byte{0x41, 0x42, 0x43, 0x44}},
			},
		}

		b := make(common.RawBytes, block.Len())
		err := block.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other, err := BlockFromRaw(b, 5)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("info", other.Info.Eq(block.Info), ShouldBeTrue)
		SoMsg("sof", other.SOFields, ShouldResemble, block.SOFields)

	})
}

func TestBlock_SetMac(t *testing.T) {
	Convey("SetMac and Verify agree", t, func() {
		mac, err := util.InitMac(make(common.RawBytes, 16))
		xtest.FailOnErr(t, err)
		now := time.Now()
		block := &Block{
			Info: &Info{
				BwCls:    13,
				PathType: sibra.PathTypeEphemeral,
				RLC:      4,
				Index:    2,
				ExpTick:  sibra.TimeToTick(now).Add(4),
			},
			SOFields: []*SOField{
				{
					Ingress: 10,
					Egress:  12,
					Mac:     common.RawBytes{0x01, 0x02, 0x03, 0x04},
				},
				{
					Ingress: 10,
					Egress:  12,
					Mac:     common.RawBytes{0x01, 0x02, 0x03, 0x04},
				},
			},
		}
		ids := []sibra.ID{
			sibra.NewEphemIDRand(addr.AS(0x101010)),
			sibra.NewSteadyID(addr.AS(0x101010), 1),
			sibra.NewSteadyID(addr.AS(0x202020), 2),
			sibra.NewSteadyID(addr.AS(0x303030), 3),
		}
		pLens := []uint8{3, 4, 5}
		err = block.SetMac(mac, 0, ids, pLens)
		SoMsg("err 0", err, ShouldBeNil)
		err = block.SetMac(mac, 1, ids, pLens)
		SoMsg("err 1", err, ShouldBeNil)

		Convey("Authentic", func() {
			SoMsg("Verify 0", block.Verify(mac, 0, ids, pLens, now), ShouldBeNil)
			SoMsg("Verify 2", block.Verify(mac, 1, ids, pLens, now), ShouldBeNil)
		})
		Convey("Unauthentic", func() {
			block.SOFields[0].Mac[0] ^= 0x1
			SoMsg("Verify 0", block.Verify(mac, 0, ids, pLens, now), ShouldNotBeNil)
			SoMsg("Verify 1", block.Verify(mac, 1, ids, pLens, now), ShouldNotBeNil)
		})
	})
}

func TestBlock_Copy(t *testing.T) {
	Convey("Copy works as expected", t, func() {
		block := &Block{
			Info: &Info{
				BwCls:    13,
				PathType: sibra.PathTypeEphemeral,
				RLC:      4,
				Index:    2,
				ExpTick:  384555855,
			},
			SOFields: []*SOField{
				{Ingress: 1, Egress: 2, Mac: []byte{0x01, 0x02, 0x03, 0x04}},
				{Ingress: 3, Egress: 4, Mac: []byte{0x11, 0x12, 0x13, 0x14}},
				{Ingress: 5, Egress: 6, Mac: []byte{0x21, 0x22, 0x23, 0x24}},
				{Ingress: 7, Egress: 8, Mac: []byte{0x31, 0x32, 0x33, 0x34}},
				{Ingress: 9, Egress: 0, Mac: []byte{0x41, 0x42, 0x43, 0x44}},
			},
		}
		c := block.Copy()
		Convey("copy is equal", func() {
			SoMsg("Eq", c, ShouldResemble, block)
		})
		Convey("info not affected by change", func() {
			c.Info.Index += 1
			SoMsg("NotEq", c.Info.Eq(block.Info), ShouldBeFalse)
		})
		Convey("SOF not affected by change", func() {
			c.SOFields[0].Mac[0] ^= 0x1
			SoMsg("NotEq", c, ShouldNotResemble, block)
		})
	})
}
