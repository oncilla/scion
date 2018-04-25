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

package sbalgo

import (
	"path/filepath"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	libsibra "github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/sibra"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

func BenchmarkSibraSlowE100_10(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      100,
		ReqPerExistingSrc: 10,
	}
	benchmarkSibraSlow(p, b)
}

func BenchmarkSibraFastE100_10(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      100,
		ReqPerExistingSrc: 10,
	}
	benchmarkSibraFast(p, b)
}

func BenchmarkSibraSlowE1000_100(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      1000,
		ReqPerExistingSrc: 100,
	}
	benchmarkSibraSlow(p, b)
}

func BenchmarkSibraSlowE10000_10(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      10000,
		ReqPerExistingSrc: 10,
	}
	benchmarkSibraSlow(p, b)
}

func BenchmarkSibraSlowE100000_5(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      100000,
		ReqPerExistingSrc: 5,
	}
	benchmarkSibraSlow(p, b)
}

func BenchmarkSibraFastE1000_100(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      1000,
		ReqPerExistingSrc: 100,
	}
	benchmarkSibraFast(p, b)
}

func BenchmarkSibraFastE10000_10(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      10000,
		ReqPerExistingSrc: 10,
	}
	benchmarkSibraFast(p, b)
}

func BenchmarkSibraFastE100000_5(b *testing.B) {
	p := benchParams{
		ExistingSrcs:      10000,
		ReqPerExistingSrc: 5,
	}
	benchmarkSibraFast(p, b)
}

type benchParams struct {
	ExistingSrcs      int
	ReqPerExistingSrc int
}

var result sibra.Algo

func benchmarkSibraFast(p benchParams, b *testing.B) {
	b.StopTimer()
	topo, err := topology.LoadFromFile(filepath.Join("testdata", topology.CfgName))
	if err != nil {
		panic(err)
	}
	mat, err := state.MatrixFromFile(filepath.Join("testdata", state.MatrixName))
	if err != nil {
		panic(err)
	}
	s, err := NewSibraFast(topo, mat)
	if err != nil {
		panic(err)
	}
	benchmarkAdmitSteady(s, p, b)
}

func benchmarkSibraSlow(p benchParams, b *testing.B) {
	b.StopTimer()
	topo, err := topology.LoadFromFile(filepath.Join("testdata", topology.CfgName))
	if err != nil {
		panic(err)
	}
	mat, err := state.MatrixFromFile(filepath.Join("testdata", state.MatrixName))
	if err != nil {
		panic(err)
	}
	s, err := NewSibraSlow(topo, mat)
	if err != nil {
		panic(err)
	}
	benchmarkAdmitSteady(s, p, b)
}

func benchmarkAdmitSteady(s sibra.Algo, p benchParams, b *testing.B) {
	src := addr.IA{I: 1}
	ifidsLocal := sibra.IFTuple{
		InIfid: 0,
		EgIfid: 81,
	}
	for r := 0; r < p.ReqPerExistingSrc; r++ {
		for src.A = 0; src.A < addr.AS(p.ExistingSrcs); src.A++ {
			p := setupParams(ifidsLocal, src, uint32(r), 10, sbresv.PathTypeUp, 255, 3)
			s.AdmitSteady(p)
		}
	}
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		src.A = addr.AS(n % p.ExistingSrcs)
		r := n + p.ReqPerExistingSrc
		p := setupParams(ifidsLocal, src, uint32(r), 10, sbresv.PathTypeUp, 255, 3)
		s.AdmitSteady(p)
	}
}

func setupParams(ifids sibra.IFTuple, src addr.IA, suf uint32, maxBw sbresv.BwCls,
	pt sbresv.PathType, rtt sbresv.RttCls, numHop uint8) sibra.AdmParams {

	info := &sbresv.Info{
		BwCls:    maxBw,
		PathType: pt,
		ExpTick:  sbresv.CurrentTick().Add(sbresv.MaxSteadyTicks),
		RttCls:   rtt,
	}
	req := sbreq.NewSteadyReq(sbreq.RSteadySetup, info, 0, maxBw, numHop)
	id := sbresv.NewSteadyID(src.A, suf)
	extn, err := libsibra.NewSteadySetup(req, id)
	if err != nil {
		panic(err)
	}
	p := sibra.AdmParams{
		Ifids: ifids,
		Extn:  extn,
		Src:   src,
		Req:   req,
	}
	return p
}
