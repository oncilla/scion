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
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
)

func TestPld_WritePld(t *testing.T) {
	Convey("Write confirm should return correct result", t, func() {
		pld := Pld{
			Type:     RSteadyConfIndex,
			Accepted: true,
			Data: &ConfirmIndex{
				FailHop:  2,
				FailCode: InvalidInfo,
				Idx:      1,
				State:    sibra.StatePending,
			},
			NumHops: 3,
			Auths: []common.RawBytes{
				make(common.RawBytes, AuthLen),
				make(common.RawBytes, AuthLen),
				make(common.RawBytes, AuthLen),
			},
			TimeStamp: uint32(time.Now().Unix()),
			TotalLen:  59,
			Response:  false,
		}
		b := make(common.RawBytes, pld.Len())
		n, err := pld.WritePld(b)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("n", n, ShouldEqual, pld.TotalLen)
		other, err := PldFromRaw(b)
		SoMsg("parseErr", err, ShouldBeNil)
		SoMsg("Type", other.Type, ShouldEqual, pld.Type)
		SoMsg("Accepted", other.Accepted, ShouldEqual, pld.Accepted)
		SoMsg("Data", other.Data, ShouldResemble, pld.Data)
		SoMsg("NumHops", other.NumHops, ShouldEqual, pld.NumHops)
		SoMsg("Auths", other.Auths, ShouldResemble, pld.Auths)
		SoMsg("TimeStamp", other.TimeStamp, ShouldEqual, pld.TimeStamp)
		SoMsg("TotalLen", other.TotalLen, ShouldEqual, pld.TotalLen)
		SoMsg("Response", other.Response, ShouldEqual, pld.Response)
	})
}

func TestPld_Copy(t *testing.T) {
	Convey("Write confirm should return correct result", t, func() {
		pld := &Pld{
			Type:     RSteadyConfIndex,
			Accepted: true,
			Data: &ConfirmIndex{
				FailHop:  2,
				FailCode: InvalidInfo,
				Idx:      1,
				State:    sibra.StatePending,
			},
			NumHops: 3,
			Auths: []common.RawBytes{
				make(common.RawBytes, AuthLen),
				make(common.RawBytes, AuthLen),
				make(common.RawBytes, AuthLen),
			},
			TimeStamp: uint32(time.Now().Unix()),
			TotalLen:  59,
			Response:  false,
		}
		other, err := pld.Copy()
		SoMsg("err", err, ShouldBeNil)
		SoMsg("Eq", other, ShouldResemble, pld)
	})
}

func TestDataType_Steady(t *testing.T) {
	Convey("Data report correctly if steady or ephemeral", t, func() {
		steady := []Data{&SteadySucc{}, &SteadyReq{}, &ConfirmIndex{}}
		ephem := []Data{&EphemClean{}, &EphemReq{}, &EphemFailed{}}
		Convey("Steady", func() {
			for _, v := range steady {
				SoMsg(v.Type().String(), v.Steady(), ShouldBeTrue)
				SoMsg(v.Type().String(), v.Steady(), ShouldEqual, v.Type().Steady())
			}
		})
		Convey("Ephemeral", func() {
			for _, v := range ephem {
				SoMsg(v.Type().String(), v.Steady(), ShouldBeFalse)
				SoMsg(v.Type().String(), v.Steady(), ShouldEqual, v.Type().Steady())
			}
		})
	})
}
