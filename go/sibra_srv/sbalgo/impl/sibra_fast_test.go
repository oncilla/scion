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

package impl

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/lib/xtest"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

func TestAlgoFast_Available(t *testing.T) {
	Convey("Available is calculated correctly", t, func() {
		s := loadAlgoFast(t)
		ifids := sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 81,
		}
		p := setupParams(ifids, s.Topo.ISD_AS, 0, 27, sibra.PathTypeUp, 10, 3)
		Convey("Initial available is correct", func() {
			in := s.Infos[p.Ifids.InIfid].Ingress.Total
			eg := s.Infos[p.Ifids.EgIfid].Egress.Total
			min := sibra.Bps(float64(minBps(in, eg)) * s.Delta)
			SoMsg("avail", s.Available(p.Ifids, p.Extn.GetCurrID()), ShouldEqual, min)
		})
		_, err := s.AdmitSteady(p)
		xtest.FailOnErr(t, err)
		Convey("Available is the same", func() {
			in := s.Infos[p.Ifids.InIfid].Ingress.Total
			eg := s.Infos[p.Ifids.EgIfid].Egress.Total
			min := sibra.Bps(float64(minBps(in, eg)) * s.Delta)
			SoMsg("avail", s.Available(p.Ifids, p.Extn.GetCurrID()), ShouldEqual, min)
		})
		p = setupParams(ifids, s.Topo.ISD_AS, 1, 27, sibra.PathTypeUp, 10, 3)
		Convey("Available has decreased", func() {
			reserved := sibra.BwCls(27).Bps()
			in := s.Infos[p.Ifids.InIfid].Ingress.Total - reserved
			eg := s.Infos[p.Ifids.EgIfid].Egress.Total - reserved
			min := sibra.Bps(float64(minBps(in, eg)) * s.Delta)
			SoMsg("avail", s.Available(p.Ifids, p.Extn.GetCurrID()), ShouldEqual, min)
		})
		ifids = sbalgo.IFTuple{
			InIfid: 16,
			EgIfid: 81,
		}
		p = setupParams(ifids, s.Topo.ISD_AS, 1, 27, sibra.PathTypeUp, 10, 3)
		Convey("Available affected", func() {
			in := s.Infos[p.Ifids.InIfid].Ingress.Total
			eg := s.Infos[p.Ifids.EgIfid].Egress.Total
			min := sibra.Bps(float64(minBps(in, eg)) * s.Delta)
			SoMsg("avail", s.Available(p.Ifids, p.Extn.GetCurrID()), ShouldBeLessThan, min)
		})
		ifids = sbalgo.IFTuple{
			InIfid: 81,
			EgIfid: 16,
		}
		p = setupParams(ifids, s.Topo.ISD_AS, 1, 27, sibra.PathTypeDown, 10, 3)
		Convey("Available not affected", func() {
			in := s.Infos[p.Ifids.InIfid].Ingress.Total
			eg := s.Infos[p.Ifids.EgIfid].Egress.Total
			min := sibra.Bps(float64(minBps(in, eg)) * s.Delta)
			SoMsg("avail", s.Available(p.Ifids, p.Extn.GetCurrID()), ShouldEqual, min)
		})

	})
}

func TestAlgoFast_transDem(t *testing.T) {
	Convey("Transit demand is calculated correctly", t, func() {
		s := loadAlgoFast(t)
		ifidsLocal := sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 81,
		}
		p := setupParams(ifidsLocal, s.Topo.ISD_AS, 0, 27, sibra.PathTypeUp, 10, 3)
		c := &calcState{
			AdmParams: p,
		}
		s.setCalcState(c)
		Convey("No transit demand except for request", func() {
			for in := range s.Infos {
				for eg := range s.Infos {
					tup := sbalgo.IFTuple{
						InIfid: in,
						EgIfid: eg,
					}
					str := fmt.Sprintf("TransDem(%d->%d)", in, eg)
					if tup != ifidsLocal {
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, 0)
					} else {
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, p.Req.MaxBw.Bps())
					}
				}
			}
		})
		_, err := s.AdmitSteady(p)
		xtest.FailOnErr(t, err)
		Convey("Transit demand ignores current reservation", func() {
			s.Lock()
			defer s.Unlock()
			for in := range s.Infos {
				for eg := range s.Infos {
					tup := sbalgo.IFTuple{
						InIfid: in,
						EgIfid: eg,
					}
					str := fmt.Sprintf("TransDem(%d->%d)", in, eg)
					if tup != ifidsLocal {
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, 0)
					} else {
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, p.Req.MaxBw.Bps())
					}
				}
			}
		})
		p = setupParams(ifidsLocal, s.Topo.ISD_AS, 1, 27, sibra.PathTypeUp, 10, 3)
		c = &calcState{
			AdmParams: p,
		}
		s.setCalcState(c)
		Convey("Transit demand considers existing reservation", func() {
			s.Lock()
			defer s.Unlock()
			for in := range s.Infos {
				for eg := range s.Infos {
					tup := sbalgo.IFTuple{
						InIfid: in,
						EgIfid: eg,
					}
					str := fmt.Sprintf("TransDem(%d->%d)", in, eg)
					if tup != ifidsLocal {
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, 0)
					} else {
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, p.Req.MaxBw.Bps()*2)
					}
				}
			}
		})
		ifidsUp := sbalgo.IFTuple{
			InIfid: 16,
			EgIfid: 81,
		}
		up := s.Topo.ISD_AS
		up.A++
		p = setupParams(ifidsUp, up, 1, 27, sibra.PathTypeUp, 10, 3)
		c = &calcState{
			AdmParams: p,
		}
		s.setCalcState(c)
		Convey("Transit demand considers other interface pairs", func() {
			s.Lock()
			defer s.Unlock()
			for in := range s.Infos {
				for eg := range s.Infos {
					tup := sbalgo.IFTuple{
						InIfid: in,
						EgIfid: eg,
					}
					str := fmt.Sprintf("TransDem(%d->%d)", in, eg)
					switch {
					case tup == ifidsUp:
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, p.Req.MaxBw.Bps())
					case tup == ifidsLocal:
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, p.Req.MaxBw.Bps())
					default:
						SoMsg(str, s.capTempTransDem(tup, c), ShouldEqual, 0)
					}
				}
			}
		})
		_, err = s.AdmitSteady(p)
		xtest.FailOnErr(t, err)

	})
}

