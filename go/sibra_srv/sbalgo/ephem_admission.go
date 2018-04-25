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

package sbalgo

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/sibra"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

var _ sibra.EphemAdm = (*ephemAdm)(nil)

type ephemAdm struct {
	*state.SibraState
}

func (e *ephemAdm) AdmitEphemSetup(steady *sbextn.Steady) (sibra.EphemRes, error) {
	if steady.IsTransfer() {
		return e.ephemSetupTransAdm(steady)
	}
	return e.ephemSetupAdm(steady)
}

func (e *ephemAdm) ephemSetupAdm(steady *sbextn.Steady) (sibra.EphemRes, error) {
	if !steady.Request.GetBase().Accepted {
		// FIXME(roosd): avoid computations if failcode > bwexceeded
		r := steady.Request.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
		if !ok {
			return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		// FIXME(roosd): check steady entry is not outdated
		res := sibra.EphemRes{
			MaxBw:    minBwCls(r.Info.BwCls, sbresv.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	r := steady.Request.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, true) {
		return sibra.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntry, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	_, ok = stEntry.EphemResvMap.Get(steady.ReqID)
	if ok {
		return sibra.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	alloc, ok, err := stEntry.EphemeralBW.AllocExpiring(
		uint64(reqBwCls.Bps()), r.Block.Info.ExpTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sibra.EphemRes{}, common.NewBasicError("Unable to alloc expiring", err)
	}
	if !ok {
		res := sibra.EphemRes{
			MaxBw:    sbresv.Bps(alloc).ToBwCls(true),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	entry := &state.EphemResvEntry{
		SteadyEntry: stEntry,
		Id:          steady.ReqID.Copy(),
		ActiveIdx: state.EphemResvIdx{
			Info:      *r.Block.Info,
			Allocated: alloc,
		},
	}
	if err := stEntry.EphemResvMap.Add(steady.ReqID, entry); err != nil {
		stEntry.EphemeralBW.DeallocExpiring(alloc, r.Block.Info.ExpTick)
		return sibra.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	res := sibra.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) ephemSetupTransAdm(steady *sbextn.Steady) (sibra.EphemRes, error) {
	// FIXME(roosd): clean up is not handled correctly
	if !steady.Request.GetBase().Accepted {
		r := steady.Request.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
		if !ok {
			return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		res := sibra.EphemRes{
			MaxBw: minBwCls(r.Info.BwCls,
				sbresv.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		stEntry, ok = e.SteadyMap.Get(steady.IDs[steady.CurrSteady+1])
		if !ok {
			return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		res.MaxBw = minBwCls(res.MaxBw,
			sbresv.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true))
		return res, nil
	}
	r := steady.Request.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, true) {
		return sibra.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntryBefore, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady])
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	stEntryAfter, ok := e.SteadyMap.Get(steady.IDs[steady.CurrSteady+1])
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	_, ok = stEntryBefore.EphemResvMap.Get(steady.ReqID)
	if ok {
		return sibra.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	_, ok = stEntryAfter.EphemResvMap.Get(steady.ReqID)
	if ok {
		return sibra.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	allocBefore, ok, err := stEntryBefore.EphemeralBW.AllocExpiring(
		uint64(reqBwCls.Bps()), r.Block.Info.ExpTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sibra.EphemRes{}, common.NewBasicError("Unable to alloc expiring on before", err)
	}
	res := sibra.EphemRes{
		MaxBw: reqBwCls,
	}
	if !ok {
		res.MaxBw = sbresv.Bps(allocBefore).ToBwCls(true)
		res.FailCode = sbreq.BwExceeded
	}
	allocAfter, ok, err := stEntryAfter.EphemeralBW.AllocExpiring(
		uint64(reqBwCls.Bps()), r.Block.Info.ExpTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sibra.EphemRes{}, common.NewBasicError("Unable to alloc expiring on after", err)
	}
	if !ok || res.FailCode == sbreq.BwExceeded {
		res.MaxBw = minBwCls(res.MaxBw, sbresv.Bps(allocAfter).ToBwCls(true))
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
		Id:          steady.ReqID.Copy(),
		ActiveIdx: state.EphemResvIdx{
			Info:      *r.Block.Info,
			Allocated: allocBefore,
		},
	}
	if err := stEntryBefore.EphemResvMap.Add(steady.ReqID, entryBefore); err != nil {
		stEntryBefore.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		stEntryAfter.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		return sibra.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	entryAfter := &state.EphemResvEntry{
		SteadyEntry: stEntryAfter,
		Id:          steady.ReqID.Copy(),
		ActiveIdx: state.EphemResvIdx{
			Info:      *r.Block.Info,
			Allocated: allocAfter,
		},
	}
	if err := stEntryAfter.EphemResvMap.Add(steady.ReqID, entryAfter); err != nil {
		stEntryBefore.EphemResvMap.Delete(steady.ReqID)
		stEntryBefore.EphemeralBW.DeallocExpiring(allocBefore, r.Block.Info.ExpTick)
		stEntryAfter.EphemeralBW.DeallocExpiring(allocAfter, r.Block.Info.ExpTick)
		return sibra.EphemRes{FailCode: sbreq.EphemExists}, nil
	}
	res = sibra.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) AdmitEphemRenew(ephem *sbextn.Ephemeral) (sibra.EphemRes, error) {
	if ephem.IsSteadyTransfer() {
		return e.ephemRenewTransAdm(ephem)
	}
	return e.ephemRenewAdm(ephem)
}

func (e *ephemAdm) ephemRenewAdm(ephem *sbextn.Ephemeral) (sibra.EphemRes, error) {
	if !ephem.Request.GetBase().Accepted {
		// FIXME(roosd): avoid computations if failcode > bwexceeded
		r := ephem.Request.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
		if !ok {
			return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		// FIXME(roosd): consider already reserved BW
		res := sibra.EphemRes{
			MaxBw: minBwCls(r.Info.BwCls,
				sbresv.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	r := ephem.Request.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, false) {
		return sibra.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntry, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	ephemEntry, ok := stEntry.EphemResvMap.Get(ephem.ReqID)
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	oldBW := ephemEntry.ActiveIdx.Allocated
	oldTick := ephemEntry.ActiveIdx.Info.ExpTick
	alloc, ok, err := stEntry.EphemeralBW.ExchangeExpiring(uint64(reqBwCls.Bps()), oldBW,
		r.Block.Info.ExpTick, oldTick)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sibra.EphemRes{}, common.NewBasicError("Unable to exchange expiring", err)
	}
	if !ok {
		res := sibra.EphemRes{
			MaxBw:    sbresv.Bps(alloc).ToBwCls(true),
			FailCode: sbreq.BwExceeded,
		}
		return res, nil
	}
	if err := ephemEntry.AddIdx(r.Block.Info, alloc); err != nil {
		stEntry.EphemeralBW.UndoExchangeExpiring(uint64(reqBwCls.Bps()), oldBW,
			r.Block.Info.ExpTick, oldTick)
		res := sibra.EphemRes{
			FailCode: sbreq.InvalidInfo,
		}
		return res, nil
	}
	res := sibra.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) ephemRenewTransAdm(ephem *sbextn.Ephemeral) (sibra.EphemRes, error) {
	if !ephem.Request.GetBase().Accepted {
		r := ephem.Request.(*sbreq.EphemFailed)
		stEntry, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
		if !ok {
			return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		// FIXME(roosd): consider already reserved BW
		res := sibra.EphemRes{
			MaxBw: minBwCls(r.Info.BwCls,
				sbresv.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true)),
			FailCode: sbreq.BwExceeded,
		}
		stEntry, ok = e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady+1])
		if !ok {
			return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
		}
		res.MaxBw = minBwCls(res.MaxBw,
			sbresv.Bps(stEntry.EphemeralBW.Free()).ToBwCls(true))
		return res, nil
	}
	r := ephem.Request.(*sbreq.EphemReq)
	if !e.validateEphemInfo(r.Block.Info, false) {
		return sibra.EphemRes{FailCode: sbreq.InvalidInfo}, nil
	}
	stEntryBefore, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady])
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	stEntryAfter, ok := e.SteadyMap.Get(ephem.SteadyIds()[ephem.CurrSteady+1])
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.SteadyNotExists}, nil
	}
	ephemEntryBefore, ok := stEntryBefore.EphemResvMap.Get(ephem.ReqID)
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	ephemEntryAfter, ok := stEntryAfter.EphemResvMap.Get(ephem.ReqID)
	if !ok {
		return sibra.EphemRes{FailCode: sbreq.EphemNotExists}, nil
	}
	reqBwCls := r.Block.Info.BwCls
	oldBWBefore := ephemEntryBefore.ActiveIdx.Allocated
	oldTickBefore := ephemEntryBefore.ActiveIdx.Info.ExpTick
	allocBefore, ok, err := stEntryBefore.EphemeralBW.ExchangeExpiring(uint64(reqBwCls.Bps()),
		oldBWBefore, r.Block.Info.ExpTick, oldTickBefore)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sibra.EphemRes{}, common.NewBasicError("Unable to alloc expiring on before", err)
	}
	res := sibra.EphemRes{
		MaxBw: reqBwCls,
	}
	if !ok {
		res.MaxBw = sbresv.Bps(allocBefore).ToBwCls(true)
		res.FailCode = sbreq.BwExceeded
	}
	oldBWAfter := ephemEntryAfter.ActiveIdx.Allocated
	oldTickAfer := ephemEntryAfter.ActiveIdx.Info.ExpTick
	allocAfter, ok, err := stEntryAfter.EphemeralBW.ExchangeExpiring(uint64(reqBwCls.Bps()),
		oldBWAfter, r.Block.Info.ExpTick, oldTickAfer)
	if err != nil {
		// This should not be possible, since the info has been validated above.
		return sibra.EphemRes{}, common.NewBasicError("Unable to alloc expiring on after", err)
	}
	if !ok || res.FailCode == sbreq.BwExceeded {
		res.MaxBw = minBwCls(res.MaxBw, sbresv.Bps(allocAfter).ToBwCls(true))
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
		res := sibra.EphemRes{
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
		res := sibra.EphemRes{
			FailCode: sbreq.InvalidInfo,
		}
		return res, nil
	}
	res = sibra.EphemRes{
		AllocBw:  reqBwCls,
		MaxBw:    reqBwCls,
		FailCode: sbreq.FailCodeNone,
	}
	return res, nil
}

func (e *ephemAdm) validateEphemInfo(info *sbresv.Info, setup bool) bool {
	return info.PathType == sbresv.PathTypeEphemeral && info.BwCls != 0 &&
		info.ExpTick.Sub(sbresv.CurrentTick()) <= sbresv.MaxEphemTicks &&
		(!setup || info.Index == 0)
}

func (e *ephemAdm) CleanEphemSetup(steady *sbextn.Steady) error {
	if steady.IsTransfer() {
		return e.ephemSetupTransCleanUp(steady)
	}
	return e.ephemSetupCleanUp(steady)
}

func (e *ephemAdm) ephemSetupCleanUp(steady *sbextn.Steady) error {
	var info *sbresv.Info
	switch r := steady.Request.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
	case *sbreq.EphemClean:
		info = r.Info
	}
	return e.ephemSetupCleanupEntry(steady.ReqID, steady.IDs[steady.CurrSteady], info, &sbresv.Info{})
}

func (e *ephemAdm) ephemSetupTransCleanUp(steady *sbextn.Steady) error {
	var info *sbresv.Info
	offA, offB := 1, 0
	switch r := steady.Request.(type) {
	case *sbreq.EphemFailed:
		info = r.Info
		// The response is traveling in the reverse direction.
		offA, offB = 0, -1
	case *sbreq.EphemClean:
		info = r.Info
	}
	// Clean the index for the reservation after the transfer in reservation direction.
	errA := e.ephemSetupCleanupEntry(steady.ReqID, steady.IDs[steady.CurrSteady+offA],
		info, &sbresv.Info{})
	// Clean the index for the reservation before the transfer in reservation direction.
	errB := e.ephemSetupCleanupEntry(steady.ReqID, steady.IDs[steady.CurrSteady+offB],
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

func (e *ephemAdm) ephemSetupCleanupEntry(ephemId, steadyId sbresv.ID, failed, last *sbresv.Info) error {
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

func (e *ephemAdm) CleanEphemRenew(ephem *sbextn.Ephemeral) error {
	// FIXME(roosd): implement
	return nil
}
