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

package conf

import (
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

const (
	LocalResvExpiry = sbresv.MaxSteadyTicks * sbresv.TickDuration
	LocalGCInterval = LocalResvExpiry
)

type LocalResvs struct {
	cache *cache.Cache
}

func NewLocalResvs() *LocalResvs {
	c := cache.New(LocalResvExpiry, LocalGCInterval)
	return &LocalResvs{cache: c}
}

func (m *LocalResvs) Get(id sbresv.ID, idx sbresv.Index) *LocalResvEntry {
	entry, ok := m.cache.Get(m.toKey(id, idx))
	if !ok {
		return nil
	}
	return entry.(*LocalResvEntry)
}

func (m *LocalResvs) GetAll(id sbresv.ID) []*LocalResvEntry {
	res := make([]*LocalResvEntry, 0)
	for idx := sbresv.Index(0); idx < sbresv.NumIndexes; idx++ {
		e := m.Get(id, idx)
		if e != nil {
			res = append(res, e)
		}
	}
	return res
}

func (m *LocalResvs) Set(id sbresv.ID, idx sbresv.Index, e *LocalResvEntry, exp time.Duration) {
	m.cache.Set(m.toKey(id, idx), e, exp)
}

func (m *LocalResvs) Delete(id sbresv.ID, idx sbresv.Index) {
	m.cache.Delete(m.toKey(id, idx))
}

func (m *LocalResvs) Items() map[string]cache.Item {
	return m.cache.Items()
}

func (m *LocalResvs) toKey(id sbresv.ID, idx sbresv.Index) string {
	return fmt.Sprintf("id: %s idx: %d", id, idx)
}

type LocalResvEntry struct {
	Id       sbresv.ID
	State    sbresv.State
	Block    *sbresv.Block
	Creation time.Time
}
