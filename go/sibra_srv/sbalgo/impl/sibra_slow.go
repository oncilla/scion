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
	"math"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

var _ sbalgo.Algo = (*AlgoSlow)(nil)
var _ sbalgo.SteadyAdm = (*AlgoSlow)(nil)
var _ sbalgo.EphemAdm = (*AlgoSlow)(nil)

// AlgoSlow implements the SIBRA algorithm.
type AlgoSlow struct {
	*algoBase
	//
	SrcToIds map[addr.IA]map[string]struct{}
}

func NewSibraSlow(topo *topology.Topo, matrix state.Matrix) (*AlgoSlow, error) {
	s, err := state.NewSibraState(topo, matrix)
	if err != nil {
		return nil, err
	}
	a := &AlgoSlow{
		algoBase: &algoBase{
			ephemAdm: &ephemAdm{s},
		},
		SrcToIds: make(map[addr.IA]map[string]struct{}),
	}
	return a, nil
}

// AdmitSteady does executes the SIBRA algorithm. The provided interfaces must be
// in the reservation direction.
func (s *AlgoSlow) AdmitSteady(params sbalgo.AdmParams) (sbalgo.SteadyRes, error) {
	return admitSteady(s, params, s.Topo)
}

// Available calculates the available bandwidth on the out interface. It assumes
// the caller holds the lock over the receiver.
func (s *AlgoSlow) Available(ifids sbalgo.IFTuple, id sibra.ID) sibra.Bps {
	// Entry for eg is guaranteed to exist
	in := s.Infos[ifids.InIfid].Ingress.Total - s.reqResIn(ifids.InIfid, id)
	eg := s.Infos[ifids.EgIfid].Egress.Total - s.reqResEg(ifids.EgIfid, id)
	return sibra.Bps(float64(minBps(in, eg)) * s.Delta)
}

func (s *AlgoSlow) reqResIn(in common.IFIDType, id sibra.ID) sibra.Bps {
	var sum sibra.Bps
	for _, ids := range s.SrcToIds {
		for i := range ids {
			entry, ok := s.SteadyMap.Get(sibra.ID(i))
			if !ok || entry.Ifids.InIfid != in || entry.Id.Eq(id) {
				continue
			}
			sum += entry.AllocBw()
		}
	}
	return sum
}

func (s *AlgoSlow) reqResEg(eg common.IFIDType, id sibra.ID) sibra.Bps {
	var sum sibra.Bps
	for _, ids := range s.SrcToIds {
		for i := range ids {
			entry, ok := s.SteadyMap.Get(sibra.ID(i))
			if !ok || entry.Ifids.EgIfid != eg || entry.Id.Eq(id) {
				continue
			}
			sum += entry.AllocBw()
		}
	}
	return sum
}

// Ideal calculates the ideal bandwidth the reservation should get. It assumes
// the caller holds the lock over the receiver.
func (s *AlgoSlow) Ideal(p sbalgo.AdmParams) sibra.Bps {
	outCap := float64(s.Infos[p.Ifids.EgIfid].Egress.Total)
	tubeRatio := s.tubeRatio(p.Ifids, p)
	linkRatio := s.linkRatio(p)
	return sibra.Bps(outCap * tubeRatio * linkRatio)
}

func (s *AlgoSlow) tubeRatio(ifids sbalgo.IFTuple, p sbalgo.AdmParams) float64 {
	transDem := s.transDem(ifids, p)
	sum := transDem
	tup := ifids
	for in := range s.Infos {
		tup.InIfid = in
		// Avoid recomputing trans demand for ifids.
		if tup != ifids {
			capIn := s.Infos[ifids.InIfid].Ingress.Total
			sum += math.Min(float64(capIn), s.transDem(tup, p))

		}
	}
	if sum > 0 {
		return transDem / sum
	}
	panic(fmt.Sprintf("Sum of transit demand between (%d,%d) is 0", ifids.InIfid, ifids.EgIfid))
}

