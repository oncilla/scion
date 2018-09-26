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
	"strings"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/assert"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

type demState struct {
	src          addr.IA
	ifids        sbalgo.IFTuple
	currInDem    sibra.Bps
	currEgDem    sibra.Bps
	currSrcDem   sibra.Bps
	currSrcAlloc sibra.Bps
	nextInDem    sibra.Bps
	nextEgDem    sibra.Bps
	nextSrcDem   sibra.Bps
	nextSrcAlloc sibra.Bps
}

// calcState holds the calculation state for the ideal computation.
type calcState struct {
	sbalgo.AdmParams
	tempInDem  sibra.Bps
	tempEgDem  sibra.Bps
	tempSrcDem sibra.Bps
}

type SrcMap map[addr.IA]SrcEntry

func (s SrcMap) String() string {
	var infoList []string
	for k, v := range s {
		infoList = append(infoList, k.String())
		infoList = append(infoList, fmt.Sprintf("  SrcAlloc: %s", v.SrcAlloc))
		infoList = append(infoList, fmt.Sprintf("  SrcDem:   %s", v.SrcDemand))
		infoList = append(infoList, fmt.Sprintf("  InDem:    %s", v.InDemand))
		infoList = append(infoList, fmt.Sprintf("  EgDem:    %s", v.EgDemand))
	}
	return strings.Join(infoList, "\n")
}

// SrcEntry stores the out demand and the source demand values for a given
// source. It is used to improve the performance of the SIBRA algorithm.
type SrcEntry struct {
	// InDemand holds the sum of all max bandwidth classes for all
	// reservations going through an ingress interface for this source.
	InDemand map[common.IFIDType]sibra.Bps
	// EgDemand holds the sum of all max bandwidth classes for all
	// reservations going through an egress interface for this source.
	EgDemand map[common.IFIDType]sibra.Bps
	// SrcDemand holds the sum of all max bandwidth classes for all
	// reservations going through the interface tuple for this source.
	SrcDemand map[sbalgo.IFTuple]sibra.Bps
	// SrcAlloc holds the sum of the allocated bandwidth for all
	// reservations going through the interface tuple for this source.
	SrcAlloc map[sbalgo.IFTuple]sibra.Bps
	// ephemMap holds all ephemeral reservations for a given source.
	ephemMap *state.EphemResvMap
}

var _ sbalgo.Algo = (*AlgoFast)(nil)
var _ sbalgo.SteadyAdm = (*AlgoFast)(nil)
var _ sbalgo.EphemAdm = (*AlgoFast)(nil)

// AlgoFast implements the SIBRA algorithm. It caches the calculation of the
// sums in the algorithm in order to improve performance. In stead of naively
// calculating the sum. The SIBRA algorithm computes the differences caused
// by the reservation request and uses the cached sums to avoid many table
// look ups.
type AlgoFast struct {
	*algoBase
	// TransitDemand keeps track of the transit demand between interfaces.
	TransitDemand map[sbalgo.IFTuple]float64
	// TransitAlloc keeps track of the transit allocation between interfaces.
	TransitAlloc map[sbalgo.IFTuple]float64
	// SrcMap maps source AS to the source entry which keeps track of the
	// source demand and out demand values of a given source.
	SourceMap SrcMap
}

func NewSibraFast(topo *topology.Topo, matrix state.Matrix) (*AlgoFast, error) {
	s, err := state.NewSibraState(topo, matrix)
	if err != nil {
		return nil, err
	}
	a := &AlgoFast{
		algoBase: &algoBase{
			ephemAdm: &ephemAdm{s},
		},
		TransitDemand: make(map[sbalgo.IFTuple]float64),
		TransitAlloc:  make(map[sbalgo.IFTuple]float64),
		SourceMap:     make(SrcMap, 10000),
	}
	return a, nil
}

// AdmitSteady does executes the SIBRA algorithm. The provided interfaces must be
// in the reservation direction.
func (s *AlgoFast) AdmitSteady(params sbalgo.AdmParams) (sbalgo.SteadyRes, error) {
	return admitSteady(s, params, s.Topo)
}

