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

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
)

var (
	rawConfirm, _ = hex.DecodeString("110207")
)

func TestConfirmIndexFromRaw(t *testing.T) {
	Convey("Parse failed ephem setup correctly", t, func() {
		conf, err := ConfirmIndexFromRaw(rawConfirm)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("FailHop", conf.FailHop, ShouldEqual, 2)
		SoMsg("FailCode", conf.FailCode, ShouldEqual, InvalidInfo)
		SoMsg("Idx", conf.Idx, ShouldEqual, sibra.Index(1))
		SoMsg("State", conf.State, ShouldEqual, sibra.StatePending)
	})
}

func TestConfirmIndex_Write(t *testing.T) {
	Convey("Write confirm should return correct result", t, func() {
		confirm := &ConfirmIndex{
			FailHop:  2,
			FailCode: InvalidInfo,
			Idx:      1,
			State:    sibra.StatePending,
		}
		b := make(common.RawBytes, confirm.Len())
		err := confirm.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other, err := ConfirmIndexFromRaw(b)
		SoMsg("FailHop", other.FailHop, ShouldEqual, confirm.FailHop)
		SoMsg("FailCode", other.FailCode, ShouldEqual, confirm.FailCode)
		SoMsg("Idx", other.Idx, ShouldEqual, confirm.Idx)
		SoMsg("State", other.State, ShouldEqual, confirm.State)
	})
}
