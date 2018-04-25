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
	EphemResvGCInterval = sbresv.TickInterval * time.Second
)

type EphemResvMap struct {
	*ephemResvMap
}

// TODO(roosd) add cleanup routine

type ephemResvMap struct {
	resvs     map[string]*EphemResvEntry
	mu        sync.RWMutex
	onEvicted func(*EphemResvEntry)
	onDelete  func(*EphemResvEntry)
	janitor   *ephemJanitor
}

func NewEpehmResvMap() *EphemResvMap {
	m := &ephemResvMap{
		resvs: make(map[string]*EphemResvEntry),
	}
	M := &EphemResvMap{m}
	runEphemJanitor(m, EphemResvGCInterval)
	runtime.SetFinalizer(M, stopEphemJanitor)
	return M
}

func (m *ephemResvMap) Add(id sbresv.ID, entry *EphemResvEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.get(id)
	if ok {
		return common.NewBasicError("Ephemeral reservation already exists", nil, "id", id)
	}
	m.resvs[string([]byte(id))] = entry
	return nil
}

func (m *ephemResvMap) Get(id sbresv.ID) (*EphemResvEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.get(id)
}

func (m *ephemResvMap) get(id sbresv.ID) (*EphemResvEntry, bool) {
	r, ok := m.resvs[string([]byte(id))]
	if !ok {
		return nil, false
	}
	if r.Expired(time.Now()) {
		return nil, false
	}
	return r, true
}

func (m *ephemResvMap) Delete(id sbresv.ID) {
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

func (m *ephemResvMap) DeleteExpired() {
	m.mu.Lock()
	var evicted []*EphemResvEntry
	now := time.Now()
	for k, v := range m.resvs {
		if v.Expired(now) {
			delete(m.resvs, k)
			if m.onDelete != nil {
				m.onDelete(v)
			}
			evicted = append(evicted, v)
		}
	}
	m.mu.Unlock()
	if m.onEvicted != nil {
		for _, v := range evicted {
			m.onEvicted(v)
		}
	}
}

func (m *ephemResvMap) OnEvicted(f func(*EphemResvEntry)) {
	m.mu.Lock()
	m.onEvicted = f
	m.mu.Unlock()
}

func (m *ephemResvMap) OnDelete(f func(*EphemResvEntry)) {
	m.mu.Lock()
	m.onDelete = f
	m.mu.Unlock()
}

type ephemJanitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *ephemJanitor) Run(m *ephemResvMap) {
	j.stop = make(chan bool)
	time.Sleep((sbresv.CurrentTick() + 1).Time().Sub(time.Now()))
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			m.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopEphemJanitor(m *EphemResvMap) {
	m.janitor.stop <- true
}

func runEphemJanitor(m *ephemResvMap, interval time.Duration) {
	j := &ephemJanitor{
		Interval: interval,
	}
	m.janitor = j
	go j.Run(m)
}