// Available calculates the available bandwidth on the out interface. It assumes
// the caller holds the lock over the receiver.
func (s *AlgoFast) Available(ifids sbalgo.IFTuple, id sibra.ID) sibra.Bps {
	var alloc sibra.Bps
	entry, ok := s.SteadyMap.Get(id)
	if ok && entry.Ifids == ifids {
		alloc = entry.Allocated
	}
	free := minBps(s.Infos[ifids.EgIfid].Egress.Free(), s.Infos[ifids.InIfid].Ingress.Free())
	return sibra.Bps(float64(free+alloc) * s.Delta)
}

// Ideal calculates the ideal bandwidth the reservation should get. It assumes
// the caller holds the lock over the receiver.
func (s *AlgoFast) Ideal(p sbalgo.AdmParams) sibra.Bps {
	c := &calcState{
		AdmParams: p,
	}
	s.setCalcState(c)
	outCap := float64(s.Infos[c.Ifids.EgIfid].Egress.Total)
	tubeRatio := s.tubeRatio(c)
	linkRatio := s.linkRatio(c)
	return sibra.Bps(outCap * tubeRatio * linkRatio)
}

func (s *AlgoFast) setCalcState(c *calcState) {
	var currSrcDem sibra.Bps
	inCap := s.Infos[c.Ifids.InIfid].Ingress.Total
	egCap := s.Infos[c.Ifids.EgIfid].Egress.Total
	c.tempSrcDem = minBps(minBps(c.Req.MaxBw.Bps(), inCap), egCap)
	// If src does not exist, this returns 0
	currInDem := s.SourceMap[c.Src].InDemand[c.Ifids.InIfid]
	currEgDem := s.SourceMap[c.Src].EgDemand[c.Ifids.EgIfid]
	// Set source demand for the ifid pair.
	if entry, ok := s.SourceMap[c.Src]; ok {
		currSrcDem = entry.SrcDemand[c.Ifids]
		c.tempSrcDem += currSrcDem
		// Avoid double counting the reservation's demand
		if steady, ok := s.SteadyMap.Get(c.Extn.GetCurrID()); ok && steady.Ifids == c.Ifids {
			c.tempSrcDem -= minBps(steady.LastMax, minBps(inCap, egCap))
		}
	}
	// Set temporary ingress/egress demand.
	c.tempEgDem = currEgDem + c.tempSrcDem - currSrcDem
	c.tempInDem = currInDem + c.tempSrcDem - currSrcDem
}

func (s *AlgoFast) tubeRatio(c *calcState) float64 {
	ifids := c.Ifids
	transDem := s.capTempTransDem(ifids, c)
	sum := transDem
	for inIfid := range s.InIfids[c.Ifids.EgIfid] {
		ifids.InIfid = inIfid
		// Avoid recomputing trans demand for ifids.
		if c.Ifids != ifids {
			sum += s.capTempTransDem(ifids, c)
		}
	}
	if sum <= 0 {
		if assert.On {
			assert.Must(false, "Sum of transit demand is 0",
				"in", c.Ifids.InIfid, "eg", c.Ifids.EgIfid)
		}
		// XXX(roosd): Panic to find possible bugs. Should be removed
		// before deploying to production and return 0 instead.
		log.Crit("Sum of transit demand less or equal 0",
			"dem", sum, "ingress", c.Ifids.InIfid, "egress", c.Ifids.EgIfid)
		panic("Sum of transit demand less or equal 0")
		return 0
	}
	return transDem / sum
}

