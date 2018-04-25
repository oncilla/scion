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

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

func Test_EphemeralSetup(t *testing.T) {
	Convey("Indexes are correct in forward direction", t, func() {
		steady := &Steady{
			&Base{
				Steady:    true,
				Forward:   true,
				IsRequest: true,
				Version:   Version,
				PathLens:  []uint8{2, 3, 2},
				IDs: []sbresv.ID{sbresv.NewSteadyID(addr.AS(1), 0),
					sbresv.NewSteadyID(addr.AS(2), 1), sbresv.NewSteadyID(addr.AS(3), 4)},
				Request: &sbreq.EphemReq{
					ReqID: sbresv.NewEphemID(addr.AS(4), nil),
					Block: &sbresv.Block{
						Info:     &sbresv.Info{},
						SOFields: make([]*sbresv.SOField, 5),
					},
				},
			},
		}
		err := steady.UpdateIndices()
		SoMsg("Initial update indices", err, ShouldBeNil)

		type expVal struct {
			SOFIndex     uint8
			CurrHop      int
			TotalHops    int
			CurrSteady   int
			TotalSteady  int
			RelSteadyHop int
			CurrBlock    int
			RelSOFIdx    int
			FirstHop     bool
			LastHop      bool
			IsTransfer   bool
		}

		checkFields := func(prefix string, exp expVal) {
			SoMsg(prefix+" SOFIndex", steady.SOFIndex, ShouldEqual, exp.SOFIndex)
			SoMsg(prefix+" CurrHop", steady.CurrHop, ShouldEqual, exp.CurrHop)
			SoMsg(prefix+" TotalHops", steady.TotalHops, ShouldEqual, exp.TotalHops)
			SoMsg(prefix+" CurrSteady", steady.CurrSteady, ShouldEqual, exp.CurrSteady)
			SoMsg(prefix+" TotalSteady", steady.TotalSteady, ShouldEqual, exp.TotalSteady)
			SoMsg(prefix+" RelSteadyHop", steady.RelSteadyHop, ShouldEqual, exp.RelSteadyHop)
			SoMsg(prefix+" CurrBlock", steady.CurrBlock, ShouldEqual, exp.CurrBlock)
			SoMsg(prefix+" RelSOFIdx", steady.RelSOFIdx, ShouldEqual, exp.RelSOFIdx)
			SoMsg(prefix+" FirstHop", steady.FirstHop(), ShouldEqual, exp.FirstHop)
			SoMsg(prefix+" LastHop", steady.LastHop(), ShouldEqual, exp.LastHop)
			SoMsg(prefix+" Transfer", steady.IsTransfer(), ShouldEqual, exp.IsTransfer)
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

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 1", err, ShouldBeNil)

		checkFields("Hop 1", expVal{
			SOFIndex:     1,
			CurrHop:      1,
			TotalHops:    5,
			CurrSteady:   0,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    0,
			RelSOFIdx:    1,
			IsTransfer:   true,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 2", err, ShouldBeNil)

		checkFields("Hop 2", expVal{
			SOFIndex:     2,
			CurrHop:      1,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 0,
			CurrBlock:    1,
			RelSOFIdx:    0,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 3", err, ShouldBeNil)

		checkFields("Hop 3", expVal{
			SOFIndex:     3,
			CurrHop:      2,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    1,
			RelSOFIdx:    1,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 4", err, ShouldBeNil)

		checkFields("Hop 4", expVal{
			SOFIndex:     4,
			CurrHop:      3,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 2,
			CurrBlock:    1,
			RelSOFIdx:    2,
			IsTransfer:   true,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 5", err, ShouldBeNil)

		checkFields("Hop 5", expVal{
			SOFIndex:     5,
			CurrHop:      3,
			TotalHops:    5,
			CurrSteady:   2,
			TotalSteady:  3,
			RelSteadyHop: 0,
			CurrBlock:    2,
			RelSOFIdx:    0,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 6", err, ShouldBeNil)

		checkFields("Hop 6", expVal{
			SOFIndex:     6,
			CurrHop:      4,
			TotalHops:    5,
			CurrSteady:   2,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    2,
			RelSOFIdx:    1,
			LastHop:      true,
		})

	})

	Convey("Indexes are correct in reverse direction", t, func() {
		steady := &Steady{
			&Base{
				Steady:    true,
				Forward:   false,
				IsRequest: true,
				Version:   Version,
				SOFIndex:  6,
				PathLens:  []uint8{2, 3, 2},
				IDs: []sbresv.ID{sbresv.NewSteadyID(addr.AS(1), 0),
					sbresv.NewSteadyID(addr.AS(2), 1), sbresv.NewSteadyID(addr.AS(3), 4)},
				Request: &sbreq.EphemReq{
					ReqID: sbresv.NewEphemID(addr.AS(4), nil),
					Block: &sbresv.Block{
						Info:     &sbresv.Info{},
						SOFields: make([]*sbresv.SOField, 5),
					},
				},
			},
		}
		err := steady.UpdateIndices()
		SoMsg("Initial update indices", err, ShouldBeNil)

		type expVal struct {
			SOFIndex     uint8
			CurrHop      int
			TotalHops    int
			CurrSteady   int
			TotalSteady  int
			RelSteadyHop int
			CurrBlock    int
			RelSOFIdx    int
			FirstHop     bool
			LastHop      bool
			IsTransfer   bool
		}

		checkFields := func(prefix string, exp expVal) {
			SoMsg(prefix+" SOFIndex", steady.SOFIndex, ShouldEqual, exp.SOFIndex)
			SoMsg(prefix+" CurrHop", steady.CurrHop, ShouldEqual, exp.CurrHop)
			SoMsg(prefix+" TotalHops", steady.TotalHops, ShouldEqual, exp.TotalHops)
			SoMsg(prefix+" CurrSteady", steady.CurrSteady, ShouldEqual, exp.CurrSteady)
			SoMsg(prefix+" TotalSteady", steady.TotalSteady, ShouldEqual, exp.TotalSteady)
			SoMsg(prefix+" RelSteadyHop", steady.RelSteadyHop, ShouldEqual, exp.RelSteadyHop)
			SoMsg(prefix+" CurrBlock", steady.CurrBlock, ShouldEqual, exp.CurrBlock)
			SoMsg(prefix+" RelSOFIdx", steady.RelSOFIdx, ShouldEqual, exp.RelSOFIdx)
			SoMsg(prefix+" FirstHop", steady.FirstHop(), ShouldEqual, exp.FirstHop)
			SoMsg(prefix+" LastHop", steady.LastHop(), ShouldEqual, exp.LastHop)
			SoMsg(prefix+" Transfer", steady.IsTransfer(), ShouldEqual, exp.IsTransfer)
		}

		checkFields("Hop 6", expVal{
			SOFIndex:     6,
			CurrHop:      4,
			TotalHops:    5,
			CurrSteady:   2,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    2,
			RelSOFIdx:    1,
			FirstHop:     true,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 5", err, ShouldBeNil)

		checkFields("Hop 5", expVal{
			SOFIndex:     5,
			CurrHop:      3,
			TotalHops:    5,
			CurrSteady:   2,
			TotalSteady:  3,
			RelSteadyHop: 0,
			CurrBlock:    2,
			RelSOFIdx:    0,
			IsTransfer:   true,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 4", err, ShouldBeNil)

		checkFields("Hop 4", expVal{
			SOFIndex:     4,
			CurrHop:      3,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 2,
			CurrBlock:    1,
			RelSOFIdx:    2,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 3", err, ShouldBeNil)

		checkFields("Hop 3", expVal{
			SOFIndex:     3,
			CurrHop:      2,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    1,
			RelSOFIdx:    1,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 2", err, ShouldBeNil)

		checkFields("Hop 2", expVal{
			SOFIndex:     2,
			CurrHop:      1,
			TotalHops:    5,
			CurrSteady:   1,
			TotalSteady:  3,
			RelSteadyHop: 0,
			CurrBlock:    1,
			RelSOFIdx:    0,
			IsTransfer:   true,
		})

		err = steady.NextSOFIndex()
		SoMsg("NextSOF 1", err, ShouldBeNil)

		checkFields("Hop 1", expVal{
			SOFIndex:     1,
			CurrHop:      1,
			TotalHops:    5,
			CurrSteady:   0,
			TotalSteady:  3,
			RelSteadyHop: 1,
			CurrBlock:    0,
			RelSOFIdx:    1,
		})

		err = steady.NextSOFIndex()
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

/* // TODO(roosd): implement tests
func Test_NewSteadySetup(t *testing.T) {
	Convey("NewSteadySetup should return correct extn", t, func() {
		r := req.NewSteadyReq(req.RSteadySetup, 1, 2, 3, resv.PathTypeUp, 2)
		SoMsg("type", r.Base.Type, ShouldEqual, req.RSteadySetup)
		e, err := NewSteadySetup(r, resv.ID{1, 2, 3, 4, 5, 6, 7, 8, 9, 0})
		SoMsg("err", err, ShouldBeNil)
		SoMsg("len", e.Len(), ShouldEqual, common.ExtnFirstLineLen+16+(2+1)*common.LineLen)
		SoMsg("Steady", e.Steady, ShouldBeTrue)
		SoMsg("Setup", e.Setup, ShouldBeTrue)
		SoMsg("Forward", e.Forward, ShouldBeTrue)
		SoMsg("BestEffort", e.BestEffort, ShouldBeFalse)
		SoMsg("IsRequest", e.IsRequest, ShouldBeTrue)
		SoMsg("Version", e.Version, ShouldEqual, 0)
		SoMsg("SOFIndex", e.SOFIndex, ShouldEqual, 0)
		SoMsg("PathLens", e.PathLens, ShouldResemble, []byte{2, 0, 0})
		SoMsg("IDs", e.IDs, ShouldResemble, []resv.ID{{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}})
		SoMsg("ActiveBlocks", e.ActiveBlocks, ShouldResemble, []*resv.Block{})
		SoMsg("Request", e.Request, ShouldResemble, r)
		SoMsg("CurrHop", e.CurrHop, ShouldResemble, 0)
		SoMsg("TotalHops", e.TotalHops, ShouldResemble, 2)
		SoMsg("CurrSteady", e.CurrSteady, ShouldResemble, 0)
		SoMsg("RelSteadyHop", e.RelSteadyHop, ShouldResemble, 0)
		SoMsg("TotalSteady", e.TotalSteady, ShouldResemble, 1)
	})
}


func Test_SteadyExtnPack(t *testing.T) {
	Convey("NewSteadySetup should return correct extn", t, func() {
		r := req.NewSteadyReq(req.RSteadySetup, 1, 2, 3, resv.PathTypeUp, 2)
		SoMsg("type", r.Base.Type, ShouldEqual, req.RSteadySetup)
		e, err := NewSteadySetup(r, resv.ID{1, 2, 3, 4, 5, 6, 7, 8, 9, 0})
		SoMsg("err", err, ShouldBeNil)
		err = e.NextSOFIndex()
		SoMsg("err", err, ShouldBeNil)
		packed, err := e.Pack()
		SoMsg("err", err, ShouldBeNil)
		parsed, err := SteadyFromRaw(packed)
		SoMsg("err", err, ShouldBeNil)

		SoMsg("len", parsed.Len(), ShouldEqual, common.ExtnFirstLineLen+16+(2+1)*common.LineLen)
		SoMsg("Steady", parsed.Steady, ShouldBeTrue)
		SoMsg("Setup", parsed.Setup, ShouldBeTrue)
		SoMsg("Forward", parsed.Forward, ShouldBeTrue)
		SoMsg("BestEffort", parsed.BestEffort, ShouldBeFalse)
		SoMsg("IsRequest", parsed.IsRequest, ShouldBeTrue)
		SoMsg("Version", parsed.Version, ShouldEqual, 0)
		SoMsg("SOFIndex", parsed.SOFIndex, ShouldEqual, 1)
		SoMsg("PathLens", parsed.PathLens, ShouldResemble, []byte{2, 0, 0})
		SoMsg("IDs", parsed.IDs, ShouldResemble, []resv.ID{{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}})
		SoMsg("ActiveBlocks", parsed.ActiveBlocks, ShouldResemble, []*resv.Block{})
		SoMsg("Request", parsed.Request, ShouldResemble, r)
		SoMsg("CurrHop", parsed.CurrHop, ShouldResemble, 1)
		SoMsg("TotalHops", parsed.TotalHops, ShouldResemble, 2)
		SoMsg("CurrSteady", parsed.CurrSteady, ShouldResemble, 0)
		SoMsg("RelSteadyHop", parsed.RelSteadyHop, ShouldResemble, 1)
		SoMsg("TotalSteady", parsed.TotalSteady, ShouldResemble, 1)
	})
}*/
