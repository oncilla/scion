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

package impl

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

var _ sbalgo.EphemAdm = (*ephem)(nil)

type ephem struct {
	*state.SibraState
}

func (e *ephem) AdmitEphemSetup(steady *sbextn.Steady, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	if steady.IsTransfer() {
		return e.setupTrans(steady, p)
	}
	return e.setup(steady, p)
}

func (e *ephem) setup(steady *sbextn.Steady, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	info := e.extractInfo(p, true)
	if info == nil {
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntry, fc := e.checkSteady(steady.GetCurrID(), steady.GetCurrBlock().Info, p)
	if fc != sbreq.FailCodeNone {
		return sbalgo.EphemRes{FailCode: fc}, nil
	}
	if !p.Accepted {
		return e.checkBw(stEntry, info), nil
	}
	id := p.Data.(*sbreq.EphemReq).ID
	_, ok := stEntry.EphemResvMap.Get(id)
	if ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	res, alloc, err := e.allocExpiring(stEntry, info)
	if err != nil || res.FailCode != sbreq.FailCodeNone {
		return res, err
	}
	if err := e.addEntry(stEntry, id, info, alloc); err != nil {
		stEntry.EphemeralBW.DeallocExpiring(uint64(alloc), info.ExpTick)
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	return res, nil
}

func (e *ephem) setupTrans(steady *sbextn.Steady, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	info := e.extractInfo(p, true)
	if info == nil {
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stIngress, fc := e.checkSteady(steady.GetCurrID(), steady.GetCurrBlock().Info, p)
	if fc != sbreq.FailCodeNone {
		return sbalgo.EphemRes{FailCode: fc}, nil
	}
	stEgress, fc := e.checkSteady(steady.IDs[steady.CurrSteady+1],
		steady.ActiveBlocks[steady.CurrSteady+1].Info, p)
	if fc != sbreq.FailCodeNone {
		return sbalgo.EphemRes{FailCode: fc}, nil
	}
	if !p.Accepted {
		in := e.checkBw(stIngress, info)
		if eg := e.checkBw(stEgress, info); eg.MaxBw < in.MaxBw {
			return eg, nil
		}
		return in, nil
	}
	id := p.Data.(*sbreq.EphemReq).ID
	_, ok := stIngress.EphemResvMap.Get(id)
	if ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	_, ok = stEgress.EphemResvMap.Get(id)
	if ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	res, allocIngress, err := e.allocExpiring(stIngress, info)
	if err != nil || res.FailCode != sbreq.FailCodeNone {
		return res, err
	}
	res, allocEgress, err := e.allocExpiring(stEgress, info)
	if err != nil || res.FailCode != sbreq.FailCodeNone {
		stIngress.EphemeralBW.DeallocExpiring(uint64(allocIngress), info.ExpTick)
		return res, err
	}
	if err := e.addEntry(stIngress, id, info, allocIngress); err != nil {
		stIngress.EphemeralBW.DeallocExpiring(uint64(allocIngress), info.ExpTick)
		stEgress.EphemeralBW.DeallocExpiring(uint64(allocEgress), info.ExpTick)
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	if err := e.addEntry(stEgress, id, info, allocEgress); err != nil {
		stIngress.EphemResvMap.Delete(id)
		stIngress.EphemeralBW.DeallocExpiring(uint64(allocIngress), info.ExpTick)
		stEgress.EphemeralBW.DeallocExpiring(uint64(allocEgress), info.ExpTick)
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	return res, nil
}

func (e *ephem) checkSteady(id sibra.ID, info *sbresv.Info, p *sbreq.Pld) (
	*state.SteadyResvEntry, sbreq.FailCode) {

	stEntry, ok := e.SteadyMap.Get(id)
	if !ok {
		return nil, sbreq.SteadyNotExists
	}
	if info == nil {
		// Avoid locking unnecessarily
		return stEntry, sbreq.FailCodeNone
	}
	stEntry.RLock()
	defer stEntry.RUnlock()
	if !stEntry.Indexes[stEntry.ActiveIndex].Info.Eq(info) {
		return nil, sbreq.SteadyOutdated
	}
	return stEntry, sbreq.FailCodeNone
}

func (e *ephem) checkBw(stEntry *state.SteadyResvEntry, info *sbresv.Info) sbalgo.EphemRes {
	// No need to lock steady, since EphemeralBW has own lock.
	return sbalgo.EphemRes{
		MaxBw:    minBwCls(info.BwCls, sibra.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
		FailCode: sbreq.BwExceeded,
	}
}

func (e *ephem) allocExpiring(steady *state.SteadyResvEntry, info *sbresv.Info) (
	sbalgo.EphemRes, sibra.Bps, error) {
	alloc, ok, err := steady.EphemeralBW.AllocExpiring(uint64(info.BwCls.Bps()), info.ExpTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sbalgo.EphemRes{}, 0, common.NewBasicError("Unable to alloc expiring", err)
	}
	maxBw := sibra.Bps(alloc).ToBwCls(true)
	if !ok {
		res := sbalgo.EphemRes{
			AllocBw:  0,
			MaxBw:    maxBw,
			FailCode: sbreq.BwExceeded,
		}
		return res, 0, nil
	}
	res := sbalgo.EphemRes{
		AllocBw: maxBw,
		MaxBw:   maxBw,
	}
	return res, sibra.Bps(alloc), nil
}

func (e *ephem) addEntry(steady *state.SteadyResvEntry, id sibra.ID,
	info *sbresv.Info, alloc sibra.Bps) error {
	entry := &state.EphemResvEntry{
		SteadyEntry: steady,
		Id:          id.Copy(),
		ActiveIdx: state.EphemResvIdx{
			Info:      *info,
			Allocated: uint64(alloc),
		},
	}
	return steady.EphemResvMap.Add(id, entry)
}

func (e *ephem) AdmitEphemRenew(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	if ephem.IsSteadyTransfer() {
		return e.renewTrans(ephem, p)
	}
	return e.renew(ephem, p)
}

func (e *ephem) renew(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	info := e.extractInfo(p, false)
	if info == nil {
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntry, fc := e.checkSteady(ephem.SteadyIds()[ephem.CurrSteady], nil, p)
	if fc != sbreq.FailCodeNone {
		return sbalgo.EphemRes{FailCode: fc}, nil
	}
	if !p.Accepted {
		return e.checkBw(stEntry, info), nil
	}
	id := ephem.GetCurrID()
	ephemEntry, ok := stEntry.EphemResvMap.Get(id)
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	res, alloc, undo, err := e.exchangeExpiring(stEntry, ephemEntry, info)
	if err != nil || res.FailCode != sbreq.FailCodeNone {
		return res, err
	}
	if err := ephemEntry.AddIdx(info, uint64(alloc)); err != nil {
		stEntry.EphemeralBW.UndoExchangeExpiring(undo.newBw, undo.oldBw, undo.newTick, undo.oldTick)
		log.Info("Unable to add index", "info", info, "active", ephemEntry.ActiveIdx, "err", err)
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	return res, nil
}

func (e *ephem) renewTrans(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	info := e.extractInfo(p, false)
	if info == nil {
		log.Debug("invalid info validate")
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stIngress, fc := e.checkSteady(ephem.SteadyIds()[ephem.CurrSteady], nil, p)
	if fc != sbreq.FailCodeNone {
		return sbalgo.EphemRes{FailCode: fc}, nil
	}
	stEgress, fc := e.checkSteady(ephem.SteadyIds()[ephem.CurrSteady+1], nil, p)
	if fc != sbreq.FailCodeNone {
		return sbalgo.EphemRes{FailCode: fc}, nil
	}
	if !p.Accepted {
		in := e.checkBw(stIngress, info)
		if eg := e.checkBw(stEgress, info); eg.MaxBw < in.MaxBw {
			return eg, nil
		}
		return in, nil
	}
	id := ephem.GetCurrID()
	epIngress, ok := stIngress.EphemResvMap.Get(id)
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	epEgress, ok := stEgress.EphemResvMap.Get(id)
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	res, allocIngress, undoIngress, err := e.exchangeExpiring(stIngress, epIngress, info)
	if err != nil || res.FailCode != sbreq.FailCodeNone {
		return res, err
	}
	res, allocEgress, undoEgress, err := e.exchangeExpiring(stEgress, epEgress, info)
	if err != nil || res.FailCode != sbreq.FailCodeNone {
		stIngress.EphemeralBW.UndoExchangeExpiring(undoIngress.newBw,
			undoIngress.oldBw, undoIngress.newTick, undoIngress.oldTick)
		return res, err
	}
	if err := epIngress.AddIdx(info, uint64(allocIngress)); err != nil {
		stIngress.EphemeralBW.UndoExchangeExpiring(undoIngress.newBw,
			undoIngress.oldBw, undoIngress.newTick, undoIngress.oldTick)
		stEgress.EphemeralBW.UndoExchangeExpiring(undoEgress.newBw,
			undoEgress.oldBw, undoEgress.newTick, undoEgress.oldTick)
		log.Debug("invalid info ingress", "err", err)
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	if err := epEgress.AddIdx(info, uint64(allocEgress)); err != nil {
		stIngress.EphemeralBW.UndoExchangeExpiring(undoIngress.newBw,
			undoIngress.oldBw, undoIngress.newTick, undoIngress.oldTick)
		stEgress.EphemeralBW.UndoExchangeExpiring(undoEgress.newBw,
			undoEgress.oldBw, undoEgress.newTick, undoEgress.oldTick)
		epIngress.CleanUpIdx(info, &epIngress.LastIdx.Info)
		log.Debug("invalid info egress", "err", err)
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	return res, nil
}

type undo struct {
	newTick sibra.Tick
	oldTick sibra.Tick
	newBw   uint64
	oldBw   uint64
}

func (e *ephem) exchangeExpiring(stEntry *state.SteadyResvEntry,
	ephemEntry *state.EphemResvEntry, info *sbresv.Info) (sbalgo.EphemRes, sibra.Bps, undo, error) {

	u := undo{
		newTick: info.ExpTick,
		oldTick: ephemEntry.ActiveIdx.Info.ExpTick,
		newBw:   uint64(info.BwCls.Bps()),
		oldBw:   ephemEntry.ActiveIdx.Allocated,
	}
	alloc, ok, err := stEntry.EphemeralBW.ExchangeExpiring(u.newBw, u.oldBw, u.newTick, u.oldTick)
	if err != nil {
		// This should not be possible, since the info has been validated.
		return sbalgo.EphemRes{}, 0, undo{}, err
	}
	if !ok {
		res := sbalgo.EphemRes{
			MaxBw:    sibra.Bps(alloc).ToBwCls(true),
			FailCode: sbreq.BwExceeded,
		}
		return res, 0, undo{}, nil
	}
	maxBw := sibra.Bps(alloc).ToBwCls(true)
	res := sbalgo.EphemRes{
		AllocBw: maxBw,
		MaxBw:   maxBw,
	}
	return res, sibra.Bps(alloc), u, err
}

func (e *ephem) extractInfo(pld *sbreq.Pld, setup bool) *sbresv.Info {
	var info *sbresv.Info
	switch r := pld.Data.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
	case *sbreq.EphemReq:
		info = r.Block.Info
	}
	if !e.validateInfo(info, setup) {
		return nil
	}
	return info
}

func (e *ephem) validateInfo(info *sbresv.Info, setup bool) bool {
	currTick := sibra.CurrentTick()
	return !(info.PathType != sibra.PathTypeEphemeral || info.BwCls == 0 ||
		info.ExpTick.Sub(currTick) > sibra.MaxEphemTicks || info.ExpTick <= currTick ||
		(setup && info.Index != 0))
}

func (e *ephem) CleanEphemSetup(steady *sbextn.Steady, p *sbreq.Pld) (sbreq.FailCode, error) {
	if steady.IsTransfer() {
		return e.setupCleanUpTrans(steady, p)
	}
	return e.setupCleanUp(steady, p)
}

func (e *ephem) setupCleanUp(steady *sbextn.Steady, p *sbreq.Pld) (sbreq.FailCode, error) {
	var info *sbresv.Info
	var id sibra.ID
	switch r := p.Data.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
		id = r.ID
	case *sbreq.EphemClean:
		info = r.Info
		id = r.ID
	}
	if !e.validateInfo(info, true) {
		return sbreq.InvalidInfo, nil
	}
	return e.cleanEntry(id, steady.IDs[steady.CurrSteady], info, &sbresv.Info{})
}

func (e *ephem) setupCleanUpTrans(steady *sbextn.Steady, p *sbreq.Pld) (sbreq.FailCode, error) {
	var info *sbresv.Info
	var id sibra.ID
	offIngress, offEgress := 1, 0
	switch r := p.Data.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
		id = r.ID
		// The response is traveling in the reverse direction.
		offIngress, offEgress = 0, -1
	case *sbreq.EphemClean:
		info = r.Info
		id = r.ID
	}
	if !e.validateInfo(info, true) {
		return sbreq.InvalidInfo, nil
	}
	// Clean the index for the reservation after the transfer in reservation direction.
	fcIngress, err := e.cleanEntry(id, steady.IDs[steady.CurrSteady+offIngress],
		info, &sbresv.Info{})
	// Clean the index for the reservation before the transfer in reservation direction.
	fcEgress, errEgress := e.cleanEntry(id, steady.IDs[steady.CurrSteady+offEgress],
		info, &sbresv.Info{})
	if err != nil || errEgress != nil {
		err = common.NewBasicError("Unable to clean entry", err, "egress", errEgress)
	}
	if fcIngress < fcEgress {
		return fcEgress, err
	}
	return fcIngress, err
}

func (e *ephem) CleanEphemRenew(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbreq.FailCode, error) {
	if ephem.IsTransfer() {
		return e.renewCleanUpTrans(ephem, p)
	}
	return e.renewCleanUp(ephem, p)
}

func (e *ephem) renewCleanUp(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbreq.FailCode, error) {
	var info *sbresv.Info
	switch r := p.Data.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
	case *sbreq.EphemClean:
		info = r.Info
	}
	if !e.validateInfo(info, true) {
		return sbreq.InvalidInfo, nil
	}
	return e.cleanEntry(ephem.GetCurrID(), ephem.SteadyIds()[ephem.CurrSteady], info,
		ephem.GetCurrBlock().Info)
}

func (e *ephem) renewCleanUpTrans(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbreq.FailCode, error) {
	var info *sbresv.Info
	offIngress, offEgress := 1, 0
	switch r := p.Data.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
		// The response is traveling in the reverse direction.
		offIngress, offEgress = 0, -1
	case *sbreq.EphemClean:
		info = r.Info
	}
	if !e.validateInfo(info, true) {
		return sbreq.InvalidInfo, nil
	}
	// Clean the index for the reservation after the transfer in reservation direction.
	fcIngress, err := e.cleanEntry(ephem.GetCurrID(),
		ephem.SteadyIds()[ephem.CurrSteady+offIngress], info, ephem.GetCurrBlock().Info)
	// Clean the index for the reservation before the transfer in reservation direction.
	fcEgress, errEgress := e.cleanEntry(ephem.GetCurrID(),
		ephem.SteadyIds()[ephem.CurrSteady+offEgress], info, ephem.GetCurrBlock().Info)
	if err != nil || errEgress != nil {
		err = common.NewBasicError("Unable to clean entry", err, "egress", errEgress)
	}
	if fcIngress < fcEgress {
		return fcEgress, err
	}
	return fcIngress, err
}

func (e *ephem) cleanEntry(ephemId, steadyId sibra.ID, failed,
	last *sbresv.Info) (sbreq.FailCode, error) {

	stEntry, ok := e.SteadyMap.Get(steadyId)
	if !ok {
		return sbreq.SteadyNotExists, nil
	}
	ephemEntry, ok := stEntry.EphemResvMap.Get(ephemId)
	if !ok {
		// Ephemeral already cleaned
		return sbreq.FailCodeNone, nil
	}
	cleaned, err := ephemEntry.CleanUpIdx(failed, last)
	if err != nil {
		return sbreq.InvalidInfo, nil
	}
	stEntry.EphemResvMap.Delete(ephemId)
	// An error should not occur since we sanity checked the info already and the
	// expiration tick must be in the range.
	err = stEntry.EphemeralBW.DeallocExpiring(uint64(cleaned.BwCls.Bps()), cleaned.ExpTick)
	return sbreq.FailCodeNone, err
}
