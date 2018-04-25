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

	"runtime"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

const (
	SteadyResvGCInterval = sbresv.TickInterval * time.Second
	keysSize             = 100
)

// TODO(roosd): Allocation is done naively. The SIBRA service should take
// pending reservations into account in order to prevent reserving to much bandwidth!
//
// Key insight: The dealloc ring only needs to be altered if the next dealloc entry is at
// now + 1 and we are near to the tick change.
//
// Algo to find the dealloc ticks:
//
// list = [(bwCls, exp_time)...] for all indexes
// sort(list) by bwCls and exp_time both descending
// d := 0
// deallocs = [(diff, exp_time)]
// for i:=1; i < len(list)-1; i++ {
// 		if list[d].exp_time < list[i].exp_time && list[d].bw > list[i].bw{
//			deallocs = append(deallocs, (list[d].bw-list[i], list[__d__].exp_time))
// 		}
// }
type SteadyResvMap struct {
	*steadyResvMap
}

// TODO(roosd) add cleanup routine

type steadyResvMap struct {
	resvs      map[string]*SteadyResvEntry
	mu         sync.RWMutex
	onEvicted  func(*SteadyResvEntry)
	onDelete   func(*SteadyResvEntry)
	janitor    *steadyJanitor
	sibraState *SibraState
}

func NewSteadyResvMap(state *SibraState) *SteadyResvMap {
	m := &steadyResvMap{
		resvs:      make(map[string]*SteadyResvEntry),
		sibraState: state,
	}
	M := &SteadyResvMap{m}
	runSteadyJanitor(m, SteadyResvGCInterval)
	runtime.SetFinalizer(M, stopSteadyJanitor)
	return M
}

// Add assumes caller holds lock for sibraState.
func (m *steadyResvMap) Add(id sbresv.ID, entry *SteadyResvEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.get(id)
	if ok {
		return common.NewBasicError("Steady reservation already exists", nil, "id", id)
	}
	m.resvs[string([]byte(id))] = entry
	return nil
}

// Get assumes caller holds lock for sibraState.
func (m *steadyResvMap) Get(id sbresv.ID) (*SteadyResvEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.get(id)
}

func (m *steadyResvMap) get(id sbresv.ID) (*SteadyResvEntry, bool) {
	r, ok := m.resvs[string([]byte(id))]
	if !ok {
		return nil, false
	}
	if r.Expired(time.Now()) {
		return nil, false
	}
	return r, true
}

func (m *steadyResvMap) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.resvs)
}

func (m *steadyResvMap) NonVoidIdxs() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var c int
	now := time.Now()
	for _, v := range m.resvs {
		c += v.NonVoidIdxs(now)
	}
	return c
}

// Delete assumes caller holds lock for sibraState.
func (m *steadyResvMap) Delete(id sbresv.ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.resvs[string([]byte(id))]
	if ok {
		delete(m.resvs, string([]byte(id)))
		if m.onDelete != nil {
			m.onDelete(r)
		}
	}
}

func (m *steadyResvMap) CollectBandwidth() {
	keys := make([]*SteadyResvEntry, 0, keysSize)
	// Find reservations which need to be cleaned up.
	m.mu.RLock()
	now := time.Now()
	for _, v := range m.resvs {
		if v.NeedsCleanUp(now) {
			keys = append(keys, v)
		}
	}
	m.mu.RUnlock()
	// Clean the found reservations.
	m.sibraState.Lock()
	defer m.sibraState.Unlock()
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, v := range keys {
		v.CleanUp(now)
	}
}

func (m *steadyResvMap) DeleteExpired() {
	keys := make([]string, 0, keysSize)
	values := make([]*SteadyResvEntry, 0, keysSize)
	// Find the expired and evicted reservations.
	m.mu.RLock()
	now := time.Now()
	for k, v := range m.resvs {
		if v.Expired(now) {
			keys = append(keys, k)
			values = append(values, v)
		}
	}
	m.mu.RUnlock()
	// Delete expired reservations
	m.sibraState.Lock()
	m.mu.Lock()
	now = time.Now()
	for i, k := range keys {
		values[i].CleanUp(now)
		delete(m.resvs, k)
		if m.onDelete != nil {
			m.onDelete(values[i])
		}
	}
	m.mu.Unlock()
	m.sibraState.Unlock()
	if m.onEvicted != nil {
		for _, v := range values {
			m.onEvicted(v)
		}
	}
}

func (m *steadyResvMap) OnEvicted(f func(*SteadyResvEntry)) {
	m.mu.Lock()
	m.onEvicted = f
	m.mu.Unlock()
}

func (m *steadyResvMap) OnDelete(f func(*SteadyResvEntry)) {
	m.mu.Lock()
	m.onDelete = f
	m.mu.Unlock()
}

type steadyJanitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *steadyJanitor) Run(m *steadyResvMap) {
	j.stop = make(chan bool)
	time.Sleep((sbresv.CurrentTick() + 1).Time().Sub(time.Now()))
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			m.CollectBandwidth()
			m.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopSteadyJanitor(m *SteadyResvMap) {
	m.janitor.stop <- true
}

func runSteadyJanitor(m *steadyResvMap, interval time.Duration) {
	j := &steadyJanitor{
		Interval: interval,
	}
	m.janitor = j
	go j.Run(m)
}
