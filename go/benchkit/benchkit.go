package main

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/aybabtme/benchkit"
	"golang.org/x/crypto/pbkdf2"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	libsibra "github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/lib/util"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/sibra"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

func main() {
	par := BenchFastSetup
	res := benchmark(par)
	for i, ts := range res.Each {
		fmt.Println("----------------")
		fmt.Printf("srcs %d ts %s \n", par.runs[i].Srcs, TimeStep(ts))
		fmt.Println("----------------")
	}
	err := saveBench(par, res)
	if err != nil {
		panic(err)
	}
}

type bench struct {
	kit  benchkit.BenchKit
	res  *benchkit.TimeResult
	topo *topology.Topo
	mat  state.Matrix
}

func benchmark(p benchParams) *benchkit.TimeResult {
	b := benchPrepare(p)
	b.kit.Starting()
	each := b.kit.Each()
	for i := range p.runs {
		runtime.GC()
		s, err := p.algo.Algo(b.topo, b.mat)
		if err != nil {
			panic(err)
		}
		p.runs[i].Run(i, s, p.numOps, each)
		fmt.Printf("Run %d done\n", i)
		m := p.algo.State(s).SteadyMap
		fmt.Printf("Size %d Indexes %d\n", m.Size(), m.NonVoidIdxs())
	}
	b.kit.Teardown()
	return b.res
}

func steadyTokenAuth(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	extn, ifids, pool := steadyTokenPrepare()
	extn.ActiveBlocks = []*sbresv.Block{
		extn.Request.(*sbreq.SteadySucc).Block,
	}
	tokenAuth(i, ops, each, extn.Base, ifids, pool)
}

func steadyTokenCreate(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	extn, ifids, pool := steadyTokenPrepare()
	tokenCreate(i, ops, each, extn.Base, ifids, pool)
}

func steadyTokenPrepare() (*sbextn.Steady, sibra.IFTuple, *sync.Pool) {
	sofGenKey := pbkdf2.Key(make([]byte, 16), []byte("Derive SOF Key"), 1000, 16, sha256.New)
	if _, err := util.InitMac(sofGenKey); err != nil {
		panic(fmt.Sprintf("steadyTokenPrepare %s", err))
	}
	pool := &sync.Pool{
		New: func() interface{} {
			mac, _ := util.InitMac(sofGenKey)
			return mac
		},
	}
	ifids := sibra.IFTuple{
		InIfid: 0,
		EgIfid: 81,
	}
	ids := []sbresv.ID{
		sbresv.NewSteadyID(1, 1),
	}
	info := &sbresv.Info{
		PathType: sbresv.PathTypeUp,
		ExpTick:  sbresv.CurrentTick().Add(sbresv.MaxEphemTicks),
		BwCls:    5,
	}
	req := &sbreq.SteadySucc{
		Block: sbresv.NewBlock(info, 2),
		Base: &sbreq.Base{
			Type:     sbreq.RSteadyRenewal,
			Accepted: true,
			Response: true,
		},
	}
	extn := &sbextn.Steady{
		Base: &sbextn.Base{
			IDs:      ids,
			ReqID:    ids[0],
			Request:  req,
			PathLens: []uint8{3, 0, 0},
		},
	}
	return extn, ifids, pool

}

func ephemTokenAuth(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	extn, ifids, pool := ephemTokenPrepare()
	extn.ActiveBlocks = []*sbresv.Block{
		extn.Request.(*sbreq.EphemReq).Block,
	}
	tokenAuth(i, ops, each, extn.Base, ifids, pool)
}

func ephemTokenCreate(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	extn, ifids, pool := ephemTokenPrepare()
	tokenCreate(i, ops, each, extn.Base, ifids, pool)
}

func tokenCreate(i int, ops int, each benchkit.BenchEach, base *sbextn.Base,
	ifids sibra.IFTuple, pool *sync.Pool) {
	for n := 0; n < ops; n++ {
		each.Before(i)
		err := issueSof(ifids, base, pool)
		each.After(i)
		if err != nil {
			panic(fmt.Sprintf("tokenCreate %s", err))
		}
	}
}

func tokenAuth(i int, ops int, each benchkit.BenchEach, base *sbextn.Base,
	ifids sibra.IFTuple, pool *sync.Pool) {

	err := issueSof(ifids, base, pool)
	if err != nil {
		panic(fmt.Sprintf("tokenAuth issueSof %s", err))
	}
	for n := 0; n < ops; n++ {
		mac := pool.Get().(hash.Hash)
		each.Before(i)
		err := base.VerifySOF(mac, time.Now())
		each.After(i)
		pool.Put(mac)
		if err != nil {
			panic(fmt.Sprintf("tokenAuth %s", err))
		}
	}
}

