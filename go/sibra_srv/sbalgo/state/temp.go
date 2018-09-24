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
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
)

const (
	TempResvExpiry = 1 * time.Second
	TempGCInterval = 100 * time.Millisecond
)

type TempTable struct {
	cache *cache.Cache
}

func NewTempTable() *TempTable {
	c := cache.New(TempResvExpiry, TempGCInterval)
	c.OnEvicted(tempOnEvict)
	return &TempTable{cache: c}
}

func (m *TempTable) Get(id sibra.ID, idx sibra.Index) *TempTableEntry {
	entry, ok := m.cache.Get(m.toKey(id, idx))
	if !ok {
		return nil
	}
	return entry.(*TempTableEntry)
}

func (m *TempTable) Set(id sibra.ID, idx sibra.Index, e *TempTableEntry, exp time.Duration) {
	m.cache.Set(m.toKey(id, idx), e, exp)
}

func (m *TempTable) Delete(id sibra.ID, idx sibra.Index) {
	// XXX(roosd): this is racy. However, SteadyResvEntry is protected
	// against deletion if the state is not temporary
	entry, ok := m.cache.Get(m.toKey(id, idx))
	if !ok {
		return
	}
	tmpEntry := entry.(*TempTableEntry)
	tmpEntry.Lock()
	tmpEntry.deleted = true
	tmpEntry.Unlock()
	m.cache.Delete(m.toKey(id, idx))
}

func (m *TempTable) toKey(id sibra.ID, idx sibra.Index) string {
	return fmt.Sprintf("id: %s idx: %d", id, idx)
}

func tempOnEvict(key string, value interface{}) {
	entry := value.(*TempTableEntry)
	var err error
	entry.RLock()
	if !entry.deleted {
		// TODO(roosd): remove
		//log.Debug("Evicting expired pending reservation", "key", key)
		err = entry.ResvMapEntry.CollectTempIndex(entry.Idx)
	}
	entry.RUnlock()
	if err != nil {
		log.Error("[tempOnEvict] Unable to collect temp index", "key", key, "err", err)
	}
}

type TempTableEntry struct {
	sync.RWMutex
	ResvMapEntry *SteadyResvEntry
	Idx          sibra.Index
	deleted      bool
}
