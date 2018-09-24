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
	"time"

	"github.com/scionproto/scion/go/lib/assert"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/hpkt"
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

// TODO(roosd): sanatizing should include that requests must be sent reverse when reversed path
// TODO(roosd): request -> Fwd direction
// TODO(roosd): cleanup error messages -> id etc

// FIXME(roosd): Disallow reservation size 0

//////////////////////////////////////////
// Handle Reservation at the end AS
/////////////////////////////////////////

func (h *EphemHandler) HandleSetupResvReqEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral request on end AS", "id", pkt.Steady.ReqID)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	if !pkt.Steady.Request.GetBase().Accepted {
		return h.reverseAndForward(pkt)
	}
	if err := h.sendReqToClient(pkt, h.getTimeout(pkt)); err != nil {
		log.Debug("Unable to send request to client", "err", err)
		res := sbalgo.EphemRes{
			FailCode: sbreq.ClientDenied,
			MaxBw:    0,
		}
		failEphemResv(pkt.Steady.Base, res)
		if err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err)
		}
		return h.reverseAndForward(pkt)
	}
	return nil
}

func (h *EphemHandler) getTimeout(pkt *conf.ExtPkt) time.Duration {
	var rttCls sibra.RttCls
	var numHops int
	if pkt.Steady != nil {
		rttCls = pkt.Steady.GetCurrBlock().Info.RttCls
		numHops = int(pkt.Steady.PathLens[pkt.Steady.CurrBlock])
	} else {
		rttCls = pkt.Ephem.GetCurrBlock().Info.RttCls
		numHops = pkt.Ephem.TotalHops
	}
	return rttCls.Duration() / time.Duration(numHops)
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
	log.Debug("Handling ephemeral request on end AS", "id", pkt.Ephem.ReqID)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	if !pkt.Ephem.Request.GetBase().Accepted {
		return h.reverseAndForward(pkt)
	}
	if err := h.sendReqToClient(pkt, h.getTimeout(pkt)); err != nil {
		log.Debug("Unable to send request to client", "err", err)
		res := sbalgo.EphemRes{
			FailCode: sbreq.ClientDenied,
			MaxBw:    0,
		}
		failEphemResv(pkt.Ephem.Base, res)
		if err := pkt.Conf.SibraAlgo.CleanEphemRenew(pkt.Ephem); err != nil {
			log.Error("Unable to clean ephemeral reservation", "err", err)
		}
		return h.reverseAndForward(pkt)
	}
	return nil
}

func (h *EphemHandler) HandleSetupResvRepEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on end AS", "id", pkt.Steady.ReqID)
	if !pkt.Steady.Request.GetBase().Accepted {
		if err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady); err != nil {
			log.Error("Unable to clean ephemeral reservation", "er", err)
		}
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvRepEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on end AS", "id", pkt.Ephem.ReqID)
	// TODO(roosd): check if failed or not
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleCleanSetupEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral clean up on end AS", "id", pkt.Steady.ReqID)
	if err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady); err != nil {
		return common.NewBasicError("Unable to clean ephemeral reservation", err)
	}
	return h.reverseAndForward(pkt)
}

func (h *EphemHandler) HandleCleanRenewEndAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral clean up on end AS", "id", pkt.Ephem.ReqID)
	// TODO(roosd)
	return common.NewBasicError("Not implemented", nil)
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
	// TODO(roosd): sanity check -> e.g. only requests
	log.Debug("Handling ephemeral request on hop AS", "id", pkt.Steady.ReqID)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvReqHopAS(pkt *conf.ExtPkt) error {
	// TODO(roosd): sanity check -> e.g. only requests
	log.Debug("Handling ephemeral renew request on hop AS", "id", pkt.Ephem.ReqID)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleSetupResvRepHopAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on hop AS", "id", pkt.Steady.ReqID)
	if !pkt.Steady.Request.GetBase().Accepted {
		if err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady); err != nil {
			log.Error("Unable to clean ephemeral reservation", "er", err)
		}
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvRepHopAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on hop AS", "id", pkt.Ephem.ReqID)
	// TODO(roosd): check if failed or not
	return util.Forward(pkt)
}

////////////////////////////////////
// Handle Reservation at the transfer AS
////////////////////////////////////

func (h *EphemHandler) HandleSetupResvReqTransAS(pkt *conf.ExtPkt) error {
	// TODO(roosd): sanity check -> e.g. only requests
	log.Debug("Handling ephemeral request on transfer AS", "id", pkt.Steady.ReqID)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}
