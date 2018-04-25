// Copyright 2017 ETH Zurich
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

package resvmgr

import (
	"sync"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/pathmgr"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
	"github.com/scionproto/scion/go/sibrad/syncresv"
)

type ResvKey uint64

type state int

const (
	start state = iota
	ephemRequested
	cleanUp
	ephemExists
)

type steadyMeta struct {
	sync.Mutex
	Meta      *sibra_mgmt.BlockMeta
	ResvKeys  map[ResvKey]struct{}
	timestamp time.Time
}

type ephemMeta struct {
	remote addr.HostAddr
	// TODO(roosd)
	lastReq *sbreq.EphemReq
	// TODO(roosd)
	timestamp time.Time
	// TODO(roosd)
	minBwCls sbresv.BwCls
	// TODO(roosd)
	maxBwCls sbresv.BwCls
	// TODO(roosd)
	state state
	// TODO(roosd)
	lastFailCode sbreq.FailCode
	// TODO(roosd)
	lastMaxBW sbresv.BwCls
}

type resvEntry struct {
	sync.Mutex
	// TODO(roosd)
	paths *pathmgr.SyncPaths
	// TODO(roosd)
	pathKey spathmeta.PathKey
	// TODO(roosd)
	syncResv *syncresv.Store
	// TODO(roosd)
	fixedPath bool
	// TODO(roosd)
	ephemMeta *ephemMeta
}

func (s *resvEntry) getPath() *spathmeta.AppPath {
	path := s.paths.Load().APS.GetAppPath(s.pathKey)
	if path == nil || path.Key() != s.pathKey {
		return nil
	}
	return path
}

func (s *resvEntry) getNewPath() *spathmeta.AppPath {
	path := s.paths.Load().APS.GetAppPath(s.pathKey)
	if path == nil || s.fixedPath && s.pathKey != path.Key() {
		return nil
	}
	s.pathKey = path.Key()
	return path
}

type store struct {
	mutex sync.Mutex
	// TODO(roosd)
	segIdtoSteady map[string]map[string]struct{}
	// TODO(roosd)
	steadyToMeta map[string]*steadyMeta
	// TODO(roosd)
	resvEntries map[ResvKey]*resvEntry
	// TODO(roosd)
	ephemToEntries map[string]*resvEntry
	// TODO(roosd)
	id ResvKey
}

func newStore() *store {
	return &store{
		segIdtoSteady: make(map[string]map[string]struct{}),
		steadyToMeta:  make(map[string]*steadyMeta),
		resvEntries:   make(map[ResvKey]*resvEntry),
	}
}

/*func (c *store) update(key ResvKey, ephem common.Extension) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry, ok := c.resvEntries[key]
	if !ok {
		return common.NewBasicError("Unable to find StoreEntry", nil, "key", key)
	}
	entry.syncResv.UpdateEphem(ephem)
	entry.timestamp = time.Now()
	return nil
}*/

func (c *store) getSteadyId(segId common.RawBytes) []sbresv.ID {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry := c.segIdtoSteady[string(segId)]
	list := make([]sbresv.ID, len(entry))
	for k := range entry {
		list = append(list, sbresv.ID(common.RawBytes(k)))
	}
	return list
}

func (c *store) addSteadyId(segId common.RawBytes, steadyId sbresv.ID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if entry := c.segIdtoSteady[string(segId)]; entry == nil {
		c.segIdtoSteady[string(segId)] = make(map[string]struct{})
	}
	c.segIdtoSteady[string(segId)][string(common.RawBytes(steadyId))] = struct{}{}
}

func (c *store) removeSteadyId(segId common.RawBytes, steadyId sbresv.ID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.segIdtoSteady[string(segId)], string(common.RawBytes(steadyId)))
}

func (c *store) getSteadyMeta(steadyID sbresv.ID) *steadyMeta {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.steadyToMeta[string(common.RawBytes(steadyID))]
}

func (c *store) addSteadyMeta(meta *steadyMeta) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.steadyToMeta[string(common.RawBytes(meta.Meta.Id))] = meta
}

func (c *store) removeSteadyMeta(steadyID sbresv.ID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.steadyToMeta, string(common.RawBytes(steadyID)))
}

func (c *store) addResv(entry *resvEntry) (ResvKey, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := c.id
	c.id++
	if _, ok := c.resvEntries[key]; ok {
		return 0, common.NewBasicError("StoreEntry already exists", nil, "key", key)
	}
	c.resvEntries[key] = entry
	return key, nil
}

func (c *store) getResv(key ResvKey) *resvEntry {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.resvEntries[key]
}

func (c *store) removeResv(key ResvKey) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if _, ok := c.resvEntries[key]; !ok {
		return common.NewBasicError("Unable to remove missing reservation", nil, "key", key)
	}
	delete(c.resvEntries, key)
	return nil
}
