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
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/sibra_srv/conf"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/util"
)

type SteadyHandler struct{}

//////////////////////////////////////////
// Handle Reservation at the end AS
/////////////////////////////////////////

func (h *SteadyHandler) HandleResvReqEndAS(pkt *conf.ExtPkt, r *sbreq.SteadyReq) error {
	log.Debug("Handling steady request on end AS", "id", pkt.Steady.ReqID)
	if err := h.sanityCheckReqEndAS(pkt, r); err != nil {
		return err
	}
	if err := AdmitSteadyResv(pkt, r); err != nil {
		return err
	}
	if err := h.reversePkt(pkt); err != nil {
		return err
	}
	if pkt.Steady.Request.GetBase().Accepted {
		if err := PromoteToSOFCreated(pkt); err != nil {
			return err
		}
	}
	return util.Forward(pkt)
}

func (h *SteadyHandler) sanityCheckReqEndAS(pkt *conf.ExtPkt, r *sbreq.SteadyReq) error {
	down := r.Info.PathType.Reversed()
	if !down && (pkt.Steady.SOFIndex+1 != pkt.Steady.PathLens[0]) {
		return common.NewBasicError("Invalid SOFIndex", nil, "expected",
			pkt.Steady.PathLens[0]-1, "actual", pkt.Steady.SOFIndex)
	}
	if down && (pkt.Steady.SOFIndex != 0) {
		return common.NewBasicError("Invalid SOFIndex", nil, "expected",
			0, "actual", pkt.Steady.SOFIndex)
	}
	return nil
}

func (h *SteadyHandler) HandleIdxConfEndAS(pkt *conf.ExtPkt, r *sbreq.ConfirmIndex) error {
	if err := Promote(pkt, r); err != nil {
		return err
	}
	if err := h.reversePkt(pkt); err != nil {
		return err
	}
	return util.Forward(pkt)
}

////////////////////////////////////
// Handle Reservation at the intermediate AS
////////////////////////////////////

func (h *SteadyHandler) HandleResvReqHopAS(pkt *conf.ExtPkt, r *sbreq.SteadyReq) error {
	log.Debug("Handling steady request on hop AS", "id", pkt.Steady.ReqID)
	if err := AdmitSteadyResv(pkt, r); err != nil {
		return err
	}
	return util.Forward(pkt)
}

func (h *SteadyHandler) HandleResvRepHopAS(pkt *conf.ExtPkt) error {
	log.Debug("Handling steady response on hop AS", "id", pkt.Steady.ReqID)
	if pkt.Steady.Request.GetBase().Accepted {
		if err := PromoteToSOFCreated(pkt); err != nil {
			return err
		}
	}
	return util.Forward(pkt)
}

func (h *SteadyHandler) HandleIdxConfHopAS(pkt *conf.ExtPkt, r *sbreq.ConfirmIndex) error {
	if err := Promote(pkt, r); err != nil {
		return err
	}
	return util.Forward(pkt)
}

/////////////////////////////////////////
// General functions
/////////////////////////////////////////

func AdmitSteadyResv(pkt *conf.ExtPkt, r *sbreq.SteadyReq) error {
	ifids, err := util.GetResvIfids(pkt.Steady.Base, pkt.Spkt)
	if err != nil {
		return err
	}
	log.Debug("Admitting steady reservation", "id", pkt.Steady.ReqID, "ifids", ifids)
	params := sbalgo.AdmParams{
		Ifids: ifids,
		Extn:  pkt.Steady,
		Req:   r,
		Src:   pkt.Spkt.SrcIA,
	}
	res, err := pkt.Conf.SibraAlgo.AdmitSteady(params)
	if err != nil {
		return err
	}
	if r.Accepted && !res.Accepted {
		r.Accepted = false
		r.Info.FailHop = pkt.Steady.SOFIndex
		log.Info("Fail reservation", "id", pkt.Steady.ReqID)
	}
	r.Info.BwCls = res.AllocBw
	r.OfferFields[pkt.Steady.SOFIndex].AllocBw = res.AllocBw
	r.OfferFields[pkt.Steady.SOFIndex].MinBw = res.MinBw
	r.OfferFields[pkt.Steady.SOFIndex].MaxBw = res.MaxBw
	r.Lines[pkt.Steady.SOFIndex] = sbresv.SOFieldLines
	return nil
}

func (h *SteadyHandler) reversePkt(pkt *conf.ExtPkt) error {
	// FIXME(roosd): Remove when reversing extensions is supported.
	if _, err := pkt.Steady.Reverse(); err != nil {
		return err
	}
	if err := pkt.Spkt.Reverse(); err != nil {
		return err
	}
	pkt.Spkt.SrcHost = pkt.Conf.PublicAddr.Host
	return nil
}