func (h *EphemHandler) HandleRenewResvReqTransAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral renew request on transfer AS", "id", pkt.Ephem.ReqID)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleSetupResvRepTransAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on transfer AS", "id", pkt.Steady.ReqID)
	if !pkt.Steady.Request.GetBase().Accepted {
		if err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady); err != nil {
			log.Error("Unable to clean ephemeral reservation", "er", err)
		}
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvRepTransAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on transfer AS", "id", pkt.Ephem.ReqID)
	// TODO(roosd): check if failed or not
	return util.Forward(pkt)
}

////////////////////////////////////
// Handle Reservation at the start AS
////////////////////////////////////

func (h *EphemHandler) HandleSetupResvReqStartAS(pkt *conf.ExtPkt) error {
	// TODO(roosd): sanity check -> e.g. only requests
	log.Debug("Handling ephemeral setup request on start AS", "id", pkt.Steady.ReqID)
	if err := admitSetupEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleRenewResvReqStartAS(pkt *conf.ExtPkt) error {
	// TODO(roosd): sanity check -> e.g. only requests
	log.Debug("Handling ephemeral renew request on start AS", "id", pkt.Ephem.ReqID)
	if err := admitRenewEphemResv(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleSetupResvRepStartAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on start AS", "id", pkt.Steady.ReqID)
	if !pkt.Steady.Request.GetBase().Accepted {
		if err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady); err != nil {
			log.Error("Unable to clean ephemeral reservation", "er", err)
		}
	}
	return h.sendRepToClient(pkt)
}

func (h *EphemHandler) HandleRenewResvRepStartAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral response on start AS", "id", pkt.Ephem.ReqID)
	// TODO(roosd): check if failed or not
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
	buf := make(common.RawBytes, pkt.Spkt.TotalLen())
	if _, err := hpkt.WriteScnPkt(pkt.Spkt, buf); err != nil {
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
	log.Debug("Handling ephemeral clean up setup", "id", pkt.Steady.ReqID)
	if err := pkt.Conf.SibraAlgo.CleanEphemSetup(pkt.Steady); err != nil {
		return common.NewBasicError("Unable to clean ephemeral reservation", err)
	}
	return util.Forward(pkt)
}

func (h *EphemHandler) HandleCleanRenew(pkt *conf.ExtPkt) error {
	log.Debug("Handling ephemeral clean up renewal", "id", pkt.Ephem.ReqID)
	// TODO(roosd)
	return util.Forward(pkt)
}

func admitSetupEphemResv(pkt *conf.ExtPkt) error {
	res, err := pkt.Conf.SibraAlgo.AdmitEphemSetup(pkt.Steady)
	if err != nil {
		return err
	}
	if pkt.Steady.Request.GetBase().Accepted && res.FailCode == sbreq.FailCodeNone {
		ifids, err := util.GetResvIfids(pkt.Steady.Base, pkt.Spkt)
		if assert.On {
			assert.Must(err == nil, "GetResvIfids must succeed", "err", err)
		}
		if pkt.Steady.IsTransfer() {
			ifids.EgIfid = pkt.Steady.ActiveBlocks[pkt.Steady.CurrSteady+1].SOFields[0].Egress
		}
		if err := issueSOF(pkt.Steady.Base, ifids, pkt.Conf); assert.On && err != nil {
			assert.Must(err == nil, "issueSOF must not fail", "err", err)
		}
	}
	if res.FailCode != sbreq.FailCodeNone {
		failEphemResv(pkt.Steady.Base, res)
	}
	return nil
}

func admitRenewEphemResv(pkt *conf.ExtPkt) error {
	res, err := pkt.Conf.SibraAlgo.AdmitEphemRenew(pkt.Ephem)
	if err != nil {
		return err
	}
	if pkt.Ephem.Request.GetBase().Accepted && res.FailCode == sbreq.FailCodeNone {
		ifids, err := util.GetResvIfids(pkt.Ephem.Base, pkt.Spkt)
		if assert.On {
			assert.Must(err == nil, "GetResvIfids must succeed", "err", err)
		}
		if err := issueSOF(pkt.Ephem.Base, ifids, pkt.Conf); assert.On && err != nil {
			assert.Must(err == nil, "issueSOF must not fail", "err", err)
		}
	}
	if res.FailCode != sbreq.FailCodeNone {
		failEphemResv(pkt.Ephem.Base, res)
	}
	return nil
}

func failEphemResv(base *sbextn.Base, res sbalgo.EphemRes) {
	if base.Request.GetBase().Accepted {
		r := base.Request.(*sbreq.EphemReq)
		base.Request = r.Fail(res.FailCode, res.MaxBw, base.CurrHop)
		// TODO(roosd): remove
		log.Debug("I'm failing the reservation")
	} else {
		r := base.Request.(*sbreq.EphemFailed)
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
	pkt.Spkt.SrcHost = pkt.Conf.PublicAddr.Host
	return nil
}
