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

	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/util"
	"github.com/scionproto/scion/go/lib/xtest"
)

var (
	rawSteadySuccSetup, _  = hex.DecodeString("16ebe26c0d0421000000a00c010203040000a00c01020304")
	rawSteadySuccRenew, _  = hex.DecodeString("16ebe26c0d0421000000a00c010203040000a00c01020304")
	rawSteadySuccTelesc, _ = hex.DecodeString("16ebe26e0d0421000000a00c010203040000a00c01020304")
)

func TestSteadySuccFromRaw(t *testing.T) {
	Convey("Parse steady success setup correctly", t, func() {
		succ := SteadySucc{
			Block: &sbresv.Block{
				Info: &sbresv.Info{
					BwCls:    13,
					PathType: sibra.PathTypeUp,
					RLC:      4,
					Index:    2,
					ExpTick:  384557676,
				},
				SOFields: []*sbresv.SOField{
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
			},
			DataType: RSteadySetup,
		}
		parsed, err := SteadySuccFromRaw(rawSteadySuccSetup, RSteadySetup, 2)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("Info", parsed.Block.Info, ShouldResemble, succ.Block.Info)
		SoMsg("SOFields", parsed.Block.SOFields, ShouldResemble, succ.Block.SOFields)
		SoMsg("Data", parsed.DataType, ShouldEqual, RSteadySetup)
	})
	Convey("Parse steady success renew correctly", t, func() {
		succ := SteadySucc{
			Block: &sbresv.Block{
				Info: &sbresv.Info{
					BwCls:    13,
					PathType: sibra.PathTypeUp,
					RLC:      4,
					Index:    2,
					ExpTick:  384557676,
				},
				SOFields: []*sbresv.SOField{
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
			},
			DataType: RSteadyRenewal,
		}
		parsed, err := SteadySuccFromRaw(rawSteadySuccRenew, RSteadyRenewal, 2)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("Info", parsed.Block.Info, ShouldResemble, succ.Block.Info)
		SoMsg("SOFields", parsed.Block.SOFields, ShouldResemble, succ.Block.SOFields)
		SoMsg("Data", parsed.DataType, ShouldEqual, RSteadyRenewal)
	})
	Convey("Parse steady success setup correctly", t, func() {
		succ := SteadySucc{
			Block: &sbresv.Block{
				Info: &sbresv.Info{
					BwCls:    13,
					PathType: sibra.PathTypeUp,
					RLC:      4,
					Index:    2,
					ExpTick:  384557678,
				},
				SOFields: []*sbresv.SOField{
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
			},
			DataType: RSteadySetupTelescope,
		}
		parsed, err := SteadySuccFromRaw(rawSteadySuccTelesc, RSteadySetupTelescope, 2)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("Info", parsed.Block.Info, ShouldResemble, succ.Block.Info)
		SoMsg("SOFields", parsed.Block.SOFields, ShouldResemble, succ.Block.SOFields)
		SoMsg("Data", parsed.DataType, ShouldEqual, RSteadySetupTelescope)
	})
}

func TestSteadySucc_Writec(t *testing.T) {
	Convey("Write steady setup success correctly", t, func() {
		succ := SteadySucc{
			Block: &sbresv.Block{
				Info: &sbresv.Info{
					BwCls:    13,
					PathType: sibra.PathTypeUp,
					RLC:      4,
					Index:    2,
					ExpTick:  sibra.TimeToTick(time.Now()).Add(4),
				},
				SOFields: []*sbresv.SOField{
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
			},
			DataType: RSteadySetup,
		}
		b := make(common.RawBytes, succ.Len())
		err := succ.Write(b)
		SoMsg("err", err, ShouldBeNil)
		parsed, err := SteadySuccFromRaw(b, RSteadySetup, 2)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("Info", parsed.Block.Info, ShouldResemble, succ.Block.Info)
		SoMsg("SOFields", parsed.Block.SOFields, ShouldResemble, succ.Block.SOFields)
		SoMsg("Data", parsed.DataType, ShouldEqual, RSteadySetup)
	})
	Convey("Write steady renew success correctly", t, func() {
		succ := SteadySucc{
			Block: &sbresv.Block{
				Info: &sbresv.Info{
					BwCls:    13,
					PathType: sibra.PathTypeUp,
					RLC:      4,
					Index:    2,
					ExpTick:  sibra.TimeToTick(time.Now()).Add(4),
				},
				SOFields: []*sbresv.SOField{
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
			},
			DataType: RSteadyRenewal,
		}
		b := make(common.RawBytes, succ.Len())
		err := succ.Write(b)
		SoMsg("err", err, ShouldBeNil)
		parsed, err := SteadySuccFromRaw(b, RSteadyRenewal, 2)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("Info", parsed.Block.Info, ShouldResemble, succ.Block.Info)
		SoMsg("SOFields", parsed.Block.SOFields, ShouldResemble, succ.Block.SOFields)
		SoMsg("Data", parsed.DataType, ShouldEqual, RSteadyRenewal)
	})
	Convey("Write steady setup telescope success correctly", t, func() {
		succ := SteadySucc{
			Block: &sbresv.Block{
				Info: &sbresv.Info{
					BwCls:    13,
					PathType: sibra.PathTypeUp,
					RLC:      4,
					Index:    2,
					ExpTick:  sibra.TimeToTick(time.Now()).Add(4),
				},
				SOFields: []*sbresv.SOField{
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
			},
			DataType: RSteadySetupTelescope,
		}
		b := make(common.RawBytes, succ.Len())
		err := succ.Write(b)
		SoMsg("err", err, ShouldBeNil)
		parsed, err := SteadySuccFromRaw(b, RSteadySetupTelescope, 2)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("Info", parsed.Block.Info, ShouldResemble, succ.Block.Info)
		SoMsg("SOFields", parsed.Block.SOFields, ShouldResemble, succ.Block.SOFields)
		SoMsg("Data", parsed.DataType, ShouldEqual, RSteadySetupTelescope)
	})
}

func TestSteadySucc_SetSOF(t *testing.T) {
	Convey("SetMac and Verify agree", t, func() {
		mac, err := util.InitMac(make(common.RawBytes, 16))
		xtest.FailOnErr(t, err)
		ids := []sibra.ID{sibra.NewSteadyID(addr.AS(0x101010), 0x53)}
		pLens := []uint8{2, 0, 0}
		parsed, err := SteadySuccFromRaw(rawSteadySuccSetup, RSteadySetup, 2)
		xtest.FailOnErr(t, err)
		now := parsed.Block.Info.ExpTick.Add(-1).Time()
		err = parsed.SetSOF(mac, ids, pLens, 12, 14, 0)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("Ingress", parsed.Block.SOFields[0].Ingress, ShouldEqual, common.IFIDType(12))
		SoMsg("Egress", parsed.Block.SOFields[0].Egress, ShouldEqual, common.IFIDType(14))

		Convey("Authentic", func() {
			SoMsg("Verify", parsed.Block.Verify(mac, 0, ids, pLens, now), ShouldBeNil)
		})
		Convey("Unauthentic", func() {
			parsed.Block.SOFields[0].Mac[0] ^= 0x1
			SoMsg("Verify", parsed.Block.Verify(mac, 0, ids, pLens, now), ShouldNotBeNil)
		})
	})
}