func (s *AlgoFast) capTempTransDem(ifids sbalgo.IFTuple, c *calcState) float64 {
	var currAdjSrcDem, tempAdjSrcDem float64
	inCap := s.Infos[ifids.InIfid].Ingress.Total
	egCap := s.Infos[ifids.EgIfid].Egress.Total
	_, ok := s.SourceMap[c.Src]
	switch {
	case ok:
		currAdjSrcDem, tempAdjSrcDem = s.tempAdjSrcDemExisting(inCap, egCap, ifids, c)
	case ifids == c.Ifids:
		// For a new source, only the transit demand with the same interfaces
		// is affected by the new reservation.
		currAdjSrcDem, tempAdjSrcDem = s.tempAdjSrcDemNew(inCap, egCap, c)
	default:
		// Source is not involved. Thus nothing changes for this value.
		return math.Min(float64(egCap), s.TransitDemand[ifids])
	}
	capCurr := math.Min(currAdjSrcDem, float64(inCap))
	capTemp := math.Min(tempAdjSrcDem, float64(inCap))
	return math.Min(float64(egCap), s.TransitDemand[ifids]+capTemp-capCurr)
}

// tempAdjSrcDemNew calculates the demand for the source of the requested reservation
// if the source does not already have a reservation in the reservation map.
func (s *AlgoFast) tempAdjSrcDemNew(inCap, egCap sibra.Bps, c *calcState) (float64, float64) {
	return 0, s.calcAdjSrcDem(inCap, egCap, c.tempInDem, c.tempEgDem, c.tempSrcDem)
}

// tempAdjSrcDemExisting calculates the demand for the source of the requested reservation
// if the source already has a reservation in the reservation map.
func (s *AlgoFast) tempAdjSrcDemExisting(inCap, egCap sibra.Bps, ifids sbalgo.IFTuple,
	c *calcState) (float64, float64) {
	currSrcDem := s.SourceMap[c.Src].SrcDemand[ifids]
	currInDem := s.SourceMap[c.Src].InDemand[ifids.InIfid]
	currEgDem := s.SourceMap[c.Src].EgDemand[ifids.EgIfid]
	var temp float64
	switch {
	case ifids == c.Ifids:
		temp = s.calcAdjSrcDem(inCap, egCap, c.tempInDem, c.tempEgDem, c.tempSrcDem)
	case ifids.InIfid == c.Ifids.InIfid:
		temp = s.calcAdjSrcDem(inCap, egCap, c.tempInDem, currEgDem, currSrcDem)
	case ifids.EgIfid == c.Ifids.EgIfid:
		temp = s.calcAdjSrcDem(inCap, egCap, currInDem, c.tempEgDem, currSrcDem)
	default:
		return float64(currSrcDem), float64(currSrcDem)
	}
	curr := s.calcAdjSrcDem(inCap, egCap, currInDem, currEgDem, currSrcDem)
	return curr, temp
}

func (s *AlgoFast) calcAdjSrcDem(capIn, capEg, inDem, egDem, srcDem sibra.Bps) float64 {
	return s.scalingFactor(capIn, capEg, inDem, egDem) * float64(srcDem)
}

func (s *AlgoFast) scalingFactor(capIn, capEg, inDem, egDem sibra.Bps) float64 {
	return math.Min(s.inScalFactr(capIn, inDem), s.egScalFactr(capEg, egDem))
}

func (s *AlgoFast) inScalFactr(capIn, inDem sibra.Bps) float64 {
	if inDem <= 0 {
		return 0
	}
	return float64(minBps(capIn, inDem)) / float64(inDem)
}

func (s *AlgoFast) egScalFactr(capEg, egDem sibra.Bps) float64 {
	if egDem <= 0 {
		return 0
	}
	return float64(minBps(capEg, egDem)) / float64(egDem)
}

func (s *AlgoFast) linkRatio(c *calcState) float64 {
	minMax := s.prevBw(c.AdmParams)
	if minMax <= 0 {
		return 0
	}
	prevBw := minMax.Bps()
	egCap := s.Infos[c.Ifids.EgIfid].Egress.Total
	currEgDem := s.SourceMap[c.Src].EgDemand[c.Ifids.EgIfid]
	// currSrcAlloc is 0 if the source has no reservation yet.
	currSrcAlloc := s.SourceMap[c.Src].SrcAlloc[c.Ifids]
	currScaledAlloc := s.egScalFactr(egCap, currEgDem) * float64(currSrcAlloc)
	tempScalFactr := s.egScalFactr(egCap, c.tempEgDem)
	tempScaledAlloc := tempScalFactr * float64(s.tempSrcAlloc(prevBw, currSrcAlloc, c))
	denom := s.TransitAlloc[c.Ifids] + (tempScaledAlloc) - (currScaledAlloc)
	if denom <= 0 {
		return 0
	}
	return (tempScalFactr * float64(prevBw)) / denom
}

