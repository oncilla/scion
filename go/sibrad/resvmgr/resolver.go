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
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

var requestID messenger.Counter

type resolver struct {
	log.Logger
	mu         sync.Mutex
	sciondConn *sciondConn
	conn       *snet.Conn
	msgr       *messenger.Messenger
	repMaster  repMaster
	timers     *Timers
	key        ResvKey
	store      *store
	events     chan<- *Event
	quitChan   chan<- quitSignal
	refireChan chan struct{}
	stop       <-chan struct{}
	closed     bool
}

func (r *resolver) run() {
	var err error
	var quit bool
	ticker := time.NewTicker(r.timers.NormalRefire)
	r.refireChan = make(chan struct{}, refireChanCap)
	defer log.LogPanicAndExit()
	for {
		select {
		case <-r.stop:
			goto Quit
		case <-ticker.C:
		case <-r.refireChan:
		}
		if quit, err = r.handle(); quit {
			goto Quit
		}
		if err != nil {
			r.notify(&Event{
				Code:  Error,
				Error: err,
			})
		}
	}
Quit:
	ticker.Stop()
	r.quit(err)
}

func (r *resolver) handle() (bool, error) {
	entry := r.store.getResv(r.key)
	if entry == nil {
		return true, common.NewBasicError("Entry does not exist", nil, "key", r.key)
	}
	entry.Lock()
	defer entry.Unlock()
	path := entry.getPath()
	if path == nil {
		return r.setupNewPath(entry)
	}
	// keep steady extension up-to-date
	ext := entry.syncResv.Load().Steady
	if ext == nil || ext.Expiry().Before(time.Now()) {
		if quit, err := r.resolveSteady(entry, path); quit || err != nil {
			return quit, err
		}
	}
	if entry.ephemMeta == nil {
		return false, nil
	} else {
		localSvcSB, err := r.getLocalSvcSB()
		if err != nil {
			return false, err
		}
		ext := entry.syncResv.Load().Ephemeral
		if ext != nil && ext.Expiry().Before(time.Now()) {
			r.notify(&Event{Code: ExtnExpired})
			entry.ephemMeta.state = start
			entry.syncResv.UpdateEphem(nil)
		}
		switch entry.ephemMeta.state {
		case ephemRequested, cleanUp:
			// Ignore. Need to wait until the state changes.
		case start:
			return r.setupEphem(entry, path, localSvcSB)
		case ephemExists:
			ext := entry.syncResv.Load().Ephemeral
			if ext == nil || ext.Expiry().Before(time.Now()) {
				return r.setupEphem(entry, path, localSvcSB)
			} else if r.needRenewal(ext, entry) {
				return r.renewEphem(entry, ext, path, localSvcSB)
			}
		}
	}
	return false, nil
}

func (r *resolver) setupNewPath(entry *resvEntry) (bool, error) {
	// FIXME(roosd): The steady meta possibly needs to be removed from the store
	return true, common.NewBasicError("Setting up new path not implemented", nil, "key", r.key)
}

// setupEphem starts setup requester. It assumes that entry is already locked.
func (r *resolver) setupEphem(entry *resvEntry, p *spathmeta.AppPath, sb *snet.Addr) (bool, error) {
	bwCls := entry.ephemMeta.maxBwCls
	if entry.ephemMeta.lastFailCode != sbreq.FailCodeNone {
		bwCls = maxBwCls(entry.ephemMeta.lastMaxBw, entry.ephemMeta.minBwCls)
	}
	steady := entry.syncResv.Load().Steady
	if steady == nil {
		return false, common.NewBasicError("Unable to steady reservation in syncResv", nil)
	}
	ephemId := sibra.NewEphemIDRand(snet.DefNetwork.IA().A)
	reqstr := &EphemSetup{
		reserver: &reserver{
			reqstr: &reqstr{
				Logger:     r.New("reqstr", "Setup "+ephemId.String()),
				id:         ephemId,
				entry:      entry,
				repMaster:  r.repMaster,
				msgr:       r.msgr,
				localSvcSB: sb,
				srcIA:      p.Entry.Path.SrcIA(),
				dstIA:      p.Entry.Path.DstIA(),
				srcHost:    r.conn.LocalAddr().(*snet.Addr).Host,
				dstHost:    entry.ephemMeta.remote,
				timeout:    combineRLC(steady),

				succFunc: func(_ reqstrI) {
					r.handleResvSucc(entry)
				},
				failFunc: func(i reqstrI) {
					r.handleResvFail(entry, start, i.GetPld().Data)
				},
				errFunc: func(err error, _ reqstrI) {
					r.handleSetupErr(entry, err)
				},
				timeFunc: func(i reqstrI) {
					request := i.GetPld().Data.(*sbreq.EphemReq)
					r.handleSetupTimeout(entry, request, p.Entry.Path.DstIA(), sb)
				},
			},
			bwCls: bwCls,
		},
	}
	entry.ephemMeta.state = ephemRequested
	go reqstr.Run(reqstr)
	return false, nil
}

