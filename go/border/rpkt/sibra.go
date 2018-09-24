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

package rpkt

import (
	"hash"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/scionproto/scion/go/border/metrics"
	"github.com/scionproto/scion/go/border/rcmn"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/overlay"
	"github.com/scionproto/scion/go/lib/ringbuf"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/topology"
)

type SIBRACallbackArgs struct {
	RtrPkt *RtrPkt
}

func (r *RtrPkt) processSibraMgmtSelf(p *sibra_mgmt.Pld) (HookResult, error) {
	u, err := p.Union()
	if err != nil {
		return HookError, err
	}
	switch pld := u.(type) {
	default:
		return HookError, common.NewBasicError("Unsupported destination PathMgmt payload", nil,
			"type", common.TypeOf(pld))
	}
	return HookContinue, nil
}

func (s *rSibraExtn) VerifySOF() (HookResult, error) {
	// validate reservation has not expired
	now := time.Now()
	if !now.Before(s.Info().ExpTick.Time()) {
		return HookError, common.NewBasicError("Reservation expired", nil,
			"now", now, "exp", s.Info().ExpTick.Time(), "fwd", s.Forward)
	}
	// validate hop fields
	pLens := []uint8{s.PathLens[s.CurrSteady]}
	ids := []sibra.ID{s.IDs[s.CurrSteady]}
	if !s.Steady {
		pLens = s.PathLens
		ids = s.IDs
	}
	sofMac := s.rp.Ctx.Conf.SOFMacPool.Get().(hash.Hash)
	err := s.SOF().Verify(sofMac, s.Info(), ids, pLens, s.RawVerifyingSOF())
	s.rp.Ctx.Conf.SOFMacPool.Put(sofMac)
	if err != nil {
		return HookError, err
	}
	return HookContinue, nil
}

func (s *rSibraExtn) RouteSibraRequest() (HookResult, error) {
	if s.rp.DirFrom == rcmn.DirExternal {
		callbacks.sibraF(SIBRACallbackArgs{RtrPkt: s.rp})
		return HookFinish, nil
	}
	// If the packet is steady setup, there is no reservation block present
	// and it uses a regular scion path.
	if s.Setup {
		return HookContinue, nil
	}
	return s.forwardFromLocal()
}

func (s *rSibraExtn) RouteSibraData() (HookResult, error) {
	switch s.rp.DirFrom {
	case rcmn.DirExternal:
		return s.forwardFromExternal()
	case rcmn.DirLocal:
		return s.forwardFromLocal()
	default:
		return HookError, common.NewBasicError("Unsupported forwarding DirFrom", nil,
			"dirFrom", s.rp.DirFrom)
	}
}

func (s *rSibraExtn) forwardFromExternal() (HookResult, error) {
	if s.LastHop() {
		if !s.rp.dstIA.Eq(s.rp.Ctx.Conf.IA) {
			return HookError, common.NewBasicError("Destination ISD-AS does not match",
				nil, "expected", s.rp.dstIA.Eq(s.rp.Ctx.Conf.IA), "actual", s.rp.dstIA)
		}
		if s.rp.CmnHdr.DstType != addr.HostTypeSVC {
			ot := overlay.OverlayFromIP(s.rp.dstHost.IP(), s.rp.Ctx.Conf.Topo.Overlay)
			dst := &topology.AddrInfo{
				Overlay:     ot,
				IP:          s.rp.dstHost.IP(),
				OverlayPort: overlay.EndhostPort,
			}
			s.rp.Egress = append(s.rp.Egress,
				EgressPair{s.rp.Ctx.LocSockOut, dst})
		} else {
			if _, err := s.rp.RouteResolveSVC(); err != nil {
				return HookError, err
			}
		}
	} else {
		if s.IsTransfer() {
			if err := s.incTransfer(); err != nil {
				return HookError, err
			}
		}
		if err := s.rp.validateLocalIF(s.rp.ifNext); err != nil {
			return HookError, err
		}
		nextBR := s.rp.Ctx.Conf.Topo.IFInfoMap[*s.rp.ifNext]
		nextAI := nextBR.InternalAddr.PublicAddrInfo(s.rp.Ctx.Conf.Topo.Overlay)
		ot := overlay.OverlayFromIP(nextAI.IP, s.rp.Ctx.Conf.Topo.Overlay)
		dst := &topology.AddrInfo{
			Overlay:     ot,
			IP:          nextAI.IP,
			L4Port:      nextAI.L4Port,
			OverlayPort: nextAI.L4Port,
		}
		s.rp.Egress = append(s.rp.Egress, EgressPair{s.rp.Ctx.LocSockOut, dst})
	}
	return s.egress()
}

func (s *rSibraExtn) incTransfer() error {
	if err := s.NextSOFIndex(); err != nil {
		return err
	}
	s.rp.ifNext = nil
	if _, err := s.rp.IFNext(); err != nil {
		return err
	}
	_, err := s.VerifySOF()
	return err
}

func (s *rSibraExtn) forwardFromLocal() (HookResult, error) {
	if err := s.NextSOFIndex(); err != nil {
		return HookError, nil
	}
	s.rp.Egress = append(s.rp.Egress, EgressPair{s.rp.Ctx.ExtSockOut[*s.rp.ifCurr], nil})
	return s.egress()
}

func (s *rSibraExtn) egress() (HookResult, error) {
	s.rp.RefInc(len(s.rp.Egress))
	// Call all egress functions.
	for _, epair := range s.rp.Egress {
		epair.S.Ring.Write(ringbuf.EntryList{&EgressRtrPkt{s.rp, epair.Dst}}, true)
		inSock := s.rp.Ingress.Sock
		if inSock == "" {
			inSock = "self"
		}
		metrics.ProcessSockSrcDst.With(
			prometheus.Labels{"inSock": inSock, "outSock": epair.S.Labels["sock"]}).Inc()
	}
	return HookFinish, nil
}