func (s *AlgoFast) tempSrcAlloc(prevBw, srcAlloc sibra.Bps, c *calcState) sibra.Bps {
	var alloc sibra.Bps
	if steady, ok := s.SteadyMap.Get(c.Extn.GetCurrID()); ok && steady.Ifids == c.Ifids {
		alloc = steady.Allocated
	}
	return srcAlloc + prevBw - alloc
}

// AddSteadyResv adds a steady reservation given the parameters and the allocated
// bandwidth. It assumes that the caller holds the lock over the receiver.
func (s *AlgoFast) AddSteadyResv(p sbalgo.AdmParams, alloc sibra.BwCls) error {
	// Add index and reserve the required bandwidth.
	info := *p.Req.Info
	info.BwCls = alloc
	idx := &state.SteadyResvIdx{
		Info:  info,
		MinBW: p.Req.MinBw,
		MaxBW: p.Req.MaxBw,
	}

	stEntry, ok := s.SteadyMap.Get(p.Extn.GetCurrID())
	if !ok {
		if p.Req.Info.Index != 0 {
			return common.NewBasicError("Invalid initial index", nil,
				"expected", sibra.Index(0), "actual", p.Req.Info.Index)
		}
		// If EphemResvMap is nil, it is set during updateSourceMap again.
		stEntry = &state.SteadyResvEntry{
			Src:          p.Src,
			Id:           p.Extn.GetCurrID().Copy(),
			Ifids:        p.Ifids,
			SibraAlgo:    s,
			EphemResvMap: s.SourceMap[p.Src].ephemMap,
			Cache:        true,
		}
		// We do not have to worry about garbage collection of the entry
		// since we hold the lock over the steady map.
		if err := s.SteadyMap.Add(p.Extn.GetCurrID(), stEntry); err != nil {
			return err
		}
	}
	if err := stEntry.AddIdx(idx); err != nil {
		return err
	}
	if err := s.addIndex(stEntry, idx); err != nil {
		stEntry.DelIdx(idx.Info.Index)
		return err
	}
	// Add temp entry to clean up failed reservations.
	tmpEntry := &state.TempTableEntry{
		ResvMapEntry: stEntry,
		Idx:          p.Req.Info.Index,
	}
	s.TempTable.Set(p.Extn.GetCurrID(), p.Req.Info.Index, tmpEntry, info.RLC.Duration())
	return nil
}

// addIndex assumes both AlgoFast s and SteadyResvEnry e are locked by caller.
func (s *AlgoFast) addIndex(e *state.SteadyResvEntry, idx *state.SteadyResvIdx) error {
	// Update interface info
	diff := idx.Info.BwCls.Bps() - e.Allocated
	if diff > 0 {
		if err := s.allocIfInfo(e.Ifids, diff); err != nil {
			return err
		}
		e.Allocated += diff
	}
	// adjust source map
	nextMax := idx.MaxBW.Bps()
	dem := s.getDemState(e.Src, e.Ifids)
	s.setNextDemState(dem, diff, e.LastMax, nextMax)
	// update the steady reservation entry
	if ephemMap := s.updateSourceMap(dem); ephemMap != nil {
		e.EphemResvMap = ephemMap
	}
	if e.LastMax < nextMax {
		e.LastMax = nextMax
	}
	// If source demand did not change, there is nothing left to be done.
	if dem.currSrcDem == dem.nextSrcDem {
		return nil
	}
	switch {
	case dem.currSrcDem == dem.nextSrcDem && dem.currSrcAlloc == dem.nextSrcAlloc:
		// If the source demand and alloc did not change, there is nothing to do.
		return nil
	case dem.currSrcDem == dem.nextSrcDem:
		// If the source demand did not change, only the transit alloc map must be changed.
		s.updateTransitAllocMap(dem)
	default:
		// If the source demand changed, the egress demand is affected, which influences
		// both the transit alloc and transit demand.
		s.updateTransitAllocMap(dem)
		s.updateTransitMap(dem)
	}
	return nil
}