func TestAlgoFast_tubeRatio(t *testing.T) {
	Convey("Tube ratio is calculated correctly", t, func() {
		s := loadAlgoFast(t)
		ifidsLocal := sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 81,
		}
		for i := 0; i < 10; i++ {
			p := setupParams(ifidsLocal, s.Topo.ISD_AS, uint32(i), 10, sibra.PathTypeUp, 10, 3)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
		}

		ifidsUp := sbalgo.IFTuple{
			InIfid: 16,
			EgIfid: 81,
		}
		up := s.Topo.ISD_AS
		up.A++
		bps := sibra.BwCls(10).Bps()
		otherDemand := float64(10 * bps)
		for i := 0; i < 10; i++ {
			p := setupParams(ifidsUp, up, uint32(i), 10, sibra.PathTypeUp, 10, 3)
			c := &calcState{
				AdmParams: p,
			}
			s.setCalcState(c)
			ratio := s.tubeRatio(c)
			thisDemand := float64(sibra.Bps(i+1) * bps)
			SoMsg("Ratio", ratio, ShouldEqual, (thisDemand)/(thisDemand+otherDemand))
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
		}
	})
}

func TestAlgoFast_Fuzzing(t *testing.T) {
	Convey("Fuzzing does not panic", t, func() {
		s := loadAlgoFast(t)
		for i := 0; i < 100; i++ {
			tu := ifids[rand.Int()%len(ifids)]
			bw := sibra.BwCls(rand.Int()%27) + 1
			p := setupParams(tu.ifids, s.Topo.ISD_AS, uint32(i), bw, tu.pathType, 10, 3)
			p.Req.Info.ExpTick = sibra.CurrentTick().Add(1)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
		}

		time.Sleep(10 * time.Second)

		for i := 0; i < 100; i++ {
			tu := ifids[rand.Int()%len(ifids)]
			bw := sibra.BwCls(rand.Int()%27) + 1
			p := setupParams(tu.ifids, s.Topo.ISD_AS, uint32(i+1000), bw, tu.pathType, 10, 3)
			p.Req.Info.ExpTick = sibra.CurrentTick().Add(1)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
		}

		time.Sleep(10 * time.Second)

		for i := 0; i < 100; i++ {
			tu := ifids[rand.Int()%len(ifids)]
			bw := sibra.BwCls(rand.Int()%27) + 1
			p := setupParams(tu.ifids, s.Topo.ISD_AS, uint32(i+2000), bw, tu.pathType, 10, 3)
			p.Req.Info.ExpTick = sibra.CurrentTick().Add(1)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
		}
	})
}

func loadAlgoFast(t *testing.T) *AlgoFast {
	topo, err := topology.LoadFromFile(filepath.Join("testdata", topology.CfgName))
	xtest.FailOnErr(t, err)
	mat, err := state.MatrixFromFile(filepath.Join("testdata", state.MatrixName))
	xtest.FailOnErr(t, err)
	s, err := NewSibraFast(topo, mat)
	xtest.FailOnErr(t, err)
	return s
}

type tup struct {
	ifids    sbalgo.IFTuple
	pathType sibra.PathType
}

var ifids = []tup{
	{
		ifids: sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 16,
		},
		pathType: sibra.PathTypeDown,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 17,
		},
		pathType: sibra.PathTypePeerUp,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 81,
		},
		pathType: sibra.PathTypeUp,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 16,
			EgIfid: 0,
		},
		pathType: sibra.PathTypeUp,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 16,
			EgIfid: 17,
		},
		pathType: sibra.PathTypePeerUp,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 16,
			EgIfid: 81,
		},
		pathType: sibra.PathTypeUp,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 17,
			EgIfid: 0,
		},
		pathType: sibra.PathTypePeerUp,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 17,
			EgIfid: 16,
		},
		pathType: sibra.PathTypePeerDown,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 81,
			EgIfid: 0,
		},
		pathType: sibra.PathTypeDown,
	},
	{
		ifids: sbalgo.IFTuple{
			InIfid: 81,
			EgIfid: 16,
		},
		pathType: sibra.PathTypeDown,
	},
}
