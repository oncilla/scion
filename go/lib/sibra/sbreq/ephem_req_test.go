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

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

func Test_Fail(t *testing.T) {
	Convey("NewSteadyReq should return correct request", t, func() {
		req := &EphemReq{
			Base: &Base{
				Type:     REphmSetup,
				Accepted: true,
			},
			ReqID: sbresv.NewEphemID(addr.AS(0), nil),
			Block: &sbresv.Block{
				Info:     &sbresv.Info{},
				SOFields: make([]*sbresv.SOField, 10),
			},
		}

		rep := req.Fail(ClientDenied, 10, 9)
		SoMsg("Size mismatch", rep.Len(), ShouldEqual, req.Len())

		buf := make(common.RawBytes, rep.Len())
		err := rep.Write(buf)
		SoMsg("Err write", err, ShouldBeNil)
		p, err := Parse(buf, 10)
		SoMsg("Err parse", err, ShouldBeNil)
		SoMsg("Size mismatch", p.Len(), ShouldEqual, rep.Len())
		SoMsg("Same", p, ShouldResemble, rep)

	})
}