func (s *AlgoFast) setNextDemState(dem *demState, diff, lastMax, nextMax sibra.Bps) {
	min := minBps(s.Infos[dem.ifids.InIfid].Ingress.Total, s.Infos[dem.ifids.EgIfid].Egress.Total)
	capLastMax := minBps(min, lastMax)
	capNextMax := minBps(min, nextMax)
	if diff > 0 {
		dem.nextSrcAlloc += diff
	}
	dem.nextSrcDem = dem.currSrcDem + capNextMax - capLastMax
	dem.nextInDem = dem.currInDem + dem.nextSrcDem - dem.currSrcDem
	dem.nextEgDem = dem.currEgDem + dem.nextSrcDem - dem.currSrcDem
}

func (s *AlgoFast) updateSourceMap(dem *demState) *state.EphemResvMap {
	if _, ok := s.SourceMap[dem.src]; !ok {
		ephemMap := state.NewEpehmResvMap()
		s.SourceMap[dem.src] = SrcEntry{
			EgDemand: map[common.IFIDType]sibra.Bps{
				dem.ifids.EgIfid: dem.nextEgDem,
			},
			InDemand: map[common.IFIDType]sibra.Bps{
				dem.ifids.InIfid: dem.nextInDem,
			},
			SrcDemand: map[sbalgo.IFTuple]sibra.Bps{
				dem.ifids: dem.nextSrcDem,
			},
			SrcAlloc: map[sbalgo.IFTuple]sibra.Bps{
				dem.ifids: dem.nextSrcAlloc,
			},
			ephemMap: ephemMap,
		}
		return ephemMap
	}
	if dem.nextSrcDem != dem.currSrcDem {
		s.SourceMap[dem.src].SrcDemand[dem.ifids] = dem.nextSrcDem
		s.SourceMap[dem.src].InDemand[dem.ifids.InIfid] = dem.nextInDem
		s.SourceMap[dem.src].EgDemand[dem.ifids.EgIfid] = dem.nextEgDem
	}
	if dem.nextSrcAlloc != dem.currSrcAlloc {
		s.SourceMap[dem.src].SrcAlloc[dem.ifids] = dem.nextSrcAlloc
	}
	return nil
}

func (s *AlgoFast) allocIfInfo(ifids sbalgo.IFTuple, bps sibra.Bps) error {
	if err := s.Infos[ifids.InIfid].Ingress.Alloc(bps); err != nil {
		return err
	}
	if err := s.Infos[ifids.EgIfid].Egress.Alloc(bps); err != nil {
		errC := s.Infos[ifids.InIfid].Ingress.Dealloc(bps)
		if assert.On {
			assert.Must(errC == nil, "No error during info cleanup")
		}
		return err
	}
	if err := s.TransitCap[ifids].Alloc(bps); err != nil {
		errC := s.Infos[ifids.InIfid].Ingress.Dealloc(bps)
		if assert.On {
			assert.Must(errC == nil, "No error during info cleanup")
		}
		errC = s.Infos[ifids.EgIfid].Egress.Dealloc(bps)
		if assert.On {
			assert.Must(errC == nil, "No error during info cleanup")
		}
		return err
	}
	return nil
}

