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
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

var _ sbalgo.EphemAdm = (*ephemAdm)(nil)

// XXX(roosd): this code is really ugly
type ephemAdm struct {
	*state.SibraState
}

func (e *ephemAdm) AdmitEphemSetup(steady *sbextn.Steady, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	if steady.IsTransfer() {
		return e.setupTrans(steady, p)
	}
	return e.setup(steady, p)
}

func (e *ephemAdm) setup(steady *sbextn.Steady, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	if !p.Accepted {
		// FIXME(roosd): avoid computations if failcode > bwexceeded
		r := p.Data.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
		if !ok {
			return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		// FIXME(roosd): check steady entry is not outdated
		res := sbalgo.EphemRes{
			MaxBw:    minBwCls(r.Info.BwCls, sibra.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	r := p.Data.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, true) {
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntry, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	_, ok = stEntry.EphemResvMap.Get(r.ID)
	if ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	alloc, ok, err := stEntry.EphemeralBW.AllocExpiring(
		uint64(reqBwCls.Bps()), r.Block.Info.ExpTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sbalgo.EphemRes{}, common.NewBasicError("Unable to alloc expiring", err)
	}
	if !ok {
		res := sbalgo.EphemRes{
			MaxBw:    sibra.Bps(alloc).ToBwCls(true),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	entry := &state.EphemResvEntry{
		SteadyEntry: stEntry,
		Id:          r.ID.Copy(),
		ActiveIdx: state.EphemResvIdx{
			Info:      *r.Block.Info,
			Allocated: alloc,
		},
	}
	if err := stEntry.EphemResvMap.Add(r.ID, entry); err != nil {
		stEntry.EphemeralBW.DeallocExpiring(alloc, r.Block.Info.ExpTick)
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	res := sbalgo.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) setupTrans(steady *sbextn.Steady, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	// FIXME(roosd): clean up is not handled correctly
	if !p.Accepted {
		r := p.Data.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
		if !ok {
			return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		res := sbalgo.EphemRes{
			MaxBw: minBwCls(r.Info.BwCls,
				sibra.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		stEntry, ok = e.SteadyMap.Get(steady.IDs[steady.CurrSteady+1])
		if !ok {
			return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		res.MaxBw = minBwCls(res.MaxBw,
			sibra.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true))
		return res, nil
	}
	r := p.Data.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, true) {
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntryBefore, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	stEntryAfter, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady+1])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	_, ok = stEntryBefore.EphemResvMap.Get(r.ID)
	if ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	_, ok = stEntryAfter.EphemResvMap.Get(r.ID)
	if ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	allocBefore, ok, err := stEntryBefore.EphemeralBW.AllocExpiring(
		uint64(reqBwCls.Bps()), r.Block.Info.ExpTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sbalgo.EphemRes{}, common.NewBasicError("Unable to alloc expiring on before", err)
	}
	res := sbalgo.EphemRes{
		MaxBw: reqBwCls,
	}
	if !ok {
		res.MaxBw = sibra.Bps(allocBefore).ToBwCls(true)
		res.FailCode = sbreq.BwExceeded
	}
	allocAfter, ok, err := stEntryAfter.EphemeralBW.AllocExpiring(
		uint64(reqBwCls.Bps()), r.Block.Info.ExpTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sbalgo.EphemRes{}, common.NewBasicError("Unable to alloc expiring on after", err)
	}
	if !ok || res.FailCode == sbreq.BwExceeded {
		res.MaxBw = minBwCls(res.MaxBw, sibra.Bps(allocAfter).ToBwCls(true))
		res.FailCode = sbreq.BwExceeded
		if res.FailCode == sbreq.FailCodeNone {
			stEntryBefore.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		}
		if ok {
			stEntryAfter.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		}
		return res, nil
	}
	entryBefore := &state.EphemResvEntry{
		SteadyEntry: stEntryBefore,
		Id:          r.ID.Copy(),
		ActiveIdx: state.EphemResvIdx{
			Info:      *r.Block.Info,
			Allocated: allocBefore,
		},
	}
	if err := stEntryBefore.EphemResvMap.Add(r.ID, entryBefore); err != nil {
		stEntryBefore.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		stEntryAfter.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	entryAfter := &state.EphemResvEntry{
		SteadyEntry: stEntryAfter,
		Id:          r.ID.Copy(),
		ActiveIdx: state.EphemResvIdx{
			Info:      *r.Block.Info,
			Allocated: allocAfter,
		},
	}
	if err := stEntryAfter.EphemResvMap.Add(r.ID, entryAfter); err != nil {
		stEntryBefore.EphemResvMap.Delete(r.ID)
		stEntryBefore.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		stEntryAfter.EphemeralBW.DeallocExpiring(allocAfter, r.Block.Info.ExpTick)
		return sbalgo.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	res = sbalgo.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) AdmitEphemRenew(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	if ephem.IsSteadyTransfer() {
		return e.renewTrans(ephem, p)
	}
	return e.renew(ephem, p)
}

func (e *ephemAdm) renew(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	if !p.Accepted {
		// FIXME(roosd): avoid computations if failcode > bwexceeded
		r := p.Data.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
		if !ok {
			return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		// FIXME(roosd): consider already reserved BW
		res := sbalgo.EphemRes{
			MaxBw: minBwCls(r.Info.BwCls,
				sibra.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	r := p.Data.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, false) {
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntry, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	ephemEntry, ok := stEntry.EphemResvMap.Get(ephem.IDs[0])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	oldBW := ephemEntry.ActiveIdx.Allocated
	oldTick := ephemEntry.ActiveIdx.Info.ExpTick
	alloc, ok, err := stEntry.EphemeralBW.ExchangeExpiring(uint64(reqBwCls.Bps()), oldBW,
		r.Block.Info.ExpTick, oldTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sbalgo.EphemRes{}, common.NewBasicError("Unable to exchange expiring", err)
	}
	if !ok {
		res := sbalgo.EphemRes{
			MaxBw:    sibra.Bps(alloc).ToBwCls(true),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	if err := ephemEntry.AddIdx(r.Block.Info, alloc); err != nil {
		stEntry.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()), oldBW,
			r.Block.Info.ExpTick, oldTick)
		res := sbalgo.EphemRes{
			FailCode: sbreq.InvalidInfo,
		}
		return res, nil
	}
	res := sbalgo.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) renewTrans(ephem *sbextn.Ephemeral, p *sbreq.Pld) (sbalgo.EphemRes, error) {
	if !p.Accepted {
		r := p.Data.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
		if !ok {
			return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		// FIXME(roosd): consider already reserved BW
		res := sbalgo.EphemRes{
			MaxBw: minBwCls(r.Info.BwCls,
				sibra.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		stEntry, ok = e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady+1])
		if !ok {
			return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		res.MaxBw = minBwCls(res.MaxBw,
			sibra.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true))
		return res, nil
	}
	r := p.Data.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, false) {
		return sbalgo.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntryBefore, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	stEntryAfter, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady+1])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	ephemEntryBefore, ok := stEntryBefore.EphemResvMap.Get(ephem.IDs[0])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	ephemEntryAfter, ok := stEntryAfter.EphemResvMap.Get(ephem.IDs[0])
	if !ok {
		return sbalgo.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	oldBWBefore := ephemEntryBefore.ActiveIdx.Allocated
	oldTickBefore := ephemEntryBefore.ActiveIdx.Info.ExpTick
	allocBefore, ok, err := stEntryBefore.EphemeralBW.ExchangeExpiring(uint64(reqBwCls.Bps()),
		oldBWBefore, r.Block.Info.ExpTick, oldTickBefore)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sbalgo.EphemRes{}, common.NewBasicError("Unable to alloc expiring on before", err)
	}
	res := sbalgo.EphemRes{
		MaxBw: reqBwCls,
	}
	if !ok {
		res.MaxBw = sibra.Bps(allocBefore).ToBwCls(true)
		res.FailCode = sbreq.BwExceeded
	}
	oldBWAfter := ephemEntryAfter.ActiveIdx.Allocated
	oldTickAfer := ephemEntryAfter.ActiveIdx.Info.ExpTick
	allocAfter, ok, err := stEntryAfter.EphemeralBW.ExchangeExpiring(uint64(reqBwCls.Bps()),
		oldBWAfter, r.Block.Info.ExpTick, oldTickAfer)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sbalgo.EphemRes{}, common.NewBasicError("Unable to alloc expiring on after", err)
	}
	if !ok || res.FailCode == sbreq.BwExceeded {
		res.MaxBw = minBwCls(res.MaxBw, sibra.Bps(allocAfter).ToBwCls(true))
		res.FailCode = sbreq.BwExceeded
		if res.FailCode == sbreq.FailCodeNone {
			stEntryBefore.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()),
				oldBWBefore, r.Block.Info.ExpTick, oldTickBefore)
		}
		if ok {
			stEntryAfter.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()),
				oldBWAfter, r.Block.Info.ExpTick, oldTickAfer)
		}
		return res, nil
	}
	if err := ephemEntryBefore.AddIdx(r.Block.Info, allocBefore); err != nil {
		stEntryBefore.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()),
			oldBWBefore, r.Block.Info.ExpTick, oldTickBefore)
		stEntryAfter.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()),
			oldBWAfter, r.Block.Info.ExpTick, oldTickAfer)
		res := sbalgo.EphemRes{
			FailCode: sbreq.InvalidInfo,
		}
		return res, nil
	}
	if err := ephemEntryAfter.AddIdx(r.Block.Info, allocAfter); err != nil {
		stEntryBefore.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()),
			oldBWBefore, r.Block.Info.ExpTick, oldTickBefore)
		stEntryAfter.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()),
			oldBWAfter, r.Block.Info.ExpTick, oldTickAfer)
		ephemEntryBefore.CleanUpIdx(r.Block.Info, &ephemEntryBefore.LastIdx.Info)
		res := sbalgo.EphemRes{
			FailCode: sbreq.InvalidInfo,
		}
		return res, nil
	}
	res = sbalgo.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) validateEphemInfo(info *sbresv.Info, setup bool) bool {
	return info.PathType == sibra.PathTypeEphemeral && info.BwCls != 0 &&
		info.ExpTick.Sub(sibra.CurrentTick()) <= sibra.MaxEphemTicks &&
		(!setup || info.Index == 0)
}

