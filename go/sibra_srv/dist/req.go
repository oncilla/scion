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

package dist

import (
	"net"

	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/infra"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra/resvdb/query"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/sibra_srv/conf"
)

var _ infra.Handler = (*SteadyReqHandler)(nil)

type SteadyReqHandler struct{}

func (h *SteadyReqHandler) Handle(r *infra.Request) {
	saddr := r.Peer.(*snet.Addr)
	pld := r.Message.(*sibra_mgmt.SteadyReq)
	config := conf.Get()
	log.Debug("[SteadyReqHandler] Received request", "addr", saddr, "req", pld)

	p := &query.Params{
		StartsAt: []addr.IA{pld.StartIA()},
		EndsAt:   []addr.IA{pld.EndIA()},
		SegID:    pld.SegID,
	}
	results, err := config.ResvDB.Get(p)
	if err != nil {
		h.logDropReq(saddr, pld, err)
		return
	}
	k := 5
	blocks := make([]*sibra_mgmt.BlockMeta, 0, 5)
	for i := 0; len(blocks) < k && i < len(results); i++ {
		if results[i].BlockMeta.Expiry().After(time.Now()) {
			blocks = append(blocks, results[i].BlockMeta)
		}
	}

	rep := &sibra_mgmt.SteadyRep{
		Req: pld,
		Recs: &sibra_mgmt.SteadyRecs{
			Entries: blocks,
		},
	}

	msgr, ok := infra.MessengerFromContext(r.Context())
	if !ok {
		h.logDropReq(saddr, pld, common.NewBasicError("No messenger found", nil))
		return
	}
	if err = msgr.SendSibraSteady(r.Context(), rep, saddr, r.ID); err != nil {
		h.logDropReq(saddr, pld, common.NewBasicError("Unable to reply", err))
	}
}

func (h *SteadyReqHandler) logDropReq(addr net.Addr, req *sibra_mgmt.SteadyReq, err error) {
	log.Error("[SteadyReqHandler] Dropping request", "addr", addr, "req", req, "err", err)
}
