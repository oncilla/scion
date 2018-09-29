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

package sbextn

import (
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sibra"
	. "github.com/smartystreets/goconvey/convey"
)

func Test_EphemeralRenewal(t *testing.T) {
	Convey("Indexes are correct in forward direction", t, func() {
		ephem := &Ephemeral{
			&Base{
				Forward:   true,
				IsRequest: true,
				Version:   sibra.Version,
				PathLens:  []uint8{2, 3, 2},
				IDs: []sibra.ID{sibra.NewEphemID(addr.AS(0), nil), sibra.NewSteadyID(addr.AS(1), 0),
					sibra.NewSteadyID(addr.AS(2), 1), sibra.NewSteadyID(addr.AS(3), 4)},
			},
		}
		err := ephem.UpdateIndices()
		SoMsg("Initial update indices", err, ShouldBeNil)

		type expVal struct {
			SOFIndex       uint8
			CurrHop        int
			TotalHops      int
			CurrSteady     int
			TotalSteady    int
			RelSteadyHop   int
			CurrBlock      int
			RelSOFIdx      int
			FirstHop       bool
			LastHop        bool
			IsTransfer     bool
			SteadyTransfer bool
		}

		checkFields := func(prefix string, exp expVal) {
			SoMsg(prefix+" SOFIndex", ephem.SOFIndex, ShouldEqual, exp.SOFIndex)
			SoMsg(prefix+" CurrHop", ephem.CurrHop, ShouldEqual, exp.CurrHop)
			SoMsg(prefix+" TotalHops", ephem.TotalHops, ShouldEqual, exp.TotalHops)
			SoMsg(prefix+" CurrSteady", ephem.CurrSteady, ShouldEqual, exp.CurrSteady)
			SoMsg(prefix+" TotalSteady", ephem.TotalSteady, ShouldEqual, exp.TotalSteady)
			SoMsg(prefix+" RelSteadyHop", ephem.RelSteadyHop, ShouldEqual, exp.RelSteadyHop)
			SoMsg(prefix+" CurrBlock", ephem.CurrBlock, ShouldEqual, exp.CurrBlock)
			SoMsg(prefix+" RelSOFIdx", ephem.RelSOFIdx, ShouldEqual, exp.RelSOFIdx)
			SoMsg(prefix+" FirstHop", ephem.FirstHop(), ShouldEqual, exp.FirstHop)
			SoMsg(prefix+" LastHop", ephem.LastHop(), ShouldEqual, exp.LastHop)
			SoMsg(prefix+" IsTransfer", ephem.IsTransfer(), ShouldEqual, exp.IsTransfer)
			SoMsg(prefix+" IsSteadyTransfer", ephem.IsSteadyTransfer(), ShouldEqual, exp.SteadyTransfer)
		}

		checkFields("Hop 0", expVal{
			SOFIndex:     0,
			CurrHop:      0,
			TotalHops:    5,
			CurrSteady:   0,
			TotalSteady:  3,
			RelSteadyHop: 0,
			CurrBlock:    0,
			RelSOFIdx:    0,
			FirstHop:     true,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 1", err, ShouldBeNil)

		checkFields("Hop 1", expVal{
			SOFIndex:       1,
			CurrHop:        1,
			TotalHops:      5,
			CurrSteady:     0,
			TotalSteady:    3,
			RelSteadyHop:   1,
			CurrBlock:      0,
			RelSOFIdx:      1,
			SteadyTransfer: true,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 2", err, ShouldBeNil)

		checkFields("Hop 2", expVal{
			SOFIndex:     2,
			CurrHop:      2,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    0,
			RelSOFIdx:    2,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 3", err, ShouldBeNil)

		checkFields("Hop 3", expVal{
			SOFIndex:       3,
			CurrHop:        3,
			TotalHops:      5,
			CurrSteady:     1,
			TotalSteady:    3,
			RelSteadyHop:   2,
			CurrBlock:      0,
			RelSOFIdx:      3,
			SteadyTransfer: true,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 4", err, ShouldBeNil)

		checkFields("Hop 4", expVal{
			SOFIndex:     4,
			CurrHop:      4,
			TotalHops:    5,
			CurrSteady:   2,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    0,
			RelSOFIdx:    4,
			LastHop:      true,
		})
	})

	Convey("Indexes are correct in reverse direction", t, func() {
		ephem := &Ephemeral{
			&Base{
				IsRequest: true,
				Version:   sibra.Version,
				SOFIndex:  4,
				PathLens:  []uint8{2, 3, 2},
				IDs: []sibra.ID{sibra.NewEphemID(addr.AS(0), nil), sibra.NewSteadyID(addr.AS(1), 0),
					sibra.NewSteadyID(addr.AS(2), 1), sibra.NewSteadyID(addr.AS(3), 4)},
			},
		}
		err := ephem.UpdateIndices()
		SoMsg("Initial update indices", err, ShouldBeNil)

		type expVal struct {
			SOFIndex       uint8
			CurrHop        int
			TotalHops      int
			CurrSteady     int
			TotalSteady    int
			RelSteadyHop   int
			CurrBlock      int
			RelSOFIdx      int
			FirstHop       bool
			LastHop        bool
			IsTransfer     bool
			SteadyTransfer bool
		}

		checkFields := func(prefix string, exp expVal) {
			SoMsg(prefix+" SOFIndex", ephem.SOFIndex, ShouldEqual, exp.SOFIndex)
			SoMsg(prefix+" CurrHop", ephem.CurrHop, ShouldEqual, exp.CurrHop)
			SoMsg(prefix+" TotalHops", ephem.TotalHops, ShouldEqual, exp.TotalHops)
			SoMsg(prefix+" CurrSteady", ephem.CurrSteady, ShouldEqual, exp.CurrSteady)
			SoMsg(prefix+" TotalSteady", ephem.TotalSteady, ShouldEqual, exp.TotalSteady)
			SoMsg(prefix+" RelSteadyHop", ephem.RelSteadyHop, ShouldEqual, exp.RelSteadyHop)
			SoMsg(prefix+" CurrBlock", ephem.CurrBlock, ShouldEqual, exp.CurrBlock)
			SoMsg(prefix+" RelSOFIdx", ephem.RelSOFIdx, ShouldEqual, exp.RelSOFIdx)
			SoMsg(prefix+" FirstHop", ephem.FirstHop(), ShouldEqual, exp.FirstHop)
			SoMsg(prefix+" LastHop", ephem.LastHop(), ShouldEqual, exp.LastHop)
			SoMsg(prefix+" IsTransfer", ephem.IsTransfer(), ShouldEqual, exp.IsTransfer)
			SoMsg(prefix+" IsSteadyTransfer", ephem.IsSteadyTransfer(), ShouldEqual, exp.SteadyTransfer)
		}

		checkFields("Hop 4", expVal{
			SOFIndex:     4,
			CurrHop:      4,
			TotalHops:    5,
			CurrSteady:   2,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    0,
			RelSOFIdx:    4,
			FirstHop:     true,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 3", err, ShouldBeNil)

		checkFields("Hop 3", expVal{
			SOFIndex:       3,
			CurrHop:        3,
			TotalHops:      5,
			CurrSteady:     1,
			TotalSteady:    3,
			RelSteadyHop:   2,
			CurrBlock:      0,
			RelSOFIdx:      3,
			SteadyTransfer: true,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 2", err, ShouldBeNil)

		checkFields("Hop 2", expVal{
			SOFIndex:     2,
			CurrHop:      2,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    0,
			RelSOFIdx:    2,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 1", err, ShouldBeNil)

		checkFields("Hop 1", expVal{
			SOFIndex:       1,
			CurrHop:        1,
			TotalHops:      5,
			CurrSteady:     0,
			TotalSteady:    3,
			RelSteadyHop:   1,
			CurrBlock:      0,
			RelSOFIdx:      1,
			SteadyTransfer: true,
		})

		err = ephem.NextSOFIndex()
		SoMsg("NextSOF 0", err, ShouldBeNil)

		checkFields("Hop 0", expVal{
			SOFIndex:     0,
			CurrHop:      0,
			TotalHops:    5,
			CurrSteady:   0,
			TotalSteady:  3,
			RelSteadyHop: 0,
			CurrBlock:    0,
			RelSOFIdx:    0,
			LastHop:      true,
		})
	})

}
