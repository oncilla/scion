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
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/xtest"
)

func TestEphemFailed_Write(t *testing.T) {
	Convey("NewSteadyReq should return correct request", t, func() {
		failed := &EphemFailed{
			ID:       sibra.NewEphemIDRand(addr.AS(0)),
			Info:     &sbresv.Info{},
			DataLen:  100,
			FailHop:  3,
			FailCode: BwExceeded,
			Offers:   make([]sibra.BwCls, 5),
		}
		b := make(common.RawBytes, 100)
		err := failed.Write(b)
		xtest.FailOnErr(t, err)
		other, err := EphemFailedFromRaw(b, true, 5)
		SoMsg("FailHop", other.FailHop, ShouldEqual, failed.FailHop)
		SoMsg("FailCode", other.FailCode, ShouldEqual, failed.FailCode)
	})
}
