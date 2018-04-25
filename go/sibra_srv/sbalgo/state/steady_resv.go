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

package state

import (
	"sync"

	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/sibra"
)

const (
	IndexExists      = "Index already exists"
	IndexNonExistent = "Index does not exist"
	InfoNotMatching  = "Info does not match"
	InvalidState     = "Invalid state"
)

// TODO(roosd): Make sure deadlocks cannot occure -> need for locking hierarchy
//

type SteadyResvEntry struct {
	sync.RWMutex
	// Src is the reservation source AS.
	Src addr.IA
	// Id is the reservation ID.
	Id sbresv.ID
	// Ifids TODO(roosd)
	Ifids sibra.IFTuple
	// Indexes TODO(roosd)
	Indexes [sbresv.NumIndexes]*SteadyResvIdx
	// SibraAlgo TODO(roosd)
	SibraAlgo sibra.Algo
	// TODO(roosd)
	BaseResv *SteadyResvEntry
	// TODO(roosd)
	Telescopes []*SteadyResvEntry
	// Ephemeral Bandwidth
	EphemeralBW *BWProvider
	// TODO(roosd)
	EphemResvMap *EphemResvMap
	// TODO(roosd)
	Allocated sbresv.Bps
	// TODO(roosd)
	LastMax sbresv.Bps
	// TODO(roosd)
	ActiveIndex sbresv.Index
	// Cache indicates if the Allocated and LastMax are cached.
	Cache bool
}

func (e *SteadyResvEntry) NeedsCleanUp(now time.Time) bool {
	e.Lock()
	defer e.Unlock()
	return e.needsCleanUp(now)
}

func (e *SteadyResvEntry) needsCleanUp(now time.Time) bool {
	return e.LastMax != e.maxBw(now) || e.allocBw(now) != e.Allocated || e.expired(now)
}

// CleanUp assumes that the call has the lock for SibraAlgo.
func (e *SteadyResvEntry) CleanUp(now time.Time) {
	e.Lock()
	defer e.Unlock()
	e.cleanUp(now)
}

// cleanUp assumes that the caller has the lock for SibraAlgo and the SteadyResvEntry.
func (e *SteadyResvEntry) cleanUp(now time.Time) {
	if e.BaseResv != nil {
		// If this is a telescoped extension, bandwidth has not been actually reserved
		return
	}
	var lastMax, allocDiff sbresv.Bps
	// Cache values in order for the fast SIBRA algorithm to work.
	if e.Cache {
		lastMax = e.LastMax
		e.LastMax = e.maxBw(now)
		// calculate required allocation
		newAlloc := e.allocBw(now)
		allocDiff = e.Allocated - newAlloc
		e.Allocated = newAlloc
		if allocDiff < 0 {
			panic("Allocated less than required")
		}
	}
	// Avoid unnecessary cleanup
	if (!e.Cache || lastMax == e.LastMax && allocDiff == 0) && !e.expired(now) {
		return
	}
	c := sibra.CleanParams{
		Ifids:   e.Ifids,
		Id:      e.Id,
		Src:     e.Src,
		LastMax: lastMax,
		CurrMax: e.LastMax,
		Dealloc: allocDiff,
		Remove:  e.expired(now),
	}
	e.SibraAlgo.CleanSteadyResv(c)
}

func (e *SteadyResvEntry) Expired(now time.Time) bool {
	e.RLock()
	defer e.RUnlock()
	return e.expired(now)
}

func (e *SteadyResvEntry) expired(now time.Time) bool {
	idx := e.Indexes[e.ActiveIndex]
	if idx == nil {
		return true
	}
	return !idx.Active(now)
}

// AddIdx adds an index to the reservation and updates the sibra state accordingly.
// The caller must have a lock on the sibra state.
func (e *SteadyResvEntry) AddIdx(idx *SteadyResvIdx) error {
	e.Lock()
	defer e.Unlock()
	sub := e.Indexes[idx.Info.Index]
	if sub != nil && sub.Active(time.Now()) {
		return common.NewBasicError(IndexExists, nil, "id", e.Id, "idx", idx)
	}
	e.Indexes[idx.Info.Index] = idx
	return nil
}

func (e *SteadyResvEntry) DelIdx(idx sbresv.Index) {
	e.Lock()
	defer e.Unlock()
	e.Indexes[idx] = nil
}

func (e *SteadyResvEntry) PromoteToSOFCreated(info *sbresv.Info) error {
	e.Lock()
	defer e.Unlock()
	sub := e.Indexes[info.Index]
	if sub == nil {
		return common.NewBasicError(IndexNonExistent, nil, "idx", info.Index)
	}
	if sub.State != sbresv.StateTemp {
		return common.NewBasicError(InvalidState, nil,
			"id", e.Id, "idx", info.Index, "state", sub.State)
	}
	if sub.SOFCreated {
		return common.NewBasicError("SOF already created", nil, "idx", info.Index)
	}
	if sub.Info.BwCls < info.BwCls {
		return common.NewBasicError("Invalid actual BW class", nil, "idx", info.Index,
			"max", sub.Info.BwCls, "actual", info.BwCls)
	}
	if sub.Info.ExpTick != info.ExpTick || sub.Info.RttCls != info.RttCls ||
		sub.Info.PathType != info.PathType || info.FailHop != 0 {
		return common.NewBasicError("Invalid info", nil, "expected", sub.Info, "actual", info)
	}
	sub.SOFCreated = true
	sub.Info.BwCls = info.BwCls
	e.cleanUp(time.Now())
	return nil
}

