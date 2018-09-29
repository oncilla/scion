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
)

var (
	rawSteadyReqSetup, _ = hex.DecodeString("020a1e0a021116ebe0d8050421000b0c010c0c01000901000c" +
		"01000c01")
	rawSteadyReqRenew, _ = hex.DecodeString("020a1e0a021116ebe0d8050421000b0c010c0c01000901000c" +
		"01000c01")
	rawSteadyReqBaseID, _   = hex.DecodeString("00000010101000000053")
	rawSteadyReqSetupTel, _ = hex.DecodeString("020a1e0a02110000001010100000005316ebe0dd0504210" +
		"00b0c010c0c01000901000c01000c01")
)

func TestSteadyReqFromRaw(t *testing.T) {
	Convey("Parse steady setup correctly", t, func() {
		req := &SteadyReq{
			FailHop:  2,
			MaxBw:    30,
			MinBw:    10,
			EndProps: sibra.StartLocal | sibra.EndLocal,
			Split:    2,
			AccBw:    10,
			DataType: RSteadySetup,
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  384557272,
				RLC:      4,
				PathType: sibra.PathTypeUp,
			},
			OfferFields: []*Offer{
				{AllocBw: 11, MaxBw: 12, LineLen: 1},
				{AllocBw: 12, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 9, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
			},
		}
		other, err := SteadyReqFromRaw(rawSteadyReqSetup, RSteadySetup, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("Eq", other, ShouldResemble, req)
	})
	Convey("Parse steady renew correctly", t, func() {
		req := &SteadyReq{
			FailHop:  2,
			MaxBw:    30,
			MinBw:    10,
			EndProps: sibra.StartLocal | sibra.EndLocal,
			Split:    2,
			AccBw:    10,
			DataType: RSteadyRenewal,
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  384557272,
				RLC:      4,
				PathType: sibra.PathTypeUp,
			},
			OfferFields: []*Offer{
				{AllocBw: 11, MaxBw: 12, LineLen: 1},
				{AllocBw: 12, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 9, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
			},
		}
		other, err := SteadyReqFromRaw(rawSteadyReqRenew, RSteadyRenewal, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("Eq", other, ShouldResemble, req)
	})
	Convey("Parse steady setup telescope correctly", t, func() {
		req := &SteadyReq{
			BaseID:   sibra.ID(rawSteadyReqBaseID),
			FailHop:  2,
			MaxBw:    30,
			MinBw:    10,
			EndProps: sibra.StartLocal | sibra.EndLocal,
			Split:    2,
			AccBw:    10,
			DataType: RSteadySetupTelescope,
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  384557277,
				RLC:      4,
				PathType: sibra.PathTypeUp,
			},
			OfferFields: []*Offer{
				{AllocBw: 11, MaxBw: 12, LineLen: 1},
				{AllocBw: 12, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 9, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
			},
		}
		other, err := SteadyReqFromRaw(rawSteadyReqSetupTel, RSteadySetupTelescope, 5)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("Eq", other, ShouldResemble, req)
	})
}

func TestSteadyReq_Write(t *testing.T) {
	Convey("Write steady setup should return correct result", t, func() {
		req := &SteadyReq{
			FailHop:  2,
			MaxBw:    30,
			MinBw:    10,
			EndProps: sibra.StartLocal | sibra.EndLocal,
			Split:    2,
			AccBw:    10,
			DataType: RSteadySetup,
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  sibra.CurrentTick().Add(4),
				RLC:      4,
				PathType: sibra.PathTypeUp,
			},
			OfferFields: []*Offer{
				{AllocBw: 11, MaxBw: 12, LineLen: 1},
				{AllocBw: 12, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 9, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
			},
		}
		b := make(common.RawBytes, req.Len())
		err := req.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other, err := SteadyReqFromRaw(b, RSteadySetup, 5)
		SoMsg("Eq", other, ShouldResemble, req)
	})
	Convey("Write steady renew should return correct result", t, func() {
		req := &SteadyReq{
			FailHop:  2,
			MaxBw:    30,
			MinBw:    10,
			EndProps: sibra.StartLocal | sibra.EndLocal,
			Split:    2,
			AccBw:    10,
			DataType: RSteadyRenewal,
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  sibra.CurrentTick().Add(4),
				RLC:      4,
				PathType: sibra.PathTypeUp,
			},
			OfferFields: []*Offer{
				{AllocBw: 11, MaxBw: 12, LineLen: 1},
				{AllocBw: 12, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 9, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
			},
		}
		b := make(common.RawBytes, req.Len())
		err := req.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other, err := SteadyReqFromRaw(b, RSteadyRenewal, 5)
		SoMsg("Eq", other, ShouldResemble, req)
	})
	Convey("Write steady setup telescope should return correct result", t, func() {
		req := &SteadyReq{
			BaseID:   sibra.NewSteadyID(addr.AS(0x101010), 0x53),
			FailHop:  2,
			MaxBw:    30,
			MinBw:    10,
			EndProps: sibra.StartLocal | sibra.EndLocal,
			Split:    2,
			AccBw:    10,
			DataType: RSteadySetupTelescope,
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  sibra.CurrentTick().Add(4),
				RLC:      4,
				PathType: sibra.PathTypeUp,
			},
			OfferFields: []*Offer{
				{AllocBw: 11, MaxBw: 12, LineLen: 1},
				{AllocBw: 12, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 9, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
				{AllocBw: 0, MaxBw: 12, LineLen: 1},
			},
		}
		b := make(common.RawBytes, req.Len())
		err := req.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other, err := SteadyReqFromRaw(b, RSteadySetupTelescope, 5)
		SoMsg("Eq", other, ShouldResemble, req)
	})
}
