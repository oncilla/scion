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

package main

import (
	"hash"

	"time"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/infra"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/sibra_srv/adm"
	"github.com/scionproto/scion/go/sibra_srv/conf"
	"github.com/scionproto/scion/go/sibra_srv/util"
)

var _ infra.Handler = (*ExternalHandler)(nil)

type ExternalHandler struct {
	steadyHandler *adm.SteadyHandler
	ephemHandler  *adm.EphemHandler
}

func (h *ExternalHandler) Handle(r *infra.Request) {
	if err := h.handle(r); err != nil {
		saddr := r.Peer.(*snet.Addr)
		log.Error("Unable to handle external packet", "err", err, "peer", saddr)
	}
}

func (h *ExternalHandler) handle(r *infra.Request) error {
	var rawPkt common.RawBytes
	switch ppld := r.Message.(type) {
	case *sibra_mgmt.ExternalPkt:
		rawPkt = ppld.RawPkt
	case *sibra_mgmt.EphemReq:
		rawPkt = ppld.RawPkt
	case *sibra_mgmt.EphemRep:
		rawPkt = ppld.RawPkt
	default:
		return common.NewBasicError("Unsupported msg type", nil, "request", ppld)
	}
	pkt := &conf.ExtPkt{
		Req:  r,
		Addr: r.Peer.(*snet.Addr),
		Spkt: &spkt.ScnPkt{},
		Conf: conf.Get(),
	}
	if err := hpkt.ParseScnPkt(pkt.Spkt, rawPkt); err != nil {
		return common.NewBasicError("Unable to parse SIBRA packet", err)
	}
	sExtn, err := util.GetSibraExtn(pkt.Spkt)
	if err != nil {
		return common.NewBasicError("Unable to get SIBRA extension", err)
	}
	var base *sbextn.Base
	switch e := sExtn.(type) {
	case *sbextn.Steady:
		pkt.Steady = e
		base = e.Base
	case *sbextn.Ephemeral:
		pkt.Ephem = e
		base = e.Base
	default:
		return common.NewBasicError("invalid type", nil, "type", e.Type())
	}
	if err := h.validate(base, pkt); err != nil {
		return common.NewBasicError("Unable to validate", err)
	}
	if base.IsRequest {
		pkt.Pld, err = sbreq.PldFromRaw(pkt.Spkt.Pld.(common.RawBytes))
		if err != nil {
			return common.NewBasicError("Unable to parse request payload", err)
		}
		return h.handleRequest(pkt)
	}
	return common.NewBasicError("Dropping non-request packet", nil)

}

func (h *ExternalHandler) validate(base *sbextn.Base, pkt *conf.ExtPkt) error {
	ifids, err := util.GetResvIfids(base, pkt.Spkt)
	if err != nil {
		return common.NewBasicError("Unable to get resv ifids", err)
	}
	if _, ok := pkt.Conf.Topo.IFInfoMap[ifids.InIfid]; !ok && ifids.InIfid != 0 {
		return common.NewBasicError("Ingress interface does not exist", err)
	}
	if _, ok := pkt.Conf.Topo.IFInfoMap[ifids.EgIfid]; !ok && ifids.EgIfid != 0 {
		return common.NewBasicError("Egress interface does not exist", err)
	}
	if !base.Setup && pkt.Spkt.Path != nil {
		return common.NewBasicError("Non setup packet must not contain path", err)
	}
	if !base.Setup {
		return h.validateSOF(base, pkt)
	}
	return nil
}

func (h *ExternalHandler) validateSOF(base *sbextn.Base, pkt *conf.ExtPkt) error {
	mac := pkt.Conf.SOFMacPool.Get().(hash.Hash)
	err := base.VerifySOF(mac, time.Now())
	pkt.Conf.SOFMacPool.Put(mac)
	return err
}

func (h *ExternalHandler) handleRequest(pkt *conf.ExtPkt) error {
	switch r := pkt.Pld.Data.(type) {
	case *sbreq.SteadyReq:
		if pkt.Pld.Response {
			return h.handleSteadyRep(pkt)
		}
		return h.handleSteadyReq(pkt, r)
	case *sbreq.SteadySucc:
		return h.handleSteadyRep(pkt)
	case *sbreq.ConfirmIndex:
		return h.handleIdxConf(pkt, r)
	case *sbreq.EphemReq:
		if pkt.Pld.Response {
			return h.handleEphemResvRep(pkt)
		}
		return h.handleEphemResvReq(pkt)
	case *sbreq.EphemFailed:
		if pkt.Pld.Response {
			return h.handleEphemResvRep(pkt)
		}
		return h.handleEphemResvReq(pkt)
	case *sbreq.EphemClean:
		if pkt.Pld.Response {
			return h.handleEphemCleanUpRep(pkt)
		}
		return h.handleEphemCleanUpReq(pkt)
	default:
		return common.NewBasicError("Unsupported request type", nil, "req", r)
	}
}

func (h *ExternalHandler) handleSteadyReq(pkt *conf.ExtPkt, r *sbreq.SteadyReq) error {
	switch {
	case pkt.Steady.LastHop() && !pkt.Pld.Response:
		return h.steadyHandler.HandleResvReqEndAS(pkt, r)
	default:
		return h.steadyHandler.HandleResvReqHopAS(pkt, r)
	}
}

func (h *ExternalHandler) handleSteadyRep(pkt *conf.ExtPkt) error {
	switch {
	case pkt.Steady.LastHop():
		return pkt.Conf.RepMaster.Handle(pkt)
	default:
		return h.steadyHandler.HandleResvRepHopAS(pkt)
	}
}

