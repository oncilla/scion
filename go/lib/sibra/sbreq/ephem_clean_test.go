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
	rawEphemCleanSetupID, _ = hex.DecodeString("0000001010104d65822107fcfd52341e")
	rawEphemCleanSetup, _   = hex.DecodeString("010000001010104d65822107fcfd52341e16ebdffd05042400")
	rawEphemCleanRenew, _   = hex.DecodeString("0016ebdfff05042400")
)

func TestEphemCleanFromRaw(t *testing.T) {
	Convey("Parse failed ephem setup correctly", t, func() {
		clean, err := EphemCleanFromRaw(rawEphemCleanSetup)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("ID", clean.ID, ShouldResemble, sibra.ID(rawEphemCleanSetupID))
		SoMsg("Info", clean.Info, ShouldResemble, &sbresv.Info{
			Index:    2,
			BwCls:    5,
			ExpTick:  384557053,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		})
		SoMsg("Setup", clean.Setup, ShouldBeTrue)
	})
	Convey("Parse failed ephem renew correctly", t, func() {
		clean, err := EphemCleanFromRaw(rawEphemCleanRenew)
		SoMsg("err", err, ShouldBeNil)
		SoMsg("ID", clean.ID, ShouldBeNil)
		SoMsg("Info", clean.Info, ShouldResemble, &sbresv.Info{
			Index:    2,
			BwCls:    5,
			ExpTick:  384557055,
			RLC:      4,
			PathType: sibra.PathTypeEphemeral,
		})
		SoMsg("Setup", clean.Setup, ShouldBeFalse)
	})
}

func TestEphemClean_Write(t *testing.T) {
	Convey("Write ephem clean setup should return correct result", t, func() {
		clean := &EphemClean{
			ID: sibra.NewEphemIDRand(addr.AS(0x101010)),
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  sibra.CurrentTick().Add(4),
				RLC:      4,
				PathType: sibra.PathTypeEphemeral,
			},
			Setup: true,
		}

		b := make(common.RawBytes, clean.Len())
		err := clean.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other, err := EphemCleanFromRaw(b)
		SoMsg("ID", other.ID, ShouldResemble, clean.ID)
		SoMsg("Info", other.Info, ShouldResemble, clean.Info)
		SoMsg("Setup", other.Setup, ShouldBeTrue)
	})
	Convey("Write ephem clean renew should return correct result", t, func() {
		clean := &EphemClean{
			Info: &sbresv.Info{
				Index:    2,
				BwCls:    5,
				ExpTick:  sibra.CurrentTick().Add(4),
				RLC:      4,
				PathType: sibra.PathTypeEphemeral,
			},
		}

		b := make(common.RawBytes, clean.Len())
		err := clean.Write(b)
		SoMsg("err", err, ShouldBeNil)
		other, err := EphemCleanFromRaw(b)
		SoMsg("ID", other.ID, ShouldBeNil)
		SoMsg("Info", other.Info, ShouldResemble, clean.Info)
		SoMsg("Setup", other.Setup, ShouldBeFalse)
	})
}
