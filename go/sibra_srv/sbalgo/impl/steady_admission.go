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

package impl

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/proto"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
)

func admitSteady(s sbalgo.Algo, p sbalgo.AdmParams, topo *topology.Topo) (sbalgo.SteadyRes, error) {
	if err := validate(p, topo); err != nil {
		return sbalgo.SteadyRes{}, common.NewBasicError("Unable to validate", err)
	}
	s.Lock()
	defer s.Unlock()

	// Available already makes sure that mBw cannot be larger
	// than capacity of the in or out link.
	avail := s.Available(p.Ifids, p.Extn.GetCurrID())
	ideal := s.Ideal(p)
	// TODO(roosd): remove
	doLog := false
	if doLog {
		logInfo("Sibra computation", p, avail, ideal, s)
	}
	if avail > ideal {
		avail = ideal
	}
	res := sbalgo.SteadyRes{
		MaxBw: avail.ToBwCls(true),
	}
	if res.MaxBw < p.Req.MinBw || !p.Accepted {
		return res, nil
	}
	res.AllocBw = minBwCls(res.MaxBw, p.Req.AccBw)
	if err := s.AddSteadyResv(p, res.AllocBw); err != nil {
		return sbalgo.SteadyRes{}, err
	}
	res.Accepted = true
	// TODO(roosd): remove
	if doLog {
		logInfo("Post adding", p, avail, ideal, s)
	}
	return res, nil
}

func logInfo(m string, p sbalgo.AdmParams, avail, ideal sibra.Bps, s sbalgo.Algo) {
	log.Info(m, "id", p.Extn.GetCurrID(), "\navail", avail, "ideal", ideal,
		"req", p.Req.MaxBw.Bps(), "ifids", p.Ifids, "\nState", s)

}

func validate(params sbalgo.AdmParams, topo *topology.Topo) error {
	if err := validateIfids(params, topo); err != nil {
		return err
	}
	if err := validateReq(params, topo); err != nil {
		return err
	}
	return nil
}

func validateIfids(params sbalgo.AdmParams, topo *topology.Topo) error {
	in, err := getLinkType(params.Ifids.InIfid, topo)
	if err != nil {
		return common.NewBasicError("Unable to find ingress ifid", err)
	}
	eg, err := getLinkType(params.Ifids.EgIfid, topo)
	if err != nil {
		return common.NewBasicError("Unable to find egress ifid", err)
	}
	if ok := params.Req.Info.PathType.ValidIFPair(in, eg); !ok {
		return common.NewBasicError("Invalid link pair", nil, "path",
			params.Req.Info.PathType, "ingress", in, "egress", eg)
	}
	return nil
}

func validateReq(params sbalgo.AdmParams, topo *topology.Topo) error {
	if params.Extn.Setup && params.Req.Info.Index != 0 {
		return common.NewBasicError("Invalid initial index", nil, "idx", params.Req.Info.Index)
	}
	if !params.Extn.Setup && params.Extn.GetCurrBlock().Info.PathType != params.Req.Info.PathType {
		return common.NewBasicError("Pathtype must not change", nil, "expected",
			params.Extn.GetCurrBlock().Info.PathType, "actual", params.Req.Info.PathType)

	}
	if params.Req.MaxBw == 0 {
		return common.NewBasicError("Maximum bandwidth class must not be zero", nil)
	}
	return nil
}

func getLinkType(ifid common.IFIDType, topo *topology.Topo) (proto.LinkType, error) {
	inInfo, ok := topo.IFInfoMap[ifid]
	switch {
	case ifid == 0:
		return proto.LinkType_unset, nil
	case ok:
		return inInfo.LinkType, nil
	}
	return proto.LinkType_unset, common.NewBasicError("Interface not found", nil, "ifid", ifid)
}

func minBwCls(a, b sibra.BwCls) sibra.BwCls {
	if a < b {
		return a
	}
	return b
}
