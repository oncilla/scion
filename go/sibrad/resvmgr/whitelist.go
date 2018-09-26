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
	"flag"
	"fmt"
	"net"
	"regexp"
	"sync"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
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

var _ flag.Value = (*Net)(nil)

var netRegexp = regexp.MustCompile(
	`^(?P<ia>\d+-[\d:A-Fa-f]+),\[(?P<net>[^\]]+)\]$`)

type Net struct {
	IA  addr.IA
	Net *net.IPNet
}

func (a *Net) String() string {
	if a == nil {
		return "<nil>"
	}
	s := fmt.Sprintf("%s,[%v]", a.IA, a.Net)
	return s
}

// UnmarshalText implements encoding.TextUnmarshaler
func (a *Net) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*a = Net{}
	}
	other, err := NetFromString(string(text))
	if err != nil {
		return err
	}
	*a = *other
	return nil
}

// NetFromString converts an sub network string of format isd-as,[CIDR]
// (e.g., 1-ff00:0:300,[192.168.0.0/16]) to a Net.
func NetFromString(s string) (*Net, error) {
	parts, err := parseNet(s)
	if err != nil {
		return nil, err
	}
	ia, err := addr.IAFromString(parts["ia"])
	if err != nil {
		return nil, common.NewBasicError("Invalid IA string", err, "ia", ia)
	}
	_, ipnet, err := net.ParseCIDR(parts["net"])
	if err != nil {
		return nil, err
	}
	return &Net{IA: ia, Net: ipnet}, nil
}

func parseNet(s string) (map[string]string, error) {
	result := make(map[string]string)
	match := netRegexp.FindStringSubmatch(s)
	if len(match) == 0 {
		return nil, common.NewBasicError("Invalid address: regex match failed", nil, "addr", s)
	}
	for i, name := range netRegexp.SubexpNames() {
		if i == 0 {
			continue
		}
		result[name] = match[i]
	}
	return result, nil
}

// This method implements flag.Value interface
func (a *Net) Set(s string) error {
	other, err := NetFromString(s)
	if err != nil {
		return err
	}
	a.IA, a.Net = other.IA, other.Net
	return nil
}
