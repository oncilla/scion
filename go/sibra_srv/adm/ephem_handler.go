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

package adm

import (
	"context"
	"hash"
	"time"

	"github.com/scionproto/scion/go/lib/assert"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/infra"
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/sibra_srv/conf"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/util"
)

var RequestID messenger.Counter

type EphemHandler struct{}

//////////////////////////////////////////
// Handle Reservation at the end AS
/////////////////////////////////////////

func (h *EphemHandler) HandleSetupResvReqEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral request on end AS", "ids", pkt.Steady.IDs)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	if !pkt.Pld.Accepted {
		return h.reverseAndForward(pkt)
	}
	if err := h.sendReqToClient(pkt, h.getTimeout(pkt)); err != nil {
		log.Debug("Unable to send request to client", "err", err)
		res := sbalgo.EphemRes{
			FailCode: sbreq.ClientDenied,
			MaxBw:    0,
		}
		failEphemResv(pkt, pkt.Steady.Base, res)
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
		return h.reverseAndForward(pkt)
	}
	return nil
}

func (h *EphemHandler) getTimeout(pkt *conf.ExtPkt) time.Duration {
	var rlc sibra.RLC
	var numHops int
	if pkt.Steady != nil {
		rlc = pkt.Steady.GetCurrBlock().Info.RLC
		numHops = int(pkt.Steady.PathLens[pkt.Steady.CurrBlock])
	} else {
		rlc = pkt.Ephem.GetCurrBlock().Info.RLC
		numHops = pkt.Ephem.TotalHops
	}
	return rlc.Duration() / time.Duration(numHops)
}

func (h *EphemHandler) sendReqToClient(pkt *conf.ExtPkt, to time.Duration) error {
	msgr, ok := infra.MessengerFromContext(pkt.Req.Context())
	if !ok {
		return common.NewBasicError("No messenger found", nil)
	}
	buf, saddr, err := h.createClientPkt(pkt)
	if err != nil {
		return common.NewBasicError("Unable to create client packet", err)
	}
	pld := &sibra_mgmt.EphemReq{
		ExternalPkt: &sibra_mgmt.ExternalPkt{
			RawPkt: buf,
		},
	}
	ctx, cancleF := context.WithTimeout(pkt.Req.Context(), to)
	defer cancleF()
	return msgr.SendSibraEphemReq(ctx, pld, saddr, RequestID.Next())
}

func (h *EphemHandler) HandleRenewResvReqEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral request on end AS", "ids", pkt.Ephem.IDs)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	if !pkt.Pld.Accepted {
		return h.reverseAndForward(pkt)
	}
	if err := h.sendReqToClient(pkt, h.getTimeout(pkt)); err != nil {
		log.Debug("Unable to send request to client", "err", err)
		res := sbalgo.EphemRes{
			FailCode: sbreq.ClientDenied,
			MaxBw:    0,
		}
		failEphemResv(pkt, pkt.Ephem.Base, res)
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
		return h.reverseAndForward(pkt)
	}
	return nil
}

func (h *EphemHandler) HandleSetupResvRepEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on end AS", "ids", pkt.Steady.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvRepEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on end AS", "ids", pkt.Ephem.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleCleanSetupEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral clean up on end AS", "ids", pkt.Steady.IDs)
	// FIXME(roosd): end hosts should be notified
	if fc, err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady, pkt.Pld); err != nil {
		log.Error("Unable to clean ephemeral reservation", err, "code", fc)
	}
	return h.reverseAndForward(pkt)
}

func (h *EphemHandler) HandleCleanRenewEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral clean up on end AS", "ids", pkt.Ephem.IDs)
	// FIXME(roosd): end hosts should be notified
	if fc, err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem, pkt.Pld); err != nil {
		log.Error("Unable to clean ephemeral reservation", err, "code", fc)
	}
	return h.reverseAndForward(pkt)
}

