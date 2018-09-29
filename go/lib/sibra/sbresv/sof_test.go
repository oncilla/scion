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

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/util"
	"github.com/scionproto/scion/go/lib/xtest"
)

var rawSOFCont, _ = hex.DecodeString("8000a00c01020304")
var rawSOFNotCont, _ = hex.DecodeString("0000a00c01020304")

func TestNewSOFieldFromRaw(t *testing.T) {
	Convey("Parse offer correctly", t, func() {
		Convey("continue set", func() {
			sof, err := NewSOFieldFromRaw(rawSOFCont)
			SoMsg("err", err, ShouldBeNil)
			SoMsg("Continue", sof.Continue, ShouldEqual, true)
			SoMsg("Ingress", sof.Ingress, ShouldEqual, 10)
			SoMsg("Egress", sof.Egress, ShouldEqual, 12)
			SoMsg("Mac", sof.Mac, ShouldResemble, common.RawBytes{0x01, 0x02, 0x03, 0x04})
		})
		Convey("continue unset", func() {
			sof, err := NewSOFieldFromRaw(rawSOFNotCont)
			SoMsg("err", err, ShouldBeNil)
			SoMsg("Continue", sof.Continue, ShouldEqual, false)
			SoMsg("Ingress", sof.Ingress, ShouldEqual, 10)
			SoMsg("Egress", sof.Egress, ShouldEqual, 12)
			SoMsg("Mac", sof.Mac, ShouldResemble, common.RawBytes{0x01, 0x02, 0x03, 0x04})
		})
	})
}

func TestOffer_Write(t *testing.T) {
	Convey("Write SOF correctly", t, func() {
		Convey("Continue flag set", func() {
			sof := SOField{
				Continue: true,
				Ingress:  10,
				Egress:   12,
				Mac:      common.RawBytes{0x01, 0x02, 0x03, 0x04},
			}
			b := make(common.RawBytes, sof.Len())
			err := sof.Write(b)
			SoMsg("err", err, ShouldBeNil)
			other, err := NewSOFieldFromRaw(b)
			SoMsg("parseErr", err, ShouldBeNil)
			SoMsg("Continue", other.Continue, ShouldEqual, sof.Continue)
			SoMsg("Ingress", other.Ingress, ShouldEqual, sof.Ingress)
			SoMsg("Egress", other.Egress, ShouldEqual, sof.Egress)
			SoMsg("Mac", other.Mac, ShouldResemble, sof.Mac)
		})
		Convey("Continue flag unset", func() {
			sof := SOField{
				Ingress: 10,
				Egress:  12,
				Mac:     common.RawBytes{0x01, 0x02, 0x03, 0x04},
			}
			b := make(common.RawBytes, sof.Len())
			err := sof.Write(b)
			SoMsg("err", err, ShouldBeNil)
			other, err := NewSOFieldFromRaw(b)
			SoMsg("parseErr", err, ShouldBeNil)
			SoMsg("Continue", other.Continue, ShouldEqual, sof.Continue)
			SoMsg("Ingress", other.Ingress, ShouldEqual, sof.Ingress)
			SoMsg("Egress", other.Egress, ShouldEqual, sof.Egress)
			SoMsg("Mac", other.Mac, ShouldResemble, sof.Mac)
		})

	})
}

func TestSOField_SetMac(t *testing.T) {
	Convey("SetMac and CalcMac agree", t, func() {
		mac, err := util.InitMac(make(common.RawBytes, 16))
		xtest.FailOnErr(t, err)
		info := &Info{
			BwCls:    13,
			PathType: sibra.PathTypeEphemeral,
			RLC:      4,
			Index:    2,
			ExpTick:  sibra.CurrentTick().Add(4),
		}
		ids := []sibra.ID{
			sibra.NewEphemIDRand(addr.AS(0x101010)),
			sibra.NewSteadyID(addr.AS(0x101010), 1),
			sibra.NewSteadyID(addr.AS(0x202020), 2),
			sibra.NewSteadyID(addr.AS(0x303030), 3),
		}
		pLens := []uint8{3, 4, 5}
		sof := SOField{
			Continue: true,
			Ingress:  10,
			Egress:   12,
			Mac:      common.RawBytes{0x01, 0x02, 0x03, 0x04},
		}
		prev := sof.Pack()
		err = sof.SetMac(mac, info, ids, pLens, prev)
		SoMsg("err", err, ShouldBeNil)
		auth, err := sof.CalcMac(mac, info, ids, pLens, prev)
		SoMsg("calcErr", err, ShouldBeNil)
		SoMsg("Auth", auth, ShouldResemble, sof.Mac)
	})
	Convey("Verifiable", t, func() {
		mac, err := util.InitMac(make(common.RawBytes, 16))
		xtest.FailOnErr(t, err)
		info := &Info{
			BwCls:    13,
			PathType: sibra.PathTypeEphemeral,
			RLC:      4,
			Index:    2,
			ExpTick:  sibra.CurrentTick().Add(4),
		}
		ids := []sibra.ID{
			sibra.NewEphemIDRand(addr.AS(0x101010)),
			sibra.NewSteadyID(addr.AS(0x101010), 1),
			sibra.NewSteadyID(addr.AS(0x202020), 2),
			sibra.NewSteadyID(addr.AS(0x303030), 3),
		}
		pLens := []uint8{3, 4, 5}
		sof := SOField{
			Continue: true,
			Ingress:  10,
			Egress:   12,
			Mac:      common.RawBytes{0x01, 0x02, 0x03, 0x04},
		}
		prev := sof.Pack()
		err = sof.SetMac(mac, info, ids, pLens, prev)
		SoMsg("err", err, ShouldBeNil)
		Convey("Authentic", func() {
			err := sof.Verify(mac, info, ids, pLens, prev)
			SoMsg("verify", err, ShouldBeNil)
		})
		Convey("Unauthentic", func() {
			sof.Mac[0] ^= 0x1
			err := sof.Verify(mac, info, ids, pLens, prev)
			SoMsg("verify", err, ShouldNotBeNil)
		})

	})
}
