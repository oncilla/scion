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
	"path/filepath"
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/lib/xtest"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

func TestAlgoSlow_Available(t *testing.T) {
	Convey("Available is calculated correctly", t, func() {
		s := loadAlgoSlow(t)
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

func TestAlgoSlow_transDem(t *testing.T) {
	Convey("Transit demand is calculated correctly", t, func() {
		s := loadAlgoSlow(t)
		ifidsLocal := sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 81,
		}
		p := setupParams(ifidsLocal, s.Topo.ISD_AS, 0, 27, sibra.PathTypeUp, 10, 3)
		Convey("No transit demand except for request", func() {
			for in := range s.Infos {
				for eg := range s.Infos {
					tup := sbalgo.IFTuple{
						InIfid: in,
						EgIfid: eg,
					}
					str := fmt.Sprintf("TransDem(%d->%d)", in, eg)
					if tup != ifidsLocal {
						SoMsg(str, s.transDem(tup, p), ShouldEqual, 0)
					} else {
						SoMsg(str, s.transDem(tup, p), ShouldEqual, p.Req.MaxBw.Bps())
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
						SoMsg(str, s.transDem(tup, p), ShouldEqual, 0)
					} else {
						SoMsg(str, s.transDem(tup, p), ShouldEqual, p.Req.MaxBw.Bps())
					}
				}
			}
		})
		p = setupParams(ifidsLocal, s.Topo.ISD_AS, 1, 27, sibra.PathTypeUp, 10, 3)
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
						SoMsg(str, s.transDem(tup, p), ShouldEqual, 0)
					} else {
						SoMsg(str, s.transDem(tup, p), ShouldEqual, p.Req.MaxBw.Bps()*2)
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
						SoMsg(str, s.transDem(tup, p), ShouldEqual, p.Req.MaxBw.Bps())
					case tup == ifidsLocal:
						SoMsg(str, s.transDem(tup, p), ShouldEqual, p.Req.MaxBw.Bps())
					default:
						SoMsg(str, s.transDem(tup, p), ShouldEqual, 0)
					}
				}
			}
		})
		_, err = s.AdmitSteady(p)
		xtest.FailOnErr(t, err)

	})
}

func TestAlgoSlow_inDem(t *testing.T) {
	Convey("Ingress demand is calculated correctly", t, func() {
		s := loadAlgoSlow(t)
		ifidsLocal := sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 81,
		}
		p := setupParams(ifidsLocal, s.Topo.ISD_AS, 0, 10, sibra.PathTypeUp, 10, 3)
		bps := sibra.BwCls(10).Bps()

		checker := inDemChecker{
			prefix: "Initial ",
			ifids:  ifidsLocal,
			s:      s,
		}
		Convey("Initial in demand is correct for local", func() {
			checker.checkOwnInDem(bps, p)
		})
		for i := 0; i < 10; i++ {
			p := setupParams(ifidsLocal, s.Topo.ISD_AS, uint32(i), 10, sibra.PathTypeUp, 10, 3)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
			checker.prefix = "Check " + strconv.Itoa(i) + " "
			checker.checkOwnInDem(bps*sibra.Bps(i+1), p)
		}

		ifidsDown := sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 16,
		}
		up := s.Topo.ISD_AS
		up.A++
		p = setupParams(ifidsDown, up, 0, 10, sibra.PathTypeDown, 10, 3)
		exp := map[addr.IA]sibra.Bps{
			s.Topo.ISD_AS: 10 * bps,
		}
		checker = inDemChecker{
			prefix: "Down ",
			ifids:  ifidsDown,
			s:      s,
		}
		Convey("Initial in demand is correct for down", func() {
			checker.checkOwnInDem(bps, p)
		})
		Convey("Initial in demand is correct for other sources", func() {
			checker.checkOthersInDem(exp, p)
		})
		for i := 0; i < 10; i++ {
			p := setupParams(ifidsDown, up, uint32(i), 10, sibra.PathTypeDown, 10, 3)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
			checker.prefix = "Check " + strconv.Itoa(i) + " "
			checker.checkOwnInDem(bps*sibra.Bps(i+1), p)
			checker.checkOthersInDem(exp, p)
		}

	})
}

type inDemChecker struct {
	s      *AlgoSlow
	prefix string
	ifids  sbalgo.IFTuple
}

func (c *inDemChecker) checkOwnInDem(exp sibra.Bps, p sbalgo.AdmParams) {
	for in := range c.s.Infos {
		str := fmt.Sprintf(c.prefix+"own InDem(%d-src%s)", in, p.Src)
		if in != c.ifids.InIfid {
			SoMsg(str, c.s.inDem(p.Src, in, p), ShouldEqual, 0)
		} else {
			SoMsg(str, c.s.inDem(p.Src, in, p), ShouldEqual, exp)
		}
	}
}

