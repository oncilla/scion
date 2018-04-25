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

	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/infra"
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/snet"
)

var RequestID messenger.Counter

var _ infra.Handler = (*SteadyRegHandler)(nil)

type SteadyRegHandler struct{}

func (h *SteadyRegHandler) Handle(r *infra.Request) {
	saddr := r.Peer.(*snet.Addr)
	pld := r.Message.(*sibra_mgmt.SteadyReg)
	// FIXME(roosd): Support registering of reservations.
	log.Error("Registering reservations not supported yet", "addr", saddr, "req", pld)
	return
}

func (h *SteadyRegHandler) logDropReq(addr net.Addr, req *sibra_mgmt.SteadyReg, err error) {
	log.Error("[SteadyRegHandler] Dropping request", "addr", addr, "req", req, "err", err)
}
