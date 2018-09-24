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
	"hash"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/sibra_srv/conf"
	"github.com/scionproto/scion/go/sibra_srv/util"
)

func PromoteToSOFCreated(pkt *conf.ExtPkt) error {
	r, ok := pkt.Pld.Data.(*sbreq.SteadySucc)
	if !ok {
		return common.NewBasicError("No steady reservation response block present", nil, "req", r)
	}
	ifids, err := util.GetResvIfids(pkt.Steady.Base, pkt.Spkt)
	if err != nil {
		return err
	}
	err = pkt.Conf.SibraAlgo.PromoteToSOFCreated(ifids, pkt.Steady.GetCurrID(), r.Block.Info)
	if err != nil {
		return common.NewBasicError("Unable to promote to SOF created", err)
	}
	mac := pkt.Conf.SOFMacPool.Get().(hash.Hash)
	defer pkt.Conf.SOFMacPool.Put(mac)
	return r.SetSOF(mac, pkt.Steady.IDs, pkt.Steady.PathLens, ifids.InIfid,
		ifids.EgIfid, int(pkt.Steady.SOFIndex))

}

func Promote(pkt *conf.ExtPkt, r *sbreq.ConfirmIndex) error {
	// FIXME(roosd): Improve error handling. Notify reservation initiator why promotion failed
	switch r.State {
	case sibra.StatePending:
		return PromoteToPending(pkt, r)
	case sibra.StateActive:
		return PromoteToActive(pkt, r)
	default:
		return common.NewBasicError("Invalid state", nil, "state", r.State)
	}
}

func PromoteToPending(pkt *conf.ExtPkt, c *sbreq.ConfirmIndex) error {
	ifids, err := util.GetResvIfids(pkt.Steady.Base, pkt.Spkt)
	if err != nil {
		return err
	}
	return pkt.Conf.SibraAlgo.PromoteToPending(ifids, pkt.Steady.GetCurrID(), c)
}

func PromoteToActive(pkt *conf.ExtPkt, c *sbreq.ConfirmIndex) error {
	if len(pkt.Steady.ActiveBlocks) != 1 {
		return common.NewBasicError("Invalid number of active blocks", nil,
			"expected", 1, "actual", len(pkt.Steady.ActiveBlocks))
	}
	info := pkt.Steady.ActiveBlocks[0].Info
	if c.Idx != info.Index {
		return common.NewBasicError("Invalid index", nil, "expected", info.Index, "actual", c.Idx)
	}
	ifids, err := util.GetResvIfids(pkt.Steady.Base, pkt.Spkt)
	if err != nil {
		return err
	}
	return pkt.Conf.SibraAlgo.PromoteToActive(ifids, pkt.Steady.GetCurrID(), info, c)
}
