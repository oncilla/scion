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
	"path/filepath"
	"testing"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbcreate"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
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

var result sbalgo.Algo

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

func benchmarkAdmitSteady(s sbalgo.Algo, p benchParams, b *testing.B) {
	src := addr.IA{I: 1}
	ifidsLocal := sbalgo.IFTuple{
		InIfid: 0,
		EgIfid: 81,
	}
	for r := 0; r < p.ReqPerExistingSrc; r++ {
		for src.A = 0; src.A < addr.AS(p.ExistingSrcs); src.A++ {
			p := setupParams(ifidsLocal, src, uint32(r), 10, sibra.PathTypeUp, 255, 3)
			s.AdmitSteady(p)
		}
	}
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		src.A = addr.AS(n % p.ExistingSrcs)
		r := n + p.ReqPerExistingSrc
		p := setupParams(ifidsLocal, src, uint32(r), 10, sibra.PathTypeUp, 255, 3)
		s.AdmitSteady(p)
	}
}

func setupParams(ifids sbalgo.IFTuple, src addr.IA, suf uint32, maxBw sibra.BwCls,
	pt sibra.PathType, rtt sibra.RttCls, numHop uint8) sbalgo.AdmParams {

	info := &sbresv.Info{
		BwCls:    maxBw,
		PathType: pt,
		ExpTick:  sibra.CurrentTick().Add(sibra.MaxSteadyTicks),
		RttCls:   rtt,
	}
	req := sbreq.NewSteadyReq(sbreq.RSteadySetup, info, 0, maxBw, numHop)
	id := sibra.NewSteadyID(src.A, suf)
	extn, err := sbcreate.NewSteadySetup(req, id)
	if err != nil {
		panic(err)
	}
	p := sbalgo.AdmParams{
		Ifids:    ifids,
		Extn:     extn,
		Src:      src,
		Req:      req,
		Accepted: true,
	}
	return p
}