func (e *SteadyResvEntry) PromoteToPending(idx sbresv.Index) error {
	e.Lock()
	defer e.Unlock()
	sub := e.Indexes[idx]
	if sub == nil {
		return common.NewBasicError(IndexNonExistent, nil, "idx", idx)
	}
	if sub.State == sbresv.StatePending {
		// FIXME(roosd): Limit the time this is possible
		return nil
	}
	if sub.State != sbresv.StateTemp {
		return common.NewBasicError(InvalidState, nil, "idx", idx, "state", sub.State)
	}
	if !sub.SOFCreated {
		return common.NewBasicError("SOF not created yet", nil, "idx", idx)
	}
	sub.State = sbresv.StatePending
	return nil
}

func (e *SteadyResvEntry) PromoteToActive(idx sbresv.Index, info *sbresv.Info) error {
	e.Lock()
	defer e.Unlock()
	sub := e.Indexes[idx]
	if sub == nil {
		return common.NewBasicError(IndexNonExistent, nil, "idx", idx)
	}
	if !sub.Info.Eq(info) {
		return common.NewBasicError(InfoNotMatching, nil, "expected", sub.Info, "actual", info)
	}
	if sub.State == sbresv.StateActive {
		return nil
	}
	if sub.State != sbresv.StatePending {
		return common.NewBasicError(InvalidState, nil,
			"expected", sbresv.StatePending, "actual", sub.State)
	}
	if e.EphemeralBW != nil {
		// Adjust ephemeral bandwidth if possible.
		if err := e.EphemeralBW.SetTotal(uint64(sub.Info.BwCls.Bps())); err != nil {
			return err
		}
	} else {
		e.setEphemeralBWProvider(sub.Info.BwCls)
	}
	// Remove invalidated indexes.
	for i := e.ActiveIndex; i != idx; i = (i + 1) % sbresv.NumIndexes {
		e.Indexes[i] = nil
	}
	e.ActiveIndex = idx
	sub.State = sbresv.StateActive
	e.cleanUp(time.Now())
	return nil
}

func (e *SteadyResvEntry) setEphemeralBWProvider(cls sbresv.BwCls) {
	e.EphemeralBW = &BWProvider{
		Total: uint64(cls.Bps()),
		deallocRing: deallocRing{
			currTick: sbresv.CurrentTick(),
			freeRing: make([]uint64, sbresv.MaxEphemTicks*2),
		},
	}
}

// TODO(roosd): promote void -> need to take care of ephemeral BW provider

func (e *SteadyResvEntry) CollectTempIndex(idx sbresv.Index) error {
	e.SibraAlgo.Lock()
	defer e.SibraAlgo.Unlock()
	e.Lock()
	defer e.Unlock()
	sub := e.Indexes[idx]
	if sub == nil {
		return common.NewBasicError(IndexNonExistent, nil, "idx", idx)
	}
	if sub.State != sbresv.StateTemp {
		return common.NewBasicError(InvalidState, nil, "idx", idx, "state", sub.State)
	}
	e.Indexes[idx] = nil
	e.cleanUp(time.Now())
	return nil
}

func (e *SteadyResvEntry) MaxBw() sbresv.Bps {
	e.Lock()
	defer e.Unlock()
	return e.maxBw(time.Now())
}

func (e *SteadyResvEntry) maxBw(now time.Time) sbresv.Bps {
	var max sbresv.BwCls
	for _, v := range e.Indexes {
		if v != nil && v.Active(now) && v.MaxBW > max {
			max = v.MaxBW
		}
	}
	return max.Bps()
}

func (e *SteadyResvEntry) AllocBw() sbresv.Bps {
	e.Lock()
	defer e.Unlock()
	return e.allocBw(time.Now())
}

func (e *SteadyResvEntry) allocBw(now time.Time) sbresv.Bps {
	var max sbresv.BwCls
	for _, v := range e.Indexes {
		if v != nil && v.Active(now) && v.Info.BwCls > max {
			max = v.Info.BwCls
		}
	}
	return max.Bps()
}

func (e *SteadyResvEntry) NonVoidIdxs(now time.Time) int {
	e.Lock()
	defer e.Unlock()
	var c int
	for _, v := range e.Indexes {
		if v != nil && v.Active(now) {
			c++
		}
	}
	return c
}

type SteadyResvIdx struct {
	// TODO(roosd): comments
	Info       sbresv.Info
	MinBW      sbresv.BwCls
	MaxBW      sbresv.BwCls
	State      sbresv.State
	SOFCreated bool
}

func (i *SteadyResvIdx) Active(t time.Time) bool {
	return (i.State != sbresv.StateVoid) && (t.Before(i.Info.ExpTick.Time()))
}
