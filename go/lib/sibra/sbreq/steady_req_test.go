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

/* TODO(roosd): implement tests
func Test_NewSteadyResvReq(t *testing.T) {
	Convey("NewSteadyReq should return correct request", t, func() {
		req := NewSteadyReq(RSteadySetup, 1, 2, 3, sbresv.PathTypeUp, 20)
		SoMsg("len", req.Len(), ShouldEqual, (2+10)*common.LineLen)
		SoMsg("type", req.Base.Type, ShouldEqual, RSteadySetup)


		req = NewSteadyReq(RSteadySetup, 1, 2, 3, sbresv.PathTypeUp, 21)
		SoMsg("len", req.Len(), ShouldEqual, (2+11)*common.LineLen)
		SoMsg("type", req.Base.Type, ShouldEqual, RSteadySetup)
	})
}

func Test_SteadyResvReq_Write(t *testing.T) {
	Convey("SteadyReq.Write should write correctly", t, func() {
		req := NewSteadyReq(RSteadySetup, 1, 2, 3, sbresv.PathTypeUp, 21)
		b := make(common.RawBytes, req.Len())
		err := req.Write(b)
		SoMsg("err", err, ShouldBeNil)
		req2, err := SteadyResvReqFromRaw(b, 21)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("type", req.Type, ShouldEqual, req2.Type)
		SoMsg("acc", req.Accepted, ShouldEqual, req2.Accepted)
		SoMsg("resp", req.Response, ShouldEqual, req2.Response)
		SoMsg("info", req.Info, ShouldResemble, req2.Info)

	})
}
*/