func (s *AlgoSlow) transDem(ifids sbalgo.IFTuple, p sbalgo.AdmParams) float64 {
	var sum float64
	for src := range s.SrcToIds {
		sum += s.adjSrcDem(src, ifids, p)
	}
	if _, ok := s.SrcToIds[p.Src]; !ok && ifids == p.Ifids {
		sum += s.adjSrcDem(p.Src, ifids, p)
	}
	return sum
}

func (s *AlgoSlow) adjSrcDem(src addr.IA, ifids sbalgo.IFTuple, p sbalgo.AdmParams) float64 {
	return s.scalingFactor(src, ifids, p) * float64(s.srcDem(src, ifids, p))
}

func (s *AlgoSlow) scalingFactor(src addr.IA, ifids sbalgo.IFTuple, p sbalgo.AdmParams) float64 {
	return math.Min(s.inScalFactr(src, ifids.InIfid, p), s.egScalFactr(src, ifids.EgIfid, p))
}

func (s *AlgoSlow) inScalFactr(src addr.IA, in common.IFIDType, p sbalgo.AdmParams) float64 {
	capIn := s.Infos[in].Ingress.Total
	inDem := s.inDem(src, in, p)
	if inDem <= 0 {
		return 0
	}
	return float64(minBps(capIn, inDem)) / float64(inDem)
}

func (s *AlgoSlow) egScalFactr(src addr.IA, eg common.IFIDType, p sbalgo.AdmParams) float64 {
	capEg := s.Infos[eg].Egress.Total
	egDem := s.egDem(src, eg, p)
	if egDem <= 0 {
		return 0
	}
	return float64(minBps(capEg, egDem)) / float64(egDem)
}

func (s *AlgoSlow) inDem(src addr.IA, in common.IFIDType, p sbalgo.AdmParams) sibra.Bps {
	var sum sibra.Bps
	ifids := sbalgo.IFTuple{
		InIfid: in,
	}
	for eg := range s.Infos {
		ifids.EgIfid = eg
		sum += s.srcDem(src, ifids, p)
	}
	return sum
}

func (s *AlgoSlow) egDem(src addr.IA, eg common.IFIDType, p sbalgo.AdmParams) sibra.Bps {
	var sum sibra.Bps
	ifids := sbalgo.IFTuple{
		EgIfid: eg,
	}
	for in := range s.Infos {
		ifids.InIfid = in
		sum += s.srcDem(src, ifids, p)
	}
	return sum
}

func (s *AlgoSlow) srcDem(src addr.IA, ifids sbalgo.IFTuple, p sbalgo.AdmParams) sibra.Bps {
	var sum sibra.Bps
	for id := range s.SrcToIds[src] {
		sum += s.reqDem(sibra.ID(id), ifids, p)
	}
	// Consider the reservation if it does not exist yet
	// in the source to id mapping
	if ifids == p.Ifids && src == p.Src {
		if _, ok := s.SrcToIds[src][string([]byte(p.Extn.ReqID))]; !ok {
			sum += s.reqDem(p.Extn.ReqID, ifids, p)
		}
	}
	return sum
}

func (s *AlgoSlow) reqDem(id sibra.ID, ifids sbalgo.IFTuple, p sbalgo.AdmParams) sibra.Bps {
	capIn := s.Infos[ifids.InIfid].Ingress.Total
	capEg := s.Infos[ifids.EgIfid].Egress.Total
	// In case the calculation is done for the id which is currently
	// requested, return MaxBW if the interfaces are matching.
	if id.Eq(p.Extn.ReqID) {
		if ifids == p.Ifids {
			return minBps(minBps(capIn, capEg), p.Req.MaxBw.Bps())
		}
		return 0
	}
	entry, ok := s.SteadyMap.Get(id)
	if !ok || entry.Ifids != ifids {
		return 0
	}
	return minBps(minBps(capIn, capEg), entry.MaxBw())
}

