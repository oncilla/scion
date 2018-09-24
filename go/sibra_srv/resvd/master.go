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

package resvd

import (
	"bufio"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/sibra_srv/conf"
)

var _ conf.RepMaster = (*ResvMaster)(nil)

type ResvMaster struct {
	mu         sync.RWMutex
	ResvHandls map[string]*Reserver
	Notify     map[string]chan *conf.ExtPkt
}

func (r *ResvMaster) Run() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range conf.Get().Reservations {
		if reqstr, ok := r.ResvHandls[key]; !ok || reqstr.Closed() {
			reqstr = &Reserver{
				Logger:  log.New("resvKey", key),
				stop:    make(chan struct{}),
				resvKey: key,
			}
			r.ResvHandls[key] = reqstr
			go reqstr.Run()
		}
	}
}

func (r *ResvMaster) Handle(pkt *conf.ExtPkt) error {
	var idx sibra.Index
	switch e := pkt.Steady.Request.(type) {
	case *sbreq.SteadySucc:
		idx = e.Block.Info.Index
	case *sbreq.SteadyReq:
		idx = e.Info.Index
	case *sbreq.ConfirmIndex:
		idx = e.Idx
	default:
		return common.NewBasicError("Invalid reply", nil)
	}
	key := conf.NotifyKey{
		Id:      pkt.Steady.ReqID,
		Idx:     idx,
		ReqType: pkt.Steady.Request.GetBase().Type,
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if c, ok := r.Notify[key.String()]; ok {
		select {
		case c <- pkt:
			return nil
		default:
			return common.NewBasicError("Drop reply due to full buffer", nil)
		}
	}
	log.Info("Ignore reply. No listener registered", "id", key.Id, "idx", key.Idx,
		"rt", key.ReqType)
	return nil
}

func (r *ResvMaster) Register(key *conf.NotifyKey, c chan *conf.ExtPkt) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.Notify[key.String()]; ok {
		return common.NewBasicError("Unable to add multiple listeners", nil,
			"id", key.Id, "idx", key.Idx, "rt", key.ReqType)
	}
	r.Notify[key.String()] = c
	return nil
}

func (r *ResvMaster) Deregister(key *conf.NotifyKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Notify, key.String())
}

func (r *ResvMaster) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, reqstr := range r.ResvHandls {
		reqstr.Close()
	}
}

func (r *ResvMaster) writeResvs(writer bufio.Writer, stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			m := conf.Get().LocalResvs.Items()
			ids := []sibra.ID(nil)
			set := map[string]struct{}{}
			for _, v := range m {
				if _, ok := set[string(v.Object.(*conf.LocalResvEntry).Id)]; ok {
					continue
				}
				ids = append(ids, v.Object.(*conf.LocalResvEntry).Id)
				set[string(v.Object.(*conf.LocalResvEntry).Id)] = struct{}{}
			}
			if _, err := writer.WriteString("Tick " + time.Now().String() + "\n"); err != nil {
				log.Error("Unable to write tick", err)
			}
			sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
			for _, k := range ids {
				all := conf.Get().LocalResvs.GetAll(k)
				if len(all) == 0 {
					continue
				}
				max := all[0]
				for _, v := range all {
					if max.Creation.Before(v.Creation) {
						max = v
					}
				}
				s := fmt.Sprintf("Master %s res %s\n", k, max.Block.Info)
				if _, err := writer.WriteString(s); err != nil {
					log.Error("Unable to write reservations", err)
				}
			}
		}
	}
}
