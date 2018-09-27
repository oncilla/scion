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
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
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

// ephemMeta is the metadata for ephemeral reservations.
type ephemMeta struct {
	remote addr.HostAddr
	// timestamp indicates the last time ephemMeta was updated.
	timestamp time.Time
	// minBwCls is the minimum desired bandwidth class.
	minBwCls sibra.BwCls
	// maxBwCls is the maximum desired bandwidth class.
	maxBwCls sibra.BwCls
	// state indicates the state of the reservation resolver.
	state state
	// lastFailCode indicates the last fail code.
	lastFailCode sbreq.FailCode
	// lastMaxBw indicates the last maximum bandwidth that the on path ASes
	// were willing to grant.
	lastMaxBw sibra.BwCls
}

// resvEntry is the metadata for the resolver.
type resvEntry struct {
	sync.Mutex
	// paths is a set of possible paths.
	paths *pathmgr.SyncPaths
	// pathKey indicates which path is preferred.
	pathKey spathmeta.PathKey
	// syncResv holds the SIBRA extension headers and is used to inject them
	// into SNET.
	syncResv *syncresv.Store
	// fixedPath indicates if only the path specified by pathKey must be used.
	fixedPath bool
	// ephemMeta holds the meta data for ephemeral reservations.
	ephemMeta *ephemMeta
}

// getPath returns the path with the specified pathKey.
func (s *resvEntry) getPath() *spathmeta.AppPath {
	path := s.paths.Load().APS.GetAppPath(s.pathKey)
	if path == nil || path.Key() != s.pathKey {
		return nil
	}
	return path
}

// getNewPath gets a new path and sets the pathKey to the new one.
func (s *resvEntry) getNewPath() *spathmeta.AppPath {
	path := s.paths.Load().APS.GetAppPath(s.pathKey)
	if path == nil || s.fixedPath && s.pathKey != path.Key() {
		return nil
	}
	s.pathKey = path.Key()
	return path
}

// store keeps track of all the reservation state.
type store struct {
	mutex sync.Mutex
	// segIdToSteady maps a segment ID to known steady IDs.
	segIdToSteady map[string]map[string]struct{}
	// steadyToMeta maps steady ID to steady meta data.
	steadyToMeta map[string]*steadyMeta
	// resvEntries keeps track of all reservation entries.
	resvEntries map[ResvKey]*resvEntry
	// id is a counter to provide unique ResvKeys.
	id ResvKey
}

func newStore() *store {
	return &store{
		segIdToSteady: make(map[string]map[string]struct{}),
		steadyToMeta:  make(map[string]*steadyMeta),
		resvEntries:   make(map[ResvKey]*resvEntry),
	}
}

func (c *store) getSteadyId(segId common.RawBytes) []sibra.ID {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	entry := c.segIdToSteady[string(segId)]
	list := make([]sibra.ID, len(entry))
	for k := range entry {
		list = append(list, sibra.ID(common.RawBytes(k)))
	}
	return list
}

func (c *store) addSteadyId(segId common.RawBytes, steadyId sibra.ID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if entry := c.segIdToSteady[string(segId)]; entry == nil {
		c.segIdToSteady[string(segId)] = make(map[string]struct{})
	}
	c.segIdToSteady[string(segId)][string(common.RawBytes(steadyId))] = struct{}{}
}

func (c *store) removeSteadyId(segId common.RawBytes, steadyId sibra.ID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	delete(c.segIdToSteady[string(segId)], string(common.RawBytes(steadyId)))
}

func (c *store) getSteadyMeta(steadyID sibra.ID) *steadyMeta {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.steadyToMeta[string(common.RawBytes(steadyID))]
}

func (c *store) addSteadyMeta(meta *steadyMeta) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.steadyToMeta[string(common.RawBytes(meta.Meta.Id))] = meta
}

func (c *store) removeSteadyMeta(steadyID sibra.ID) {
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