func (c *inDemChecker) checkOthersInDem(exp map[addr.IA]sibra.Bps, p sbalgo.AdmParams) {
	in := c.ifids.InIfid
	for other, expDem := range exp {
		str := fmt.Sprintf(c.prefix+"others InDem(%d-src%s)", in, other)
		SoMsg(str, c.s.inDem(other, in, p), ShouldEqual, expDem)
	}
}

func TestAlgoSlow_egDem(t *testing.T) {
	Convey("Egress demand is calculated correctly", t, func() {
		s := loadAlgoSlow(t)
		ifidsLocal := sbalgo.IFTuple{
			InIfid: 0,
			EgIfid: 81,
		}
		p := setupParams(ifidsLocal, s.Topo.ISD_AS, 0, 10, sibra.PathTypeUp, 10, 3)
		bps := sibra.BwCls(10).Bps()

		checker := egDemChecker{
			prefix: "Initial ",
			ifids:  ifidsLocal,
			s:      s,
		}
		Convey("Initial in demand is correct for local", func() {
			checker.checkOwnEgDem(bps, p)
		})
		for i := 0; i < 10; i++ {
			p := setupParams(ifidsLocal, s.Topo.ISD_AS, uint32(i), 10, sibra.PathTypeUp, 10, 3)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
			checker.prefix = "Check " + strconv.Itoa(i) + " "
			checker.checkOwnEgDem(bps*sibra.Bps(i+1), p)
		}

		ifidsUp := sbalgo.IFTuple{
			InIfid: 16,
			EgIfid: 81,
		}
		up := s.Topo.ISD_AS
		up.A++
		p = setupParams(ifidsUp, up, 0, 10, sibra.PathTypeUp, 10, 3)
		exp := map[addr.IA]sibra.Bps{
			s.Topo.ISD_AS: 10 * bps,
		}
		checker = egDemChecker{
			prefix: "Up ",
			ifids:  ifidsUp,
			s:      s,
		}
		Convey("Initial in demand is correct for down", func() {
			checker.checkOwnEgDem(bps, p)
		})
		Convey("Initial in demand is correct for other sources", func() {
			checker.checkOthersEgDem(exp, p)
		})
		for i := 0; i < 10; i++ {
			p := setupParams(ifidsUp, up, uint32(i), 10, sibra.PathTypeUp, 10, 3)
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
			checker.prefix = "Check " + strconv.Itoa(i) + " "
			checker.checkOwnEgDem(bps*sibra.Bps(i+1), p)
			checker.checkOthersEgDem(exp, p)
		}

	})
}

type egDemChecker struct {
	s      *AlgoSlow
	prefix string
	ifids  sbalgo.IFTuple
}

func (c *egDemChecker) checkOwnEgDem(exp sibra.Bps, p sbalgo.AdmParams) {
	for eg := range c.s.Infos {
		str := fmt.Sprintf(c.prefix+"own EgDem(%d-src%s)", eg, p.Src)
		if eg != c.ifids.EgIfid {
			SoMsg(str, c.s.egDem(p.Src, eg, p), ShouldEqual, 0)
		} else {
			SoMsg(str, c.s.egDem(p.Src, eg, p), ShouldEqual, exp)
		}
	}
}

func (c *egDemChecker) checkOthersEgDem(exp map[addr.IA]sibra.Bps, p sbalgo.AdmParams) {
	eg := c.ifids.EgIfid
	for other, expDem := range exp {
		str := fmt.Sprintf(c.prefix+"others EgDem(%d-src%s)", eg, other)
		SoMsg(str, c.s.egDem(other, eg, p), ShouldEqual, expDem)
	}
}

func TestAlgoSlow_tubeRatio(t *testing.T) {
	Convey("Tube ratio is calculated correctly", t, func() {
		s := loadAlgoSlow(t)
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
			ratio := s.tubeRatio(ifidsUp, p)
			thisDemand := float64(sibra.Bps(i+1) * bps)
			SoMsg("Ratio", ratio, ShouldEqual, (thisDemand)/(thisDemand+otherDemand))
			_, err := s.AdmitSteady(p)
			xtest.FailOnErr(t, err)
		}
	})
}

func loadAlgoSlow(t *testing.T) *AlgoSlow {
	topo, err := topology.LoadFromFile(filepath.Join("testdata", topology.CfgName))
	xtest.FailOnErr(t, err)
	mat, err := state.MatrixFromFile(filepath.Join("testdata", state.MatrixName))
	xtest.FailOnErr(t, err)
	s, err := NewSibraSlow(topo, mat)
	xtest.FailOnErr(t, err)
	return s
}