func (h *ExternalHandler) handleIdxConf(pkt *conf.ExtPkt, r *sbreq.ConfirmIndex) error {
	switch {
	case !pkt.Pld.Response && pkt.Steady.LastHop():
		return h.steadyHandler.HandleIdxConfEndAS(pkt, r)
	case !pkt.Pld.Response:
		return h.steadyHandler.HandleIdxConfHopAS(pkt, r)
	case pkt.Pld.Response && pkt.Steady.LastHop():
		return pkt.Conf.RepMaster.Handle(pkt)
	default:
		return util.Forward(pkt)
	}
}

func (h *ExternalHandler) handleEphemResvReq(pkt *conf.ExtPkt) error {
	if pkt.Steady != nil {
		if pkt.Pld.Type != sbreq.REphmSetup {
			return common.NewBasicError("Invalid request type", nil, "steady", true,
				"rt", pkt.Pld.Type)
		}
		switch {
		case pkt.Steady.FirstHop():
			return h.ephemHandler.HandleSetupResvReqStartAS(pkt)
		case pkt.Steady.IsTransfer():
			return h.ephemHandler.HandleSetupResvReqTransAS(pkt)
		case pkt.Steady.LastHop():
			return h.ephemHandler.HandleSetupResvReqEndAS(pkt)
		default:
			return h.ephemHandler.HandleSetupResvReqHopAS(pkt)
		}
	}
	if pkt.Pld.Type != sbreq.REphmRenewal {
		return common.NewBasicError("Invalid request type", nil, "steady", false,
			"rt", pkt.Pld.Type)
	}
	switch {
	case pkt.Ephem.FirstHop():
		return h.ephemHandler.HandleRenewResvReqStartAS(pkt)
	case pkt.Ephem.IsSteadyTransfer():
		return h.ephemHandler.HandleRenewResvReqTransAS(pkt)
	case pkt.Ephem.LastHop():
		return h.ephemHandler.HandleRenewResvReqEndAS(pkt)
	default:
		return h.ephemHandler.HandleRenewResvReqHopAS(pkt)
	}
}

func (h *ExternalHandler) handleEphemResvRep(pkt *conf.ExtPkt) error {
	if pkt.Steady != nil {
		if pkt.Pld.Type != sbreq.REphmSetup {
			return common.NewBasicError("Invalid request type", nil, "steady", true,
				"rt", pkt.Pld.Type)
		}
		switch {
		case pkt.Steady.FirstHop():
			return h.ephemHandler.HandleSetupResvRepEndAS(pkt)
		case pkt.Steady.IsTransfer():
			return h.ephemHandler.HandleSetupResvRepTransAS(pkt)
		case pkt.Steady.LastHop():
			return h.ephemHandler.HandleSetupResvRepStartAS(pkt)
		default:
			return h.ephemHandler.HandleSetupResvRepHopAS(pkt)
		}
	}
	if pkt.Pld.Type != sbreq.REphmRenewal {
		return common.NewBasicError("Invalid request type", nil, "steady", false,
			"rt", pkt.Pld.Type)
	}
	switch {
	case pkt.Ephem.FirstHop():
		return h.ephemHandler.HandleRenewResvRepEndAS(pkt)
	case pkt.Ephem.IsSteadyTransfer():
		return h.ephemHandler.HandleRenewResvRepTransAS(pkt)
	case pkt.Ephem.LastHop():
		return h.ephemHandler.HandleRenewResvRepStartAS(pkt)
	default:
		return h.ephemHandler.HandleRenewResvRepHopAS(pkt)
	}
}

func (h *ExternalHandler) handleEphemCleanUpReq(pkt *conf.ExtPkt) error {
	if pkt.Steady != nil {
		if pkt.Pld.Type != sbreq.REphmCleanUp {
			return common.NewBasicError("Invalid request type", nil, "steady", true,
				"rt", pkt.Pld.Type)
		}
		if pkt.Steady.LastHop() {
			return h.ephemHandler.HandleCleanSetupEndAS(pkt)
		} else {
			return h.ephemHandler.HandleCleanSetup(pkt)
		}
	}
	if pkt.Pld.Type != sbreq.REphmCleanUp {
		return common.NewBasicError("Invalid request type", nil, "steady", false,
			"rt", pkt.Pld.Type)
	}
	if pkt.Ephem.LastHop() {
		return h.ephemHandler.HandleCleanRenewEndAS(pkt)
	} else {
		return h.ephemHandler.HandleCleanRenew(pkt)
	}
}

func (h *ExternalHandler) handleEphemCleanUpRep(pkt *conf.ExtPkt) error {
	if pkt.Steady != nil {
		if pkt.Pld.Type != sbreq.REphmCleanUp {
			return common.NewBasicError("Invalid request type", nil, "steady", true,
				"rt", pkt.Pld.Type)
		}
		if pkt.Steady.LastHop() {
			return h.ephemHandler.HandleCleanStartAS(pkt)
		} else {
			return util.Forward(pkt)
		}
	}
	if pkt.Pld.Type != sbreq.REphmCleanUp {
		return common.NewBasicError("Invalid request type", nil, "steady", false,
			"rt", pkt.Pld.Type)
	}
	if pkt.Ephem.LastHop() {
		return h.ephemHandler.HandleCleanStartAS(pkt)
	} else {
		return util.Forward(pkt)
	}
}
