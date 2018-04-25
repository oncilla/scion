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
	"sync"
	"time"

	log "github.com/inconshreveable/log15"

	"fmt"

	"context"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/sibra/resvdb/query"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/sibra_srv/conf"
	"github.com/scionproto/scion/go/sibra_srv/dist"
)

const (
	RegInterval = sbresv.TickDuration
	RegTimeout  = 1 * time.Second
)

type NotifyReg struct {
	Id  sbresv.ID
	Idx sbresv.Index
	Exp sbresv.Tick
}

type RegState struct {
	NotifyReg
	Done bool
}

func (s *RegState) Update(newReg NotifyReg) bool {
	if !s.Id.Eq(newReg.Id) || s.Idx != newReg.Idx || s.Exp != newReg.Exp {
		s.Id = newReg.Id.Copy()
		s.Idx = newReg.Idx
		s.Exp = newReg.Exp
		s.Done = false
		return true
	}
	return false
}

type Register struct {
	sync.Mutex
	state     RegState
	resvKey   string
	ticker    *time.Ticker
	notifyReg chan NotifyReg
	stop      chan struct{}
	stopped   bool
}

func (r *Register) Run() {
	r.ticker = time.NewTicker(RegInterval)
	for {
		select {
		case <-r.stop:
			log.Info(r.pretty("Register stopped"))
			return
		case n := <-r.notifyReg:
			if r.state.Update(n) {
				if err := r.Register(); err != nil {
					log.Error(r.pretty("Unable to register path on notify"), "err", err)
				}
			}
		case <-r.ticker.C:
			if !r.state.Done && r.state.Id != nil {
				if err := r.Register(); err != nil {
					log.Error(r.pretty("Unable to register path on tick"), "err", err)
				}
			}
		}
	}
}

func (r *Register) Register() error {
	// FIXME(roosd): Implement registration
	return common.NewBasicError("Not implemented", nil)
	p := &query.Params{
		ResvID: r.state.Id,
	}
	config := conf.Get()
	results, err := conf.Get().ResvDB.Get(p)
	if err != nil {
		return err
	}
	if len(results) < 1 {
		return common.NewBasicError("No path found", nil, "len", len(results))
	}
	bmeta := results[0].BlockMeta
	if bmeta.Block.Info.Index != r.state.Idx {
		return common.NewBasicError("Block idx does not match", nil,
			"expected", r.state.Idx, "actual", bmeta.Block.Info.Index)
	}
	if bmeta.Block.Info.ExpTick != r.state.Exp {
		return common.NewBasicError("Block exp tick does not match", nil,
			"expected", r.state.Exp, "actual", bmeta.Block.Info.ExpTick)
	}

	ctx, cancelF := context.WithTimeout(context.Background(), RegTimeout)
	defer cancelF()
	pld := &sibra_mgmt.SteadyReg{
		SteadyRecs: &sibra_mgmt.SteadyRecs{
			Entries: []*sibra_mgmt.BlockMeta{bmeta},
		},
	}
	// FIXME(roosd): select correct sibra service address.
	sb := &snet.Addr{}
	_, err = config.Messenger.RegisterSibraSteady(ctx, pld, sb, dist.RequestID.Next())
	if err != nil {
		return common.NewBasicError("Unable to register reservation", err,
			"id", r.state.Id, "idx", r.state.Id)
	}
	r.state.Done = true
	return nil
}

func (r *Register) pretty(msg string) string {
	return fmt.Sprintf("[Reg %s] %s", r.resvKey, msg)
}

func (r *Register) Close() {
	r.Lock()
	defer r.Unlock()
	if !r.stopped {
		close(r.stop)
		r.stopped = true
	}
}

func (r *Register) Closed() bool {
	r.Lock()
	defer r.Unlock()
	return r.stopped
}
