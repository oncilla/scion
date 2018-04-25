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

package resvmgr

import (
	"sync"

	"net"

	"github.com/scionproto/scion/go/lib/addr"
)

type WhitListEntry bool

type whitelist struct {
	sync.RWMutex
	m map[addr.ISD]map[addr.AS]map[string]WhitListEntry
}

func (w *whitelist) insert(ia addr.IA, ipNet *net.IPNet) {
	w.Lock()
	defer w.Unlock()
	if w.m == nil {
		w.m = make(map[addr.ISD]map[addr.AS]map[string]WhitListEntry)
	}
	if w.m[ia.I] == nil {
		w.m[ia.I] = make(map[addr.AS]map[string]WhitListEntry)
	}
	if w.m[ia.I][ia.A] == nil {
		w.m[ia.I][ia.A] = make(map[string]WhitListEntry)
	}
	w.m[ia.I][ia.A][ipNet.String()] = true
}

func (w *whitelist) isAllowed(ia addr.IA, host net.IP) bool {
	w.RLock()
	defer w.RUnlock()
	if iterateMap(w.m[ia.I][ia.A], host) {
		return true
	}
	if iterateMap(w.m[ia.I][0], host) {
		return true
	}
	if iterateMap(w.m[0][0], host) {
		return true
	}
	return false
}

func iterateMap(m map[string]WhitListEntry, host net.IP) bool {
	for rawIPNet := range m {
		_, ipNet, err := net.ParseCIDR(rawIPNet)
		if err != nil {
			continue
		}
		if ipNet.Contains(host) {
			return true
		}
	}
	return false
}