func (h *EphemHandler) reverseAndForward(pkt *conf.ExtPkt) error {
	if err := h.reversePkt(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

////////////////////////////////////
// Handle Reservation at the transit AS
////////////////////////////////////

func (h *EphemHandler) HandleSetupResvReqHopAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral request on hop AS", "ids", pkt.Steady.IDs)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvReqHopAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral renew request on hop AS", "ids", pkt.Ephem.IDs)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleSetupResvRepHopAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on hop AS", "ids", pkt.Steady.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvRepHopAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on hop AS", "ids", pkt.Ephem.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return util.Forward(pkt)
}

////////////////////////////////////
// Handle Reservation at the transfer AS
////////////////////////////////////

func (h *EphemHandler) HandleSetupResvReqTransAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral request on transfer AS", "ids", pkt.Steady.IDs)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}
func (h *EphemHandler) HandleRenewResvReqTransAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral renew request on transfer AS", "ids", pkt.Ephem.IDs)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleSetupResvRepTransAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on transfer AS", "ids", pkt.Steady.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvRepTransAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on transfer AS", "ids", pkt.Ephem.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return util.Forward(pkt)
}

////////////////////////////////////
// Handle Reservation at the start AS
////////////////////////////////////

func (h *EphemHandler) HandleSetupResvReqStartAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral setup request on start AS", "ids", pkt.Steady.IDs)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvReqStartAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral renew request on start AS", "ids", pkt.Ephem.IDs)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleSetupResvRepStartAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on start AS", "ids", pkt.Steady.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return h.sendRepToClient(pkt)
}

func (h *EphemHandler) HandleRenewResvRepStartAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on start AS", "ids", pkt.Ephem.IDs)
	if !pkt.Pld.Accepted {
		if fc, err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem, pkt.Pld); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err, "code", fc)
		}
	}
	return h.sendRepToClient(pkt)
}

func (h *EphemHandler) HandleCleanStartAS(pkt *conf.ExtPkt) error {
	return h.sendRepToClient(pkt)
}

func (h *EphemHandler) sendRepToClient(pkt *conf.ExtPkt) error {
	msgr, ok := infra.MessengerFromContext(pkt.Req.Context())
	if !ok {
		return common.NewBasicError("No messenger found", nil)
	}
	buf, saddr, err := h.createClientPkt(pkt)
	if err != nil {
		return common.NewBasicError("Unable to create client packet", err)
	}
	pld := &sibra_mgmt.EphemRep{
		ExternalPkt: &sibra_mgmt.ExternalPkt{
			RawPkt: buf,
		},
	}
	log.Debug("Sending ephem resv to client", "addr", saddr)
	return msgr.SendSibraEphemRep(pkt.Req.Context(), pld, saddr, RequestID.Next())
}

func (h *EphemHandler) createClientPkt(pkt *conf.ExtPkt) (common.RawBytes, *snet.Addr, error) {
	buf, err := util.PackWithPld(pkt.Spkt, pkt.Pld)
	if err != nil {
		return nil, nil, err
	}
	saddr := &snet.Addr{
		IA:     pkt.Spkt.DstIA,
		Host:   pkt.Spkt.DstHost,
		L4Port: sibra.Port,
	}
	return buf, saddr, nil
}

/////////////////////////////////////////
// General functions
/////////////////////////////////////////

func (h *EphemHandler) HandleCleanSetup(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral clean up setup", "ids", pkt.Steady.IDs)
	fc, err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady, pkt.Pld)
	if err != nil {
		log.Error("Unable to clean ephemeral reservation (setup)", "err", err)
	}
	if pkt.Pld.Accepted && fc != sbreq.FailCodeNone {
		pkt.Pld.Accepted = false
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleCleanRenew(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral clean up renewal", "ids", pkt.Ephem.IDs)
	fc, err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem, pkt.Pld)
	if err != nil {
		log.Error("Unable to clean ephemeral reservation (renewal)", "err", err)
	}
	if pkt.Pld.Accepted && fc != sbreq.FailCodeNone {
		pkt.Pld.Accepted = false
	}
	return util.Forward(pkt)
}

