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

package sbreq

import (
	"encoding/hex"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/util"
	"github.com/scionproto/scion/go/lib/xtest"
)

var (
	rawEphemReqSetupID, _ = hex.DecodeString("000000101010d5104dc76695721d0976")
	rawEphemReqSetup, _   = hex.DecodeString("000000101010d5104dc76695721d097616ebdddc0a04240000" +
		"000000000000000000000000000000000000000000000000000000000000000000000000000000")
	rawEphemReqRenew, _ = hex.DecodeString("16ebdddc0a042400000000000000000000000000000000000000" +
		"00000000000000000000000000000000000000000000")
)

func TestEphemReqFromRaw(t *testing.T) {
	Convey("Parse ephem setup correctly", t, func() {
		req, err := EphemReqFromRaw(rawEphemReqSetup, true, 5)
		SoMsg("err", err, ShouldBeNil)
		info := &sbresv.Info{
			ExpTick:  sibra.Tick(384556508),
			BwCls:    10,
			Index:    2,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		}
		other := EphemReq{
			ID:    sibra.ID(rawEphemReqSetupID),
			Block: sbresv.NewBlock(info, 5),
		}
		SoMsg("ID", req.ID, ShouldResemble, other.ID)
		SoMsg("Info", req.Block.Info, ShouldResemble, other.Block.Info)
		SoMsg("Sof", req.Block.SOFields, ShouldResemble, other.Block.SOFields)
	})
	Convey("Parse failed ephem renewal correctly", t, func() {
		req, err := EphemReqFromRaw(rawEphemReqRenew, false, 5)
		SoMsg("err", err, ShouldBeNil)
		info := &sbresv.Info{
			ExpTick:  sibra.Tick(384556508),
			BwCls:    10,
			Index:    2,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		}
		other := EphemReq{
			Block: sbresv.NewBlock(info, 5),
		}
		SoMsg("ID", req.ID, ShouldResemble, other.ID)
		SoMsg("Info", req.Block.Info, ShouldResemble, other.Block.Info)
		SoMsg("Sof", req.Block.SOFields, ShouldResemble, other.Block.SOFields)
	})
}

func TestEphemReq_Write(t *testing.T) {
	Convey("Write setup should return correct result", t, func() {
		info := &sbresv.Info{
			ExpTick:  sibra.CurrentTick().Add(4),
			BwCls:    10,
			Index:    2,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		}
		req := &EphemReq{
			ID:    sibra.NewEphemIDRand(addr.AS(0x101010)),
			Block: sbresv.NewBlock(info, 5),
		}
		b := make(common.RawBytes, req.Len())
		err := req.Write(b)
		xtest.FailOnErr(t, err)
		other, err := EphemReqFromRaw(b, true, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("ID", other.ID, ShouldResemble, req.ID)
		SoMsg("Block", other.Block, ShouldResemble, req.Block)
	})
	Convey("Write renewal should return correct result", t, func() {
		info := &sbresv.Info{
			ExpTick:  sibra.CurrentTick().Add(4),
			BwCls:    10,
			Index:    2,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		}
		req := &EphemReq{
			Block: sbresv.NewBlock(info, 5),
		}
		b := make(common.RawBytes, req.Len())
		err := req.Write(b)
		xtest.FailOnErr(t, err)
		other, err := EphemReqFromRaw(b, false, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("ID", other.ID, ShouldBeNil)
		SoMsg("Block", other.Block, ShouldResemble, req.Block)
	})
}

func TestEphemReq_SetSOF(t *testing.T) {
	Convey("Set SOF correctly", t, func() {
		req, err := EphemReqFromRaw(rawEphemReqRenew, false, 5)
		xtest.FailOnErr(t, err)

		mac, err := util.InitMac(make(common.RawBytes, 16))
		xtest.FailOnErr(t, err)
		ids := []sibra.ID{
			sibra.NewEphemIDRand(addr.AS(0x101010)),
			sibra.NewSteadyID(addr.AS(0x101010), 1),
			sibra.NewSteadyID(addr.AS(0x202020), 2),
			sibra.NewSteadyID(addr.AS(0x303030), 3),
		}
		pLens := []uint8{3, 4, 5}
		req.Block.SOFields[0].Egress = 2
		req.Block.SOFields[0].Ingress = 1
		err = req.Block.SetMac(mac, 0, ids, pLens)
		c := append(common.RawBytes(nil), req.Block.SOFields[0].Mac...)
		req.Block.SOFields[0].Mac[0] ^= 0x1
		req.Block.SOFields[0].Egress = 20
		req.Block.SOFields[0].Ingress = 10
		req.SetSOF(mac, ids, pLens, 1, 2, 0)
		SoMsg("Ingress", req.Block.SOFields[0].Ingress, ShouldResemble, common.IFIDType(1))
		SoMsg("Egress", req.Block.SOFields[0].Egress, ShouldResemble, common.IFIDType(2))
		SoMsg("Mac", req.Block.SOFields[0].Mac, ShouldResemble, c)
	})
}

func TestEphemReq_Fail(t *testing.T) {
	Convey("Fail Setup correctly", t, func() {
		req, err := EphemReqFromRaw(rawEphemReqSetup, true, 5)
		xtest.FailOnErr(t, err)
		failed := req.Fail(BwExceeded, 4, 4)
		SoMsg("ID", failed.ID, ShouldResemble, req.ID)
		SoMsg("Info", failed.Info, ShouldResemble, req.Block.Info)
		SoMsg("DataLen", failed.DataLen, ShouldEqual, req.Len())
		SoMsg("Code", failed.FailCode, ShouldEqual, BwExceeded)
		SoMsg("Hop", failed.FailHop, ShouldEqual, 4)
		SoMsg("Offers", failed.Offers, ShouldResemble, []sibra.BwCls{10, 10, 10, 10, 4})
	})
	Convey("Fail Renew correctly", t, func() {
		req, err := EphemReqFromRaw(rawEphemReqRenew, false, 5)
		xtest.FailOnErr(t, err)
		failed := req.Fail(InvalidInfo, 1, 3)
		SoMsg("ID", failed.ID, ShouldResemble, req.ID)
		SoMsg("Info", failed.Info, ShouldResemble, req.Block.Info)
		SoMsg("DataLen", failed.DataLen, ShouldEqual, req.Len())
		SoMsg("Code", failed.FailCode, ShouldEqual, InvalidInfo)
		SoMsg("Hop", failed.FailHop, ShouldEqual, 3)
		SoMsg("Offers", failed.Offers, ShouldResemble, []sibra.BwCls{10, 10, 10, 1, 0})
	})
}