// CleanSteadyResv updates the state bases on the given values. This function
// is called from the steady map when a reservation index needs to be cleaned
// up. LastMax indicates the maximum bandwidth class of the reservation before
// clean up. NextMax indicates the maximum after cleanup. It assumes the caller
// holds the lock over the receiver.
func (s *AlgoFast) CleanSteadyResv(c sbalgo.CleanParams) {
	if c.Dealloc > 0 {
		s.cleanIfInfo(c.Ifids, c.Dealloc)
	}
	// adjust source map
	if _, ok := s.SourceMap[c.Src]; !ok {
		return
	}
	dem := s.getDemState(c.Src, c.Ifids)
	// If max did not change there is nothing do adjust
	if c.LastMax == c.CurrMax {
		return
	}
	minCap := minBps(s.Infos[c.Ifids.InIfid].Ingress.Total, s.Infos[c.Ifids.EgIfid].Egress.Total)
	dem.nextSrcDem = dem.currSrcDem + minBps(c.CurrMax, minCap) - minBps(c.LastMax, minCap)
	dem.nextInDem = dem.currInDem + dem.nextSrcDem - dem.currSrcDem
	dem.nextEgDem = dem.currEgDem + dem.nextSrcDem - dem.currSrcDem
	s.SourceMap[c.Src].SrcDemand[c.Ifids] = dem.nextSrcDem
	s.SourceMap[c.Src].InDemand[c.Ifids.InIfid] = dem.nextInDem
	s.SourceMap[c.Src].EgDemand[c.Ifids.EgIfid] = dem.nextEgDem
	// XXX(roosd): Panic to find possible bugs. Should be removed
	// before deploying to production.
	if dem.nextSrcDem < 0 {
		log.Crit("nextSrcDem negative", "nextSrcDem", dem.nextSrcDem)
		panic("nextSrcDem negative")
	}
	if dem.nextInDem < 0 {
		log.Crit("nextInDem negative", "nextInDem", dem.nextInDem)
		panic("nextInDem negative")
	}
	if dem.nextEgDem < 0 {
		log.Crit("nextEgDem negative", "nextEgDem", dem.nextEgDem)
		panic("nextEgDem negative")
	}
	s.updateTransitMap(dem)
	// This means there is no valid index anymore.
	if dem.nextSrcDem <= 0 {
		delete(s.SourceMap[c.Src].SrcDemand, c.Ifids)
	}
	if len(s.SourceMap[c.Src].SrcDemand) <= 0 {
		delete(s.SourceMap, c.Src)
	}
}

func (s *AlgoFast) cleanIfInfo(ifids sbalgo.IFTuple, bps sibra.Bps) {
	err := s.Infos[ifids.InIfid].Ingress.Dealloc(bps)
	if assert.On {
		assert.Must(err == nil, "Error during ingress info cleanup")
	}
	err = s.Infos[ifids.EgIfid].Egress.Dealloc(bps)
	if assert.On {
		assert.Must(err == nil, "Error during egress info cleanup")
	}
	err = s.TransitCap[ifids].Dealloc(bps)
	if assert.On {
		assert.Must(err == nil, "Error during transit cap cleanup")
	}
}

func (s *AlgoFast) getDemState(src addr.IA, ifids sbalgo.IFTuple) *demState {
	return &demState{
		src:          src,
		ifids:        ifids,
		currInDem:    s.SourceMap[src].InDemand[ifids.InIfid],
		currEgDem:    s.SourceMap[src].EgDemand[ifids.EgIfid],
		currSrcDem:   s.SourceMap[src].SrcDemand[ifids],
		currSrcAlloc: s.SourceMap[src].SrcAlloc[ifids],
		nextSrcDem:   s.SourceMap[src].SrcDemand[ifids],
		nextInDem:    s.SourceMap[src].InDemand[ifids.InIfid],
		nextEgDem:    s.SourceMap[src].EgDemand[ifids.EgIfid],
		nextSrcAlloc: s.SourceMap[src].SrcAlloc[ifids],
	}
}

func (s *AlgoFast) updateTransitMap(dem *demState) {
	// Adjust affected transit maps
	for ifids := range s.SourceMap[dem.src].SrcDemand {
		// Source demands which do not share a common interface with the updated
		// reservation are not affected by the update.
		if ifids.InIfid == dem.ifids.InIfid || ifids.EgIfid == dem.ifids.EgIfid {
			s.TransitDemand[ifids] += s.nextTransDemDiff(ifids, dem)
		}
	}
}

