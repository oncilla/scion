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

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
)

const (
	RegInterval = sibra.TickDuration
)

type NotifyReg struct {
	Id  sibra.ID
	Idx sibra.Index
	Exp sibra.Tick
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
			log.Info("Register stopped")
			return
		case n := <-r.notifyReg:
			if r.state.Update(n) {
				if err := r.Register(); err != nil {
					log.Error("Unable to register path on notify", "err", err)
				}
			}
		case <-r.ticker.C:
			if !r.state.Done && r.state.Id != nil {
				if err := r.Register(); err != nil {
					log.Error("Unable to register path on tick", "err", err)
				}
			}
		}
	}
}

func (r *Register) Register() error {
	// FIXME(roosd): Implement registration
	return common.NewBasicError("Register not implemented", nil)

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