func ephemTokenPrepare() (*sbextn.Ephemeral, sibra.IFTuple, *sync.Pool) {
	sofGenKey := pbkdf2.Key(make([]byte, 16), []byte("Derive SOF Key"), 1000, 16, sha256.New)
	if _, err := util.InitMac(sofGenKey); err != nil {
		panic(fmt.Sprintf("ephemTokenPrepare %s", err))
	}
	pool := &sync.Pool{
		New: func() interface{} {
			mac, _ := util.InitMac(sofGenKey)
			return mac
		},
	}
	ifids := sibra.IFTuple{
		InIfid: 0,
		EgIfid: 81,
	}
	ids := []sbresv.ID{
		sbresv.NewEphemIDRand(1),
		sbresv.NewSteadyID(1, 1),
		sbresv.NewSteadyID(2, 1),
		sbresv.NewSteadyID(3, 1),
	}
	info := &sbresv.Info{
		PathType: sbresv.PathTypeEphemeral,
		ExpTick:  sbresv.CurrentTick().Add(sbresv.MaxEphemTicks),
		BwCls:    5,
	}
	req := sbreq.NewEphemReq(sbreq.REphmRenewal, nil, info, 2)
	extn := &sbextn.Ephemeral{
		Base: &sbextn.Base{
			IDs:      ids,
			ReqID:    ids[0],
			Request:  req,
			PathLens: []uint8{3, 3, 3},
		},
	}
	return extn, ifids, pool

}

func issueSof(ifids sibra.IFTuple, base *sbextn.Base, pool *sync.Pool) error {
	mac := pool.Get().(hash.Hash)
	err := base.SetSOF(mac, ifids.InIfid, ifids.EgIfid)
	pool.Put(mac)
	return err
}

func ephemRunSetup(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	sId := ephemPrepare(s, p)
	for n := 0; n < ops; n++ {
		extn := setupEphemParams(sId, 1, n, 1, 255, 3)
		each.Before(i)
		res, err := s.AdmitEphemSetup(extn)
		each.After(i)
		if res.AllocBw == 0 || err != nil {
			time.Sleep(10 * time.Millisecond)
			panic(fmt.Sprintf("ephemRunSetup %s %v", err, res))
		}
	}
}

func ephemRunRenew(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	sId := ephemPrepare(s, p)
	extn := setupEphemParams(sId, 1, 0, 1, 255, 3)
	res, err := s.AdmitEphemSetup(extn)
	if res.AllocBw == 0 || err != nil {
		panic(fmt.Sprintf("ephemRunRenew setup %s", err))
	}
	for n := 0; n < ops; n++ {
		extn := renewEphemParams(sId, 1, 0, 1, 255, 3, n+1)
		each.Before(i)
		res, err := s.AdmitEphemRenew(extn)
		each.After(i)
		if res.AllocBw == 0 || err != nil {
			panic(fmt.Sprintf("ephemRunRenew %s %v", err, res))
		}
	}
}

func steadyRunSetup(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	src, ifids := algoPrepare(s, p, 10)
	for n := 0; n < ops; n++ {
		src.A = addr.AS(n % p.Srcs)
		r := n + p.RpS
		p := setupSteadyParams(ifids, src, uint32(r), 10, sbresv.PathTypeUp, 255, 3)
		each.Before(i)
		res, err := s.AdmitSteady(p)
		each.After(i)
		if !res.Accepted || err != nil {
			panic(fmt.Sprintf("steadyRunSetup %s", err))
		}
		confirmSteady(s, p, res, ifids, 3)
	}
}

func steadyRunRenew(i int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach) {
	src, ifids := algoPrepare(s, p, 10)
	for n := 0; n < ops; n++ {
		src.A = addr.AS(n % p.Srcs)
		p := renewSteadyParams(ifids, src, 0, 10, sbresv.PathTypeUp, 255, 3, 1)
		each.Before(i)
		res, err := s.AdmitSteady(p)
		each.After(i)
		if !res.Accepted || err != nil {
			panic(fmt.Sprintf("steadyRunRenew %s %s %d", err, p.Extn.ReqID, p.Req.Info.Index))
		}
		confirmSteady(s, p, res, ifids, 3)
	}
}

func benchPrepare(p benchParams) bench {
	kit, res := benchkit.Time(len(p.runs), p.numOps)
	kit.Setup()
	topo, err := topology.LoadFromFile(filepath.Join("testdata", topology.CfgName))
	if err != nil {
		panic(err)
	}
	mat, err := state.MatrixFromFile(filepath.Join("testdata", state.MatrixName))
	if err != nil {
		panic(err)
	}
	mat[0][81] *= 1000
	return bench{
		kit:  kit,
		res:  res,
		topo: topo,
		mat:  mat,
	}
}

func ephemPrepare(s sibra.Algo, p runParams) sbresv.ID {
	_, ifids := algoPrepare(s, p, 30)
	id := sbresv.NewSteadyID(1, 0)
	var state *state.SibraState
	switch a := s.(type) {
	case *sbalgo.AlgoFast:
		state = a.SibraState
	case *sbalgo.AlgoSlow:
		state = a.SibraState
	}
	entry, _ := state.SteadyMap.Get(id)
	info := entry.Indexes[0].Info
	creq, err := sbreq.NewConfirmIndex(int(3), 0, sbresv.StateActive)
	if err != nil {
		panic(fmt.Sprintf("Unable to create confirm %s", err))
	}
	err = s.PromoteToActive(ifids, id, &info, creq)
	if err != nil {
		panic(fmt.Sprintf("Unable to promote pending %s", err))
	}
	return id
}