func (s *AlgoFast) nextTransDemDiff(ifids sbalgo.IFTuple, dem *demState) float64 {
	var currAdjSrcDem, nextAdjSrcDem float64
	inCap := s.Infos[ifids.InIfid].Ingress.Total
	egCap := s.Infos[ifids.EgIfid].Egress.Total
	currAdjSrcDem, nextAdjSrcDem = s.nextAdjSrcDem(inCap, egCap, ifids, dem)
	capCurr := math.Min(currAdjSrcDem, float64(inCap))
	capTemp := math.Min(nextAdjSrcDem, float64(inCap))
	return capTemp - capCurr
}

func (s *AlgoFast) nextAdjSrcDem(inCap, egCap sibra.Bps, ifids sbalgo.IFTuple,
	dem *demState) (float64, float64) {
	currSrcDem := s.SourceMap[dem.src].SrcDemand[ifids]
	var next float64
	var curr float64
	switch {
	case ifids == dem.ifids:
		curr = s.calcAdjSrcDem(inCap, egCap, dem.currInDem, dem.currEgDem, dem.currSrcDem)
		next = s.calcAdjSrcDem(inCap, egCap, dem.nextInDem, dem.nextEgDem, dem.nextSrcDem)
	case ifids.InIfid == dem.ifids.InIfid:
		currEgDem := s.SourceMap[dem.src].EgDemand[ifids.EgIfid]
		// The ingress demand has been adjusted
		curr = s.calcAdjSrcDem(inCap, egCap, dem.currInDem, currEgDem, currSrcDem)
		next = s.calcAdjSrcDem(inCap, egCap, dem.nextInDem, currEgDem, currSrcDem)
	case ifids.EgIfid == dem.ifids.EgIfid:
		currInDem := s.SourceMap[dem.src].InDemand[ifids.InIfid]
		// The egress demand has been adjusted
		curr = s.calcAdjSrcDem(inCap, egCap, currInDem, dem.currEgDem, currSrcDem)
		next = s.calcAdjSrcDem(inCap, egCap, currInDem, dem.nextEgDem, currSrcDem)
	default:
		return float64(currSrcDem), float64(currSrcDem)
	}
	return curr, next
}

func (s *AlgoFast) updateTransitAllocMap(dem *demState) {
	// Adjust affected transit maps
	for ifids := range s.SourceMap[dem.src].SrcAlloc {
		// Source allocs which do not share a common egress interface with
		// the updated reservation are not affected by the update.
		if ifids.EgIfid == dem.ifids.EgIfid {
			s.TransitAlloc[ifids] += s.nextTransAllocDiff(ifids, dem)
		}
	}
}

func (s *AlgoFast) nextTransAllocDiff(ifids sbalgo.IFTuple, dem *demState) float64 {
	var currScaledSrcAlloc, nextScaledSrcAlloc float64
	egCap := s.Infos[ifids.EgIfid].Egress.Total
	currScaledSrcAlloc, nextScaledSrcAlloc = s.nextScaledSrcAlloc(egCap, ifids, dem)
	return nextScaledSrcAlloc - currScaledSrcAlloc
}

func (s *AlgoFast) nextScaledSrcAlloc(egCap sibra.Bps, ifids sbalgo.IFTuple,
	dem *demState) (float64, float64) {
	currSrcAlloc := s.SourceMap[dem.src].SrcAlloc[ifids]
	var next float64
	var curr float64
	switch {
	case ifids == dem.ifids:
		curr = s.egScalFactr(egCap, dem.currEgDem) * float64(dem.currSrcAlloc)
		next = s.egScalFactr(egCap, dem.nextEgDem) * float64(dem.nextSrcAlloc)
	case ifids.EgIfid == dem.ifids.EgIfid:
		curr = s.egScalFactr(egCap, dem.currEgDem) * float64(currSrcAlloc)
		next = s.egScalFactr(egCap, dem.nextEgDem) * float64(currSrcAlloc)
	default:
		return float64(currSrcAlloc), float64(currSrcAlloc)
	}
	return curr, next
}

func (s *AlgoFast) String() string {
	return fmt.Sprintf("TransitDem %v\nSrcMap %v\nState %s", s.TransitDemand, s.SourceMap,
		s.ephemAdm.SibraState)
}