func (e *ephemAdm) CleanEphemSetup(steady *sbextn.Steady, p *sbreq.Pld) error {
	if steady.IsTransfer() {
		return e.setupCleanUpTrans(steady, p)
	}
	return e.setupCleanUp(steady, p)
}

func (e *ephemAdm) setupCleanUp(steady *sbextn.Steady, p *sbreq.Pld) error {
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
	return e.setupCleanEntry(id, steady.IDs[steady.CurrSteady], info, &sbresv.Info{})
}

func (e *ephemAdm) setupCleanUpTrans(steady *sbextn.Steady, p *sbreq.Pld) error {
	var info *sbresv.Info
	var id sibra.ID
	offA, offB := 1, 0
	switch r := p.Data.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
		id = r.ID
		// The response is traveling in the reverse direction.
		offA, offB = 0, -1
	case *sbreq.EphemClean:
		info = r.Info
		id = r.ID
	}
	// Clean the index for the reservation after the transfer in reservation direction.
	errA := e.setupCleanEntry(id, steady.IDs[steady.CurrSteady+offA],
		info, &sbresv.Info{})
	// Clean the index for the reservation before the transfer in reservation direction.
	errB := e.setupCleanEntry(id, steady.IDs[steady.CurrSteady+offB],
		info, &sbresv.Info{})
	switch {
	case errA != nil && errB != nil:
		return common.NewBasicError("Unable to clean both reservations", errA, "errB", errB)
	case errA != nil:
		return common.NewBasicError("Unable to clean reservation after transfer", errA)
	case errB != nil:
		return common.NewBasicError("Unable to clean reservation before transfer", errA)
	}
	return nil
}

func (e *ephemAdm) setupCleanEntry(ephemId, steadyId sibra.ID, failed, last *sbresv.Info) error {
	stEntry, ok := e.SteadyMap.Get(steadyId)
	if !ok {
		return common.NewBasicError("Steady does not exist", nil, "id", steadyId)
	}
	ephemEntry, ok := stEntry.EphemResvMap.Get(ephemId)
	if !ok {
		// Ephemeral already cleaned
		return nil
	}
	cleaned, err := ephemEntry.CleanUpIdx(failed, last)
	if err != nil {
		return err
	}
	stEntry.EphemResvMap.Delete(ephemId)
	return stEntry.EphemeralBW.DeallocExpiring(uint64(cleaned.BwCls.Bps()), cleaned.ExpTick)
}

func (e *ephemAdm) CleanEphemRenew(ephem *sbextn.Ephemeral, p *sbreq.Pld) error {
	// FIXME(roosd): implement
	return nil
}