func algoPrepare(s sibra.Algo, p runParams, bwCls sbresv.BwCls) (addr.IA, sibra.IFTuple) {
	src := addr.IA{I: 1}
	ifids := sibra.IFTuple{
		InIfid: 0,
		EgIfid: 81,
	}
	for r := 0; r < p.RpS; r++ {
		for src.A = 0; src.A < addr.AS(p.Srcs); src.A++ {
			numHop := uint8(3)
			p := setupSteadyParams(ifids, src, uint32(r), bwCls, sbresv.PathTypeUp, 255, numHop)
			res, err := s.AdmitSteady(p)
			if !res.Accepted || err != nil {
				panic(fmt.Sprintf("Not accepted %s", err))
			}
			confirmSteady(s, p, res, ifids, int(numHop))
		}
	}
	return src, ifids
}

func confirmSteady(s sibra.Algo, p sibra.AdmParams, res sibra.SteadyRes, ifids sibra.IFTuple,
	numHop int) {

	p.Req.Info.BwCls = res.AllocBw
	err := s.PromoteToSOFCreated(ifids, p.Extn.ReqID, p.Req.Info)
	if err != nil {
		panic(fmt.Sprintf("Unable to promote %s", err))
	}
	creq, err := sbreq.NewConfirmIndex(numHop, p.Req.Info.Index, sbresv.StatePending)
	if err != nil {
		panic(fmt.Sprintf("Unable to create confirm %s", err))
	}
	err = s.PromoteToPending(ifids, p.Extn.ReqID, creq)
	if err != nil {
		panic(fmt.Sprintf("Unable to promote pending %s", err))
	}
}

func renewEphemParams(steadyId sbresv.ID, src addr.AS, suf int, bwCls sbresv.BwCls,
	rtt sbresv.RttCls, numHop int, idx int) *sbextn.Ephemeral {

	info := &sbresv.Info{
		BwCls:    bwCls,
		PathType: sbresv.PathTypeEphemeral,
		ExpTick:  sbresv.CurrentTick().Add(sbresv.MaxEphemTicks),
		RttCls:   rtt,
		Index:    sbresv.Index(idx) % sbresv.NumIndexes,
	}
	suffix := make([]byte, 10)
	common.Order.PutUint64(suffix[2:], uint64(suf))
	id := sbresv.NewEphemID(src, suffix)
	req := sbreq.NewEphemReq(sbreq.REphmRenewal, nil, info, numHop)
	return &sbextn.Ephemeral{
		Base: &sbextn.Base{
			ReqID: id,
			IDs: []sbresv.ID{
				id,
				steadyId,
			},
			Request: req,
		},
	}
}

func setupEphemParams(steadyId sbresv.ID, src addr.AS, suf int, bwCls sbresv.BwCls,
	rtt sbresv.RttCls, numHop int) *sbextn.Steady {

	info := &sbresv.Info{
		BwCls:    bwCls,
		PathType: sbresv.PathTypeEphemeral,
		ExpTick:  sbresv.CurrentTick().Add(sbresv.MaxEphemTicks),
		RttCls:   rtt,
	}
	suffix := make([]byte, 10)
	common.Order.PutUint64(suffix[2:], uint64(suf))
	id := sbresv.NewEphemID(src, suffix)
	req := sbreq.NewEphemReq(sbreq.REphmSetup, id, info, numHop)

	return &sbextn.Steady{
		Base: &sbextn.Base{
			ReqID: id,
			IDs: []sbresv.ID{
				steadyId,
			},
			Request: req,
		},
	}
}

func setupSteadyParams(ifids sibra.IFTuple, src addr.IA, suf uint32, maxBw sbresv.BwCls,
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

func renewSteadyParams(ifids sibra.IFTuple, src addr.IA, suf uint32, maxBw sbresv.BwCls,
	pt sbresv.PathType, rtt sbresv.RttCls, numHop uint8, idx int) sibra.AdmParams {

	info := &sbresv.Info{
		BwCls:    maxBw,
		PathType: pt,
		ExpTick:  sbresv.CurrentTick().Add(sbresv.MaxSteadyTicks),
		RttCls:   rtt,
		Index:    sbresv.Index(idx) % sbresv.NumIndexes,
	}
	req := sbreq.NewSteadyReq(sbreq.RSteadyRenewal, info, 0, maxBw, numHop)
	id := sbresv.NewSteadyID(src.A, suf)
	extn, err := libsibra.NewSteadySetup(req, id)
	if err != nil {
		panic(err)
	}
	extn.Setup = false
	p := sibra.AdmParams{
		Ifids: ifids,
		Extn:  extn,
		Src:   src,
		Req:   req,
	}
	extn.ActiveBlocks = append(extn.ActiveBlocks, &sbresv.Block{
		Info: &sbresv.Info{
			PathType: sbresv.PathTypeUp,
		},
	})
	return p
}