func (s *AlgoSlow) linkRatio(p sbalgo.AdmParams) float64 {
	minMax := s.prevBw(p)
	if minMax <= 0 {
		return 0
	}
	nom := s.egScalFactr(p.Src, p.Ifids.EgIfid, p) * float64(minMax.Bps())
	sum := nom
	for src, ids := range s.SrcToIds {
		var srcAlloc sibra.Bps
		for i := range ids {
			entry, ok := s.SteadyMap.Get(sibra.ID(i))
			if ok && entry.Ifids == p.Ifids && !entry.Id.Eq(p.Extn.ReqID) {
				srcAlloc += entry.AllocBw()
			}
		}
		sum += s.egScalFactr(src, p.Ifids.EgIfid, p) * float64(srcAlloc)
	}
	if sum <= 0 {
		return 0
	}
	return nom / sum
}

// AddSteadyResv adds a steady reservation given the parameters and the allocated
// bandwidth. It assumes that the caller holds the lock over the receiver.
func (s *AlgoSlow) AddSteadyResv(p sbalgo.AdmParams, alloc sibra.BwCls) error {
	// Add index and reserve the required bandwidth.
	info := *p.Req.Info
	info.BwCls = alloc
	idx := &state.SteadyResvIdx{
		Info:  info,
		MinBW: p.Req.MinBw,
		MaxBW: p.Req.MaxBw,
	}

	stEntry, ok := s.SteadyMap.Get(p.Extn.ReqID)
	if !ok {
		if p.Req.Info.Index != 0 {
			return common.NewBasicError("Invalid initial index", nil,
				"expected", sibra.Index(0), "actual", p.Req.Info.Index)
		}
		stEntry = &state.SteadyResvEntry{
			Src:          p.Src,
			Id:           p.Extn.ReqID.Copy(),
			Ifids:        p.Ifids,
			SibraAlgo:    s,
			EphemResvMap: state.NewEpehmResvMap(),
		}
		// We do not have to worry about garbage collection of the entry
		// since we hold the lock over the steady map.
		if err := s.SteadyMap.Add(p.Extn.ReqID, stEntry); err != nil {
			return err
		}
	}
	if err := stEntry.AddIdx(idx); err != nil {
		return err
	}
	// Adjust src to id mapping
	s.addIndex(stEntry, idx)
	// Add temp entry to clean up failed reservations.
	tmpEntry := &state.TempTableEntry{
		ResvMapEntry: stEntry,
		Idx:          p.Req.Info.Index,
	}
	s.TempTable.Set(p.Extn.ReqID, p.Req.Info.Index, tmpEntry, info.RttCls.Duration())
	return nil
}

// addIndex assumes both AlgoSlow s and SteadyResvEnry e are locked by caller.
func (s *AlgoSlow) addIndex(e *state.SteadyResvEntry, idx *state.SteadyResvIdx) {
	if _, ok := s.SrcToIds[e.Src]; !ok {
		s.SrcToIds[e.Src] = map[string]struct{}{
			string([]byte(e.Id)): {},
		}
	} else {
		s.SrcToIds[e.Src][string([]byte(e.Id))] = struct{}{}
	}
}

// CleanSteadyResv updates the state bases on the given values. This function
// is called from the steady map when a reservation index needs to be cleaned
// up.
func (s *AlgoSlow) CleanSteadyResv(c sbalgo.CleanParams) {

	// adjust source map
	if _, ok := s.SrcToIds[c.Src]; !ok {
		return
	}
	if !c.Remove {
		return
	}
	delete(s.SrcToIds[c.Src], string([]byte(c.Id)))
	if len(s.SrcToIds[c.Src]) == 0 {
		delete(s.SrcToIds, c.Src)
	}
}

func (s *AlgoSlow) String() string {
	return fmt.Sprintf("SrcToId %s\nState %s", s.SrcToIds, s.SibraState)
}
