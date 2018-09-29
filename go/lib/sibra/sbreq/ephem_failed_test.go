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
	"github.com/scionproto/scion/go/lib/xtest"
)

var (
	rawEphemFailedSetupId, _ = hex.DecodeString("0000001010104d65822107fcfd52341e")
	rawEphemFailedSetup, _   = hex.DecodeString("0000001010104d65822107fcfd52341e16ebd8ed0a042400" +
		"0023030205040302000000")
	rawEphemFailedRenew, _ = hex.DecodeString("16ebd9a10a042400001403020504030200000000")
)

func TestEphemFailedFromRaw(t *testing.T) {
	Convey("Parse failed ephem setup correctly", t, func() {
		failed, err := EphemFailedFromRaw(rawEphemFailedSetup, true, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("ID", failed.ID, ShouldResemble, sibra.ID(rawEphemFailedSetupId))
		SoMsg("Info", failed.Info, ShouldResemble, &sbresv.Info{
			ExpTick:  sibra.Tick(384555245),
			BwCls:    10,
			Index:    2,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		})
		SoMsg("DataLen", failed.DataLen, ShouldEqual, 35)
		SoMsg("FailHop", failed.FailHop, ShouldEqual, 3)
		SoMsg("FailCode", failed.FailCode, ShouldEqual, BwExceeded)
		SoMsg("Offers", failed.Offers, ShouldResemble, []sibra.BwCls{5, 4, 3, 2, 0})
	})
	Convey("Parse failed ephem renewal correctly", t, func() {
		failed, err := EphemFailedFromRaw(rawEphemFailedRenew, false, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("ID", failed.ID, ShouldBeNil)
		SoMsg("Info", failed.Info, ShouldResemble, &sbresv.Info{
			ExpTick:  sibra.Tick(384555425),
			BwCls:    10,
			Index:    2,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		})
		SoMsg("DataLen", failed.DataLen, ShouldEqual, 20)
		SoMsg("FailHop", failed.FailHop, ShouldEqual, 3)
		SoMsg("FailCode", failed.FailCode, ShouldEqual, BwExceeded)
		SoMsg("Offers", failed.Offers, ShouldResemble, []sibra.BwCls{5, 4, 3, 2, 0})
	})
}

func TestEphemFailed_Write(t *testing.T) {
	Convey("Write setup should return correct result", t, func() {
		failed := &EphemFailed{
			ID: sibra.NewEphemIDRand(addr.AS(0x101010)),
			Info: &sbresv.Info{
				ExpTick:  sibra.CurrentTick().Add(4),
				BwCls:    10,
				Index:    2,
				RLC:      4,
				PathType: sibra.PathTypeEphemeral,
			},
			DataLen:  35,
			FailHop:  3,
			FailCode: BwExceeded,
			Offers:   []sibra.BwCls{5, 4, 3, 2, 0},
		}
		b := make(common.RawBytes, failed.Len())
		err := failed.Write(b)
		xtest.FailOnErr(t, err)
		other, err := EphemFailedFromRaw(b, true, 5)
		SoMsg("ID", other.ID, ShouldResemble, failed.ID)
		SoMsg("Info", other.Info, ShouldResemble, failed.Info)
		SoMsg("DataLen", other.DataLen, ShouldEqual, failed.DataLen)
		SoMsg("FailHop", other.FailHop, ShouldEqual, failed.FailHop)
		SoMsg("FailCode", other.FailCode, ShouldEqual, failed.FailCode)
		SoMsg("Offers", other.Offers, ShouldResemble, failed.Offers)
	})
	Convey("Write renewal should return correct result", t, func() {
		failed := &EphemFailed{
			Info: &sbresv.Info{
				ExpTick:  sibra.CurrentTick().Add(4),
				BwCls:    10,
				Index:    2,
				RLC:      4,
				PathType: sibra.PathTypeEphemeral,
			},
			DataLen:  20,
			FailHop:  3,
			FailCode: BwExceeded,
			Offers:   []sibra.BwCls{5, 4, 3, 2, 0},
		}
		b := make(common.RawBytes, failed.Len())
		err := failed.Write(b)
		xtest.FailOnErr(t, err)
		other, err := EphemFailedFromRaw(b, false, 5)
		SoMsg("ID", other.ID, ShouldBeNil)
		SoMsg("Info", other.Info, ShouldResemble, failed.Info)
		SoMsg("DataLen", other.DataLen, ShouldEqual, failed.DataLen)
		SoMsg("FailHop", other.FailHop, ShouldEqual, failed.FailHop)
		SoMsg("FailCode", other.FailCode, ShouldEqual, failed.FailCode)
		SoMsg("Offers", other.Offers, ShouldResemble, failed.Offers)
	})
}

func TestEphemFailed_MinOffer(t *testing.T) {
	Convey("Return correct min offer", t, func() {
		failed := &EphemFailed{
			Offers: []sibra.BwCls{15, 2, 29, 4, 5, 6, 7, 8},
		}
		SoMsg("Min", failed.MinOffer(), ShouldEqual, sibra.BwCls(2))
	})
}
