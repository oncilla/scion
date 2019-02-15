// Copyright 2019 Anapaya Systems
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

package svcinstance

import (
	"sync"

	"github.com/scionproto/scion/go/lib/healthpool"
	"github.com/scionproto/scion/go/lib/topology"
)

type infoMap map[healthpool.InfoKey]*info

func (m infoMap) healthpoolInfoMap() healthpool.InfoMap {
	infos := make(healthpool.InfoMap, len(m))
	for k, v := range m {
		infos[k] = v
	}
	return infos
}

type Pool struct {
	mtx   sync.Mutex
	infos infoMap
	hpool healthpool.Pool
}

func NewPool(svcInfo topology.IDAddrMap, opts healthpool.PoolOptions) (*Pool, error) {
	p := &Pool{
		infos: createMap(svcInfo, nil),
	}
	var err error
	if p.hpool, err = healthpool.NewPool(p.infos.healthpoolInfoMap(), opts); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Pool) Update(svcInfo topology.IDAddrMap) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	infos := createMap(svcInfo, p.infos)
	if err := p.hpool.Update(infos.healthpoolInfoMap()); err != nil {
		return err
	}
	p.infos = infos
	return nil
}

func (p *Pool) Choose() (Info, error) {
	hinfo, err := p.hpool.Choose()
	if err != nil {
		return Info{}, err
	}
	return Info{info: hinfo.(*info)}, nil
}

func (p *Pool) Close() {
	p.hpool.Close()
}

func createMap(svcInfo topology.IDAddrMap, oldInfos infoMap) infoMap {
	infos := make(infoMap, len(svcInfo))
	for k, svc := range svcInfo {
		if oldInfo, ok := oldInfos[healthpool.InfoKey(k)]; ok {
			infos[healthpool.InfoKey(k)] = oldInfo
			oldInfo.update(svc.PublicAddr(svc.Overlay))
		} else {
			infos[healthpool.InfoKey(k)] = &info{
				Info: healthpool.NewInfo(),
				addr: svc.PublicAddr(svc.Overlay),
			}
		}
	}
	return infos
}
