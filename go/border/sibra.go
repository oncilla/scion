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
	"math/rand"
	"sync/atomic"

	"github.com/scionproto/scion/go/border/rctx"
	"github.com/scionproto/scion/go/border/rpkt"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/util/bufpool"
)

// SIBRACallback adds SIBRA request to queue of requests which
// are sent to the SIBRA service.
func (r *Router) SIBRACallback(args rpkt.SIBRACallbackArgs) {
	args.RtrPkt.RefInc(1)
	select {
	case r.sibraQ <- args:
	default:
		log.Debug("Dropping SIBRA request packet due to full queue")
		args.RtrPkt.Release()
	}
}

// SIBRAFwd forwards all SIBRA reqeuests to the SIBRA service.
func (r *Router) SIBRAFwd() {
	defer log.LogPanicAndExit()
	// Run forever.
	for args := range r.sibraQ {
		if err := r.fwdExternalSIBRARequest(args.RtrPkt); err != nil {
			log.Error("Unable to forward request to SIBRA service", "err", err)
		}
		args.RtrPkt.Release()
	}
}

func (r *Router) fwdExternalSIBRARequest(rp *rpkt.RtrPkt) error {
	ctx := rctx.Get()
	// Pick first local address from topology as source.
	srcAddr := ctx.Conf.Net.LocAddr.PublicAddrInfo(ctx.Conf.Net.LocAddr.Overlay)
	cpld, err := ctrl.NewSibraMgmtPld(&sibra_mgmt.ExternalPkt{RawPkt: rp.Raw}, nil, nil)
	if err != nil {
		return common.NewBasicError("Unable to create ctrl payload", err)
	}
	scpld, err := cpld.SignedPld(ctrl.NullSigner)
	if err != nil {
		return common.NewBasicError("Unable to create signed ctrl payload", err)
	}
	// Fake RPT header to sibra server which uses rpt as transport.
	buffer, err := r.putRPTHdr(nextPktID.Inc(), scpld)
	if err != nil {
		return common.NewBasicError("Unable to add RPT header", err)
	}
	defer bufpool.Put(buffer)
	if err := r.genPkt(ctx.Conf.IA, addr.SvcSB, 0, srcAddr, buffer.B); err != nil {
		return common.NewBasicError("Unable to generate packet", err)
	}
	return nil
}

var nextPktID = uint56(rand.Int63n(1 << 56))

type uint56 uint64

func (u *uint56) Inc() uint56 {
	for {
		old := atomic.LoadUint64((*uint64)(u))
		new := (old + 1) % (1 << 56)
		swapped := atomic.CompareAndSwapUint64((*uint64)(u), old, new)
		if swapped {
			return uint56(new)
		}
	}
}

func (u uint56) putUint56(b common.RawBytes) {
	common.Order.PutUintN(b, uint64(u), 7)
}

// putHeader returns a new buffer containing the RPT header and packed scpld.
func (r *Router) putRPTHdr(id uint56, scpld *ctrl.SignedPld) (*bufpool.Buffer, error) {
	buffer := bufpool.Get()
	if common.LineLen+scpld.Len() > len(buffer.B) {
		bufpool.Put(buffer)
		return nil, common.NewBasicError("Unable to send, payload too long", nil,
			"pld_len", scpld.Len(), "max_allowed", len(buffer.B)-common.LineLen)
	}
	id.putUint56(buffer.B[1:])
	n, err := scpld.WritePld(buffer.B[common.LineLen:])
	if err != nil {
		bufpool.Put(buffer)
		return nil, common.NewBasicError("Unable to write payload", err)
	}
	buffer.B = buffer.B[:common.LineLen+n]
	return buffer, nil
}