func (r *resolver) handleSetupErr(entry *resvEntry, err error) {
	entry.Lock()
	if entry.ephemMeta.state == ephemRequested {
		entry.ephemMeta.state = start
	}
	entry.Unlock()
	r.notify(&Event{
		Code:  Error,
		Error: err,
	})
	r.refire(r.timers.ErrorRefire)
}

func (r *resolver) handleSetupTimeout(entry *resvEntry, request *sbreq.EphemReq, dstIa addr.IA,
	sb *snet.Addr) {

	entry.Lock()
	if entry.ephemMeta.state == ephemRequested {
		entry.ephemMeta.state = cleanUp
	}
	failedInfo := request.Block.Info.Copy()
	reqstr := &EphemCleanSetup{
		cleaner: &cleaner{
			reqstr: &reqstr{
				Logger:     r.New("reqstr", "Clean Setup "+request.ID.String()),
				id:         request.ID,
				idx:        failedInfo.Index,
				entry:      entry,
				repMaster:  r.repMaster,
				msgr:       r.msgr,
				localSvcSB: sb,
				srcIA:      r.conn.LocalAddr().(*snet.Addr).IA,
				dstIA:      dstIa,
				srcHost:    r.conn.LocalAddr().(*snet.Addr).Host,
				dstHost:    entry.ephemMeta.remote,
				timeout:    request.Block.Info.RLC.Duration(),

				succFunc: func(_ reqstrI) {
					r.handleCleanSucc(entry, start)
				},
				errFunc: func(err error, _ reqstrI) {
					r.handleCleanErr(entry, start, err)
				},
				timeFunc: func(i reqstrI) {
					r.handleSetupTimeout(entry, request, dstIa, sb)
				},
			},
			FailedInfo: failedInfo,
		},
	}
	entry.Unlock()
	go reqstr.Run(reqstr)

	r.notify(&Event{
		Code:  Error,
		Error: common.NewBasicError("Request timed out", nil, "key", r.key),
	})
	r.refire(r.timers.ErrorRefire)
}

func (r *resolver) needRenewal(ext *sbextn.Ephemeral, entry *resvEntry) bool {
	// FIXME(roosd): Add check if bandwidth class is as close to desired as possible.
	// Make sure to rate limit to 3 index per expiration SIBRA tick.
	return !sibra.CurrentTick().Add(1).Time().Before(ext.Expiry())
}

func (r *resolver) renewEphem(entry *resvEntry, ext *sbextn.Ephemeral,
	p *spathmeta.AppPath, sb *snet.Addr) (bool, error) {

	bwCls := entry.ephemMeta.maxBwCls
	if entry.ephemMeta.lastFailCode != sbreq.FailCodeNone {
		bwCls = maxBwCls(entry.ephemMeta.lastMaxBw, entry.ephemMeta.minBwCls)
	}
	ephem := entry.syncResv.Load().Ephemeral
	if ephem == nil {
		return false, common.NewBasicError("Unable to load ephem reservation from syncResv", nil)
	}
	ephemId := ephem.GetCurrID()
	reqstr := &EphemRenew{
		reserver: &reserver{
			reqstr: &reqstr{
				Logger:     r.New("reqstr", "Renew "+ephemId.String()),
				id:         ephemId,
				idx:        ext.ActiveBlocks[0].Info.Index.Add(1),
				entry:      entry,
				repMaster:  r.repMaster,
				msgr:       r.msgr,
				localSvcSB: sb,
				srcIA:      p.Entry.Path.SrcIA(),
				dstIA:      p.Entry.Path.DstIA(),
				srcHost:    r.conn.LocalAddr().(*snet.Addr).Host,
				dstHost:    entry.ephemMeta.remote,
				timeout:    ephem.ActiveBlocks[0].Info.RLC.Duration(),

				succFunc: func(_ reqstrI) {
					r.Debug("Ephemeral extension updated", "ext",
						entry.syncResv.Load().Ephemeral.ActiveBlocks[0])
					r.notify(&Event{Code: ExtnUpdated})
				},
				failFunc: func(i reqstrI) {
					r.handleResvFail(entry, ephemExists, i.GetPld().Data)
				},
				errFunc: func(err error, _ reqstrI) {
					r.handleResvErr(entry, ephemExists, err)
				},
				timeFunc: func(i reqstrI) {
					request := i.GetPld().Data.(*sbreq.EphemReq)
					r.handleRenewTimeout(entry, ephemId, request, p.Entry.Path.DstIA(), sb)
				},
			},
			bwCls: bwCls,
		},
	}
	entry.ephemMeta.state = ephemRequested
	go reqstr.Run(reqstr)
	return false, nil
}