func admitSetupEphemResv(pkt *conf.ExtPkt) error {
	res, err := pkt.Conf.SibraAlgo.AdmitEphemSetup(pkt.Steady, pkt.Pld)
	if err != nil {
		return err
	}
	if pkt.Pld.Accepted && res.FailCode == sbreq.FailCodeNone {
		ifids, err := util.GetResvIfids(pkt.Steady.Base, pkt.Spkt)
		if assert.On {
			assert.Must(err == nil, "GetResvIfids must succeed", "err", err)
		}
		if pkt.Steady.IsTransfer() {
			ifids.EgIfid = pkt.Steady.ActiveBlocks[pkt.Steady.CurrSteady+1].SOFields[0].Egress
		}
		ids := make([]sibra.ID, 1, 4)
		ids[0] = pkt.Pld.Data.(*sbreq.EphemReq).ID
		ids = append(ids, pkt.Steady.IDs...)
		err = issueSOF(pkt.Pld.Data.(*sbreq.EphemReq), ids, pkt.Steady.PathLens, ifids,
			pkt.Steady.CurrHop, pkt.Conf)
		if assert.On && err != nil {
			assert.Must(err == nil, "issueSOF must not fail", "err", err)
		}
	}
	if res.FailCode != sbreq.FailCodeNone {
		failEphemResv(pkt, pkt.Steady.Base, res)
	}
	return nil
}

func admitRenewEphemResv(pkt *conf.ExtPkt) error {
	res, err := pkt.Conf.SibraAlgo.AdmitEphemRenew(pkt.Ephem, pkt.Pld)
	if err != nil {
		return err
	}
	if pkt.Pld.Accepted && res.FailCode == sbreq.FailCodeNone {
		ifids, err := util.GetResvIfids(pkt.Ephem.Base, pkt.Spkt)
		if assert.On {
			assert.Must(err == nil, "GetResvIfids must succeed", "err", err)
		}
		err = issueSOF(pkt.Pld.Data.(*sbreq.EphemReq), pkt.Ephem.IDs, pkt.Ephem.PathLens, ifids,
			pkt.Ephem.CurrHop, pkt.Conf)
		if assert.On && err != nil {
			assert.Must(err == nil, "issueSOF must not fail", "err", err)
		}
	}
	if res.FailCode != sbreq.FailCodeNone {
		failEphemResv(pkt, pkt.Ephem.Base, res)
	}
	return nil
}

func issueSOF(req *sbreq.EphemReq, ids []sibra.ID, plens []uint8, ifids sbalgo.IFTuple,
	sofIdx int, conf *conf.Conf) error {

	mac := conf.SOFMacPool.Get().(hash.Hash)
	defer conf.SOFMacPool.Put(mac)
	return req.SetSOF(mac, ids, plens, ifids.InIfid, ifids.EgIfid, sofIdx)
}

func failEphemResv(pkt *conf.ExtPkt, base *sbextn.Base, res sbalgo.EphemRes) {
	if pkt.Pld.Accepted {
		r := pkt.Pld.Data.(*sbreq.EphemReq)
		pkt.Pld.Data = r.Fail(res.FailCode, res.MaxBw, base.CurrHop)
		pkt.Pld.Accepted = false
		// XXX(roosd): Uncomment to log which AS fails reservation
		// log.Debug("Failing reservation", "ids", base.IDs, "res", res, "data", pkt.Pld.Data)
	} else {
		r := pkt.Pld.Data.(*sbreq.EphemFailed)
		if r.FailCode < res.FailCode {
			r.FailCode = res.FailCode
		}
		r.Offers[base.CurrHop] = res.MaxBw
	}
}

func (h *EphemHandler) reversePkt(pkt *conf.ExtPkt) error {
	// FIXME(roosd): Remove when reversing extensions is supported.
	if pkt.Steady != nil {
		if _, err := pkt.Steady.Reverse(); err != nil {
			return err
		}
	} else {
		if _, err := pkt.Ephem.Reverse(); err != nil {
			return err
		}
	}
	if err := pkt.Spkt.Reverse(); err != nil {
		return err
	}
	if err := pkt.Pld.Reverse(); err != nil {
		return err
	}
	pkt.Spkt.SrcHost = pkt.Conf.PublicAddr.Host
	return nil
}
