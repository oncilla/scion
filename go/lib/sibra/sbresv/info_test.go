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

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
)

var rawInfo, _ = hex.DecodeString("16ebdb4f0d042400")

func TestNewInfoFromRaw(t *testing.T) {
	Convey("Parse Info correctly", t, func() {
		info := &Info{
			BwCls:    13,
			PathType: sibra.PathTypeEphemeral,
			RLC:      4,
			Index:    2,
			ExpTick:  384555855,
		}
		other := NewInfoFromRaw(rawInfo)
		SoMsg("BwCls", other.BwCls, ShouldEqual, info.BwCls)
		SoMsg("PathType", other.PathType, ShouldEqual, info.PathType)
		SoMsg("RLC", other.RLC, ShouldEqual, info.RLC)
		SoMsg("Index", other.Index, ShouldEqual, info.Index)
		SoMsg("ExpTick", other.ExpTick, ShouldEqual, info.ExpTick)
	})
}

func TestInfo_Write(t *testing.T) {
	Convey("Write Info correctly", t, func() {
		info := &Info{
			BwCls:    13,
			PathType: sibra.PathTypeEphemeral,
			RLC:      4,
			Index:    2,
			ExpTick:  sibra.CurrentTick().Add(4),
		}
		b := make(common.RawBytes, info.Len())
		err := info.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other := NewInfoFromRaw(b)
		SoMsg("BwCls", other.BwCls, ShouldEqual, info.BwCls)
		SoMsg("PathType", other.PathType, ShouldEqual, info.PathType)
		SoMsg("RLC", other.RLC, ShouldEqual, info.RLC)
		SoMsg("Index", other.Index, ShouldEqual, info.Index)
		SoMsg("ExpTick", other.ExpTick, ShouldEqual, info.ExpTick)
	})
}

func TestInfo_Copy(t *testing.T) {
	Convey("Copy Info correctly", t, func() {
		info := &Info{
			BwCls:    13,
			PathType: sibra.PathTypeEphemeral,
			RLC:      4,
			Index:    2,
			ExpTick:  sibra.CurrentTick().Add(4),
		}
		copy := info.Copy()
		SoMsg("equal", copy, ShouldResemble, info)
	})
}