func (r *resolver) handleRenewTimeout(entry *resvEntry, ephemId sibra.ID,
	request *sbreq.EphemReq, dstIa addr.IA, sb *snet.Addr) {

	entry.Lock()
	if entry.ephemMeta.state == ephemRequested {
		entry.ephemMeta.state = cleanUp
	}
	failedInfo := request.Block.Info.Copy()
	reqstr := &EphemCleanRenew{
		cleaner: &cleaner{
			reqstr: &reqstr{
				Logger:     r.New("reqstr", "Clean Renew "+ephemId.String()),
				id:         ephemId,
				idx:        failedInfo.Index,
				entry:      entry,
				repMaster:  r.repMaster,
				msgr:       r.msgr,
				localSvcSB: sb,
				srcIA:      r.conn.LocalAddr().(*snet.Addr).IA,
				dstIA:      dstIa,
				srcHost:    r.conn.LocalAddr().(*snet.Addr).Host,
				dstHost:    entry.ephemMeta.remote,
				timeout:    request.Block.Info.RLC.Duration(),

				succFunc: func(_ reqstrI) {
					r.handleCleanSucc(entry, start)
				},
				errFunc: func(err error, _ reqstrI) {
					r.handleCleanErr(entry, start, err)
				},
				timeFunc: func(i reqstrI) {
					r.handleRenewTimeout(entry, ephemId, request, dstIa, sb)
				},
			},
			FailedInfo: failedInfo,
		},
	}
	entry.Unlock()
	go reqstr.Run(reqstr)

	r.notify(&Event{
		Code:  Error,
		Error: common.NewBasicError("Request timed out", nil, "key", r.key),
	})
	r.refire(r.timers.ErrorRefire)
}

func (r *resolver) handleResvSucc(entry *resvEntry) {
	r.Debug("Ephemeral extension updated", "ext", entry.syncResv.Load().Ephemeral.ActiveBlocks[0])
	r.notify(&Event{Code: ExtnUpdated})
}

func (r *resolver) handleResvErr(entry *resvEntry, nextState state, err error) {
	entry.Lock()
	if entry.ephemMeta.state == ephemRequested {
		entry.ephemMeta.state = nextState
	}
	entry.Unlock()
	r.notify(&Event{
		Code:  Error,
		Error: err,
	})
	r.refire(r.timers.ErrorRefire)
}

func (r *resolver) handleResvFail(entry *resvEntry, nextState state, data sbreq.Data) {
	entry.Lock()
	defer entry.Unlock()
	if entry.ephemMeta.state == ephemRequested {
		entry.ephemMeta.state = nextState
	}
	switch entry.ephemMeta.lastFailCode {
	case sbreq.ClientDenied:
		r.quit(common.NewBasicError("Client denied reservation", nil))
		return
	case sbreq.SteadyOutdated, sbreq.SteadyNotExists:
		steady := entry.syncResv.Load().Steady
		entry.syncResv.UpdateSteady(nil)
		if steady != nil {
			for i, id := range steady.IDs {
				meta := r.store.getSteadyMeta(id)
				if meta != nil && meta.Meta.Block.Info.Index == steady.ActiveBlocks[i].Info.Index {
					r.store.removeSteadyMeta(id)
				}
			}
		}
	case sbreq.InvalidInfo:
		r.notify(&Event{
			Code:  Error,
			Error: common.NewBasicError("Invalid info", nil, "data", data),
		})
	case sbreq.BwExceeded:
		if entry.ephemMeta.lastMaxBw < entry.ephemMeta.minBwCls {
			r.quit(common.NewBasicError("Min available below requested", nil,
				"minReq", entry.ephemMeta.minBwCls, "minAvail", entry.ephemMeta.lastMaxBw))
			return
		}
	}
	r.refire(r.timers.ErrorRefire)
}

func (r *resolver) handleCleanSucc(entry *resvEntry, nextState state) {
	entry.Lock()
	if entry.ephemMeta.state == cleanUp {
		entry.ephemMeta.state = nextState
	}
	entry.Unlock()
	r.Info("Cleaned ephemeral extension")
	r.notify(&Event{Code: ExtnCleaned})
	r.refire(r.timers.ErrorRefire)
}

func (r *resolver) handleCleanErr(entry *resvEntry, nextState state, err error) {
	entry.Lock()
	if entry.ephemMeta.state == cleanUp {
		entry.ephemMeta.state = nextState
	}
	entry.Unlock()
	r.notify(&Event{
		Code:  Error,
		Error: err,
	})
	r.refire(r.timers.ErrorRefire)
}

func (r *resolver) getLocalSvcSB() (*snet.Addr, error) {
	saddr := r.sciondConn.SvcSB
	if saddr == nil {
		return nil, common.NewBasicError("SvcSB address not present", nil)
	}
	return saddr, nil
}

func (r *resolver) notify(e *Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.events <- e
	}
}

func (r *resolver) refire(wait time.Duration) {
	time.AfterFunc(wait, func() {
		r.refireChan <- struct{}{}
	})
}

func (r *resolver) quit(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		r.quitChan <- quitSignal{
			key: r.key,
			err: err,
		}
		r.events <- &Event{
			Code:  Quit,
			Error: err,
		}
		close(r.events)
		r.closed = true
	}
}

func combineRLC(steady *sbextn.Steady) time.Duration {
	var t time.Duration
	for _, v := range steady.ActiveBlocks {
		t += v.Info.RLC.Duration()
	}
	return t
}

func maxBwCls(a, b sibra.BwCls) sibra.BwCls {
	if a > b {
		return a
	}
	return b
}
