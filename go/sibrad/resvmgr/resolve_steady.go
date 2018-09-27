// Copyright 2017 ETH Zurich
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

package resvmgr

import (
	"context"
	"fmt"
	"time"

	"bytes"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbcreate"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

// resolveSteady assumes entry is locked.
func (r *resolver) resolveSteady(entry *resvEntry, path *spathmeta.AppPath) (bool, error) {
	ctx, cancelF := context.WithTimeout(context.Background(), steadyReqTimeout)
	defer cancelF()
	bmetas, err := r.fetchSteadyResv(ctx, path)
	if err != nil {
		return false, err
	}
	steadyExtn, err := sbcreate.NewSteadyBE(bmetas, true)
	if err != nil {
		return false, err
	}
	for _, bmeta := range bmetas {
		smeta := r.store.getSteadyMeta(bmeta.Id)
		if smeta == nil {
			smeta = &steadyMeta{
				Meta:      bmeta,
				timestamp: time.Now(),
				ResvKeys:  make(map[ResvKey]struct{}),
			}
			r.store.addSteadyMeta(smeta)
		}
		smeta.Lock()
		smeta.Meta = bmeta
		smeta.timestamp = time.Now()
		smeta.ResvKeys[r.key] = struct{}{}
		smeta.Unlock()
	}
	entry.syncResv.UpdateSteady(steadyExtn)
	if entry.ephemMeta == nil {
		r.events <- &Event{
			Code: ExtnUpdated,
		}
	}
	return false, nil
}

func (r *resolver) fetchSteadyResv(ctx context.Context,
	path *spathmeta.AppPath) ([]*sibra_mgmt.BlockMeta, error) {

	reqs, err := r.getSteadyReqs(path)
	if err != nil {
		return nil, common.NewBasicError("Unable to get requests", err)
	}

	errC := make(chan error, len(reqs))
	defer close(errC)
	bmetas := make([]*sibra_mgmt.BlockMeta, len(reqs))
	now := time.Now()
	for i := 0; i < len(reqs); i++ {
		steadyIds := r.store.getSteadyId(reqs[i].SegID)
		for _, steadyId := range steadyIds {
			if meta := r.store.getSteadyMeta(steadyId); meta != nil {
				meta.Lock()
				// TODO(roosd): clean up store.
				if meta.Meta.Expiry().Before(now) &&
					meta.timestamp.Add(5*time.Second).Before(time.Now()) &&
					bytes.Equal(meta.Meta.SegID(), reqs[i].SegID) {
					bmetas[i] = meta.Meta
				}
				meta.Unlock()
			}

		}
		if bmetas[i] != nil {
			continue
		}
		saddr, err := r.getAddrForSteadyReq(reqs[i])
		if err != nil {
			return nil, common.NewBasicError("Unable to get snet addrs", err)
		}
		reqCopy := reqs[i]
		iCopy := i
		go func() {
			rep, err := r.msgr.GetSibraSteady(ctx, reqCopy, saddr, requestID.Next())
			if err != nil {
				errC <- err
				return
			}
			if len(rep.Recs.Entries) < 1 {
				errC <- common.NewBasicError("No reservation found", nil)
				return
			}
			bmetas[iCopy] = rep.Recs.Entries[0]
			errC <- nil
		}()
	}
	errs := make([]error, 0, len(reqs))
	for i := 0; i < len(reqs); i++ {
		if err := <-errC; err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return nil, common.NewBasicError("Unable to get steady reservations", nil, "errs", errs)
	}
	return bmetas, nil

}

func (r *resolver) getSteadyReqs(path *spathmeta.AppPath) ([]*sibra_mgmt.SteadyReq, error) {
	sp := spath.New(path.Entry.Path.FwdPath)
	infos := make([]*spath.InfoField, 0, 3)
	pathItfsList := make([]*[]sibra_mgmt.PathInterface, 0, 3)
	var pathItfs *[]sibra_mgmt.PathInterface
	off := -1
	if err := sp.IncOffsets(); err != nil {
		return nil, err
	}
	for err := error(nil); err == nil; err = sp.IncOffsets() {
		hopf, _ := sp.GetHopField(sp.HopOff)
		infof, _ := sp.GetInfoField(sp.InfOff)
		if off != sp.InfOff {
			off = sp.InfOff
			infos = append(infos, infof)
			ifs := make([]sibra_mgmt.PathInterface, 0)
			pathItfs = &ifs
			pathItfsList = append(pathItfsList, pathItfs)
		}
		in := sibra_mgmt.PathInterface{
			IfID: hopf.ConsIngress,
		}
		eg := sibra_mgmt.PathInterface{
			IfID: hopf.ConsEgress,
		}
		if infof.ConsDir {
			*pathItfs = append(*pathItfs, in, eg)
		} else {
			*pathItfs = append(*pathItfs, eg, in)
		}
	}
	reqs := make([]*sibra_mgmt.SteadyReq, len(pathItfsList))
	start, end := 0, 0
	for i, pIfs := range pathItfsList {
		ifs := (*pIfs)[1 : len(*pIfs)-1]
		start, end = end, end+len(ifs)
		if infos[i].Peer {
			// FIXME(roosd): Peering paths are not supported currently.
			return nil, common.NewBasicError("Peering links not supported yet", nil)
		}
		pt := sibra.PathTypeUp
		if infos[i].ConsDir {
			pt = sibra.PathTypeDown
		} else if start != 0 {
			pt = sibra.PathTypeCore
		}
		for k := range ifs {
			ifs[k].RawIsdas = path.Entry.Path.Interfaces[start+k].RawIsdas
		}
		reqs[i] = &sibra_mgmt.SteadyReq{
			PathType:   pt,
			RawStartIA: ifs[0].RawIsdas,
			RawEndIA:   ifs[len(ifs)-1].RawIsdas,
			SegID:      sibra_mgmt.PathToSegID(ifs),
		}
	}
	iflists := make([]string, 0)
	for i := range reqs {
		iflists = append(iflists, fmt.Sprintf("Ifs%d %s", i, *pathItfsList[i]))
	}
	r.Debug("Resolving steady reservations")
	return reqs, nil
}

func (r *resolver) getAddrForSteadyReq(req *sibra_mgmt.SteadyReq) (*snet.Addr, error) {
	ia := req.StartIA()
	if req.PathType.Reversed() {
		ia = req.EndIA()
	}
	saddr := &snet.Addr{
		IA:   ia,
		Host: addr.SvcSB,
	}
	if ia == snet.DefNetwork.IA() {
		return r.getLocalSvcSB()
	}
	return saddr, nil
}
