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

package resvmgr

import (
	"context"
	"strconv"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/l4"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbcreate"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spkt"
)

const (
	ErrorPrepareRequest = "Unable to prepare request"
	ErrorHandleRep      = "Unable to handle reply"
	ErrorSendReq        = "Unable to send request"
)

// repMaster receives all SIBRA replies and forwards them to the
// registered listener.
type repMaster interface {
	Register(key *notifyKey, c chan common.Extension) error
	Deregister(key *notifyKey)
}

type notifyKey struct {
	Id      sibra.ID
	Idx     sibra.Index
	ReqType sbreq.RequestType
}

func (n *notifyKey) String() string {
	return n.Id.String() + strconv.Itoa(int(n.Idx)) + n.ReqType.String()
}

type reqstrI interface {
	PrepareRequest() (common.Extension, error)
	GetExtn() common.Extension
	NotifyKeys() []*notifyKey
	HandleRep(ext common.Extension) (bool, error)
	OnError(err error)
	OnTimeout()
}

var _ reqstrI = (*EphemSetup)(nil)
var _ reqstrI = (*EphemRenew)(nil)
var _ reqstrI = (*EphemCleanSetup)(nil)
var _ reqstrI = (*EphemCleanRenew)(nil)

type reqstr struct {
	log.Logger
	errFunc  func(error, reqstrI)
	timeFunc func(reqstrI)
	succFunc func(reqstrI)
	failFunc func(reqstrI)

	id         sibra.ID
	idx        sibra.Index
	entry      *resvEntry
	repMaster  repMaster
	msgr       *messenger.Messenger
	localSvcSB *snet.Addr
	srcIA      addr.IA
	dstIA      addr.IA
	srcHost    addr.HostAddr
	dstHost    addr.HostAddr
	timeout    time.Duration
	extn       common.Extension
}

func (r *reqstr) Run(i reqstrI) {
	r.Debug("Starting requester")
	var err error
	r.extn, err = i.PrepareRequest()
	if err != nil {
		r.callErr(common.NewBasicError(ErrorPrepareRequest, err), i)
		return
	}
	notify := make(chan common.Extension, 10)
	defer close(notify)
	for _, notifyKey := range i.NotifyKeys() {
		r.repMaster.Register(notifyKey, notify)
		defer r.repMaster.Deregister(notifyKey)
	}
	if err := r.sendRequest(r.extn); err != nil {
		r.callErr(common.NewBasicError(ErrorSendReq, err), i)
		return
	}
	select {
	// FIXME(roosd): handle multiple replies
	case ext := <-notify:
		succ, err := i.HandleRep(ext)
		if err != nil {
			r.callErr(common.NewBasicError(ErrorHandleRep, err), i)
			return
		}
		if succ && r.succFunc != nil {
			r.succFunc(i)
		}
		if !succ && r.failFunc != nil {
			r.failFunc(i)
		}
	case <-time.After(r.timeout):
		r.callTimeOut(i)
	}
}

func (r *reqstr) GetExtn() common.Extension {
	return r.extn
}

func (r *reqstr) callErr(err error, i reqstrI) {
	i.OnError(err)
	if r.errFunc != nil {
		r.errFunc(err, i)
	}
}

func (r reqstr) callTimeOut(i reqstrI) {
	i.OnTimeout()
	if r.timeFunc != nil {
		r.timeFunc(i)
	}
}

func (r *reqstr) sendRequest(ext common.Extension) error {
	pkt := &spkt.ScnPkt{
		DstIA:   r.dstIA,
		SrcIA:   r.srcIA,
		DstHost: r.dstHost,
		SrcHost: r.srcHost,
		HBHExt:  []common.Extension{ext},
		L4:      l4.L4Header(&l4.UDP{Checksum: make(common.RawBytes, 2)}),
		Pld:     make(common.RawBytes, 0),
	}
	buf := make(common.RawBytes, pkt.TotalLen())
	_, err := hpkt.WriteScnPkt(pkt, buf)
	if err != nil {
		return err
	}
	ctx, cancelF := context.WithTimeout(context.Background(), r.timeout)
	defer cancelF()
	err = r.msgr.SendSibraEphemReq(ctx, &sibra_mgmt.EphemReq{&sibra_mgmt.ExternalPkt{RawPkt: buf}},
		r.localSvcSB, requestID.Next())
	return nil
}

type reserver struct {
	*reqstr
	bwCls sibra.BwCls
}

func (r *reserver) OnError(err error) {
	r.Info("Reservation request failed", "id", r.id, "idx", 0, "err", err)
}

func (r *reserver) OnTimeout() {
	r.Info("Reservation request timed out", "id", r.id, "idx", 0)
}

type EphemSetup struct {
	*reserver
}

func (s *EphemSetup) PrepareRequest() (common.Extension, error) {
	s.entry.Lock()
	defer s.entry.Unlock()
	steady := s.entry.syncResv.Load().Steady
	if steady == nil {
		return nil, common.NewBasicError("Steady extension not available", nil)
	}
	steady = steady.Copy().(*sbextn.Steady)

	info := &sbresv.Info{
		ExpTick:  sibra.CurrentTick() + sibra.MaxEphemTicks,
		BwCls:    s.bwCls,
		PathType: sibra.PathTypeEphemeral,
		RttCls:   10, // FIXME(roosd): add RTT classes based on steady
	}
	ephemReq := sbreq.NewEphemReq(sbreq.REphmSetup, s.id, info, steady.TotalHops)
	if err := steady.ToRequest(ephemReq); err != nil {
		return nil, err
	}
	return steady, nil
}

func (s *EphemSetup) NotifyKeys() []*notifyKey {
	return []*notifyKey{{Id: s.id, Idx: s.idx, ReqType: sbreq.REphmSetup}}
}

func (s *EphemSetup) HandleRep(ext common.Extension) (bool, error) {
	// TODO(roosd): sanity check -> correct ID, correct Idx, correct parameters ...
	// correct response
	if _, ok := ext.(*sbextn.Steady); !ok {
		return false, common.NewBasicError("Extension is not steady", nil)
	}
	steady := ext.(*sbextn.Steady)
	s.entry.Lock()
	defer s.entry.Unlock()
	switch r := ext.(*sbextn.Steady).Request.(type) {
	case *sbreq.EphemReq:
		ids := []sibra.ID{r.ReqID}
		ids = append(ids, steady.IDs...)
		ephem, err := sbcreate.NewEphemUse(ids, steady.PathLens, r.Block, true)
		if err != nil {
			return false, err
		}
		s.entry.syncResv.UpdateEphem(ephem)
		s.entry.ephemMeta.timestamp = time.Now()
		s.entry.ephemMeta.state = ephemExists
	case *sbreq.EphemFailed:
		// FIXME(roosd): Determine error based on fail code
		s.entry.ephemMeta.lastFailCode = r.FailCode
		s.entry.ephemMeta.lastMaxBW = r.Info.BwCls
		s.entry.ephemMeta.timestamp = time.Now()
		return false, nil
	}
	return true, nil
}

type EphemRenew struct {
	*reserver
}

func (r *EphemRenew) PrepareRequest() (common.Extension, error) {
	r.entry.Lock()
	defer r.entry.Unlock()
	ephem := r.entry.syncResv.Load().Ephemeral
	if ephem == nil {
		return nil, common.NewBasicError("Ephemeral extension not available", nil)
	}
	ephem = ephem.Copy().(*sbextn.Ephemeral)
	if ephem.ActiveBlocks[0].Info.Index.Add(1) != r.idx {
		return nil, common.NewBasicError("Indexes out of sync", nil, "existing",
			ephem.ActiveBlocks[0].Info.Index, "next", r.idx)
	}
	r.id = ephem.IDs[0]
	info := &sbresv.Info{
		ExpTick:  sibra.CurrentTick() + sibra.MaxEphemTicks,
		BwCls:    r.bwCls,
		PathType: sibra.PathTypeEphemeral,
		RttCls:   10, // FIXME(roosd): add RTT classes based on steady
		Index:    r.idx,
	}
	ephemReq := sbreq.NewEphemReq(sbreq.REphmRenewal, nil, info, ephem.TotalHops)
	if err := ephem.ToRequest(ephemReq); err != nil {
		return nil, err
	}
	return ephem, nil
}

func (r *EphemRenew) NotifyKeys() []*notifyKey {
	return []*notifyKey{{Id: r.id, Idx: r.idx, ReqType: sbreq.REphmRenewal}}
}

func (r *EphemRenew) HandleRep(ext common.Extension) (bool, error) {
	// TODO(roosd): sanity check -> correct ID, correct Idx, correct parameters ...
	// correct response
	if _, ok := ext.(*sbextn.Ephemeral); !ok {
		return false, common.NewBasicError("Extension is not ephemeral", nil)
	}
	ephem := ext.(*sbextn.Ephemeral)
	r.entry.Lock()
	defer r.entry.Unlock()
	switch request := ext.(*sbextn.Ephemeral).Request.(type) {
	case *sbreq.EphemReq:
		ephem, err := sbcreate.NewEphemUse(ephem.IDs, ephem.PathLens, request.Block, true)
		if err != nil {
			return false, err
		}
		r.entry.syncResv.UpdateEphem(ephem)
		r.entry.ephemMeta.timestamp = time.Now()
		r.entry.ephemMeta.state = ephemExists
	case *sbreq.EphemFailed:
		// FIXME(roosd): Determine error based on fail code
		r.entry.ephemMeta.lastFailCode = request.FailCode
		r.entry.ephemMeta.lastMaxBW = request.Info.BwCls
		r.entry.ephemMeta.timestamp = time.Now()
		return false, nil
	}
	return true, nil
}

type cleaner struct {
	*reqstr
	FailedInfo *sbresv.Info
}

func (c *cleaner) OnError(err error) {
	c.Info("Reservation cleanup failed", "id", c.id, "idx", 0, "err", err)
}

func (c *cleaner) OnTimeout() {
	c.Info("Reservation cleanup timed out", "id", c.id, "idx", 0)
}

func (c *cleaner) NotifyKeys() []*notifyKey {
	return []*notifyKey{{Id: c.id, Idx: c.idx, ReqType: sbreq.REphmCleanUp}}
}

type EphemCleanSetup struct {
	*cleaner
}

func (c *EphemCleanSetup) PrepareRequest() (common.Extension, error) {
	c.entry.Lock()
	defer c.entry.Unlock()
	steady := c.entry.syncResv.Load().Steady
	if steady == nil {
		return nil, common.NewBasicError("Steady extension not available", nil)
	}
	steady = steady.Copy().(*sbextn.Steady)
	r := sbreq.NewEphemClean(c.id, c.FailedInfo, steady.TotalHops)
	if err := steady.ToRequest(r); err != nil {
		return nil, err
	}
	return steady, nil
}

func (c *EphemCleanSetup) HandleRep(ext common.Extension) (bool, error) {
	// TODO(roosd): sanity check -> correct ID, correct Idx, correct parameters ...
	// correct response
	steady, ok := ext.(*sbextn.Steady)
	if !ok {
		return false, common.NewBasicError("Extension is not steady", nil)
	}
	// TODO(roosd): remove
	c.Info("Got cleaner reply", "rep", steady.Request)
	switch r := ext.(*sbextn.Steady).Request.(type) {
	case *sbreq.EphemClean:
		return r.Accepted, nil
	default:
		return false, common.NewBasicError("Invalid response type", nil, "type", r.GetBase().Type)
	}
}

type EphemCleanRenew struct {
	*cleaner
}

func (c *EphemCleanRenew) PrepareRequest() (common.Extension, error) {
	c.entry.Lock()
	defer c.entry.Unlock()
	ephem := c.entry.syncResv.Load().Ephemeral
	if ephem == nil {
		return nil, common.NewBasicError("Ephemeral extension not available", nil)
	}
	ephem = ephem.Copy().(*sbextn.Ephemeral)
	r := sbreq.NewEphemClean(nil, c.FailedInfo, ephem.TotalHops)
	if err := ephem.ToRequest(r); err != nil {
		return nil, err
	}
	return ephem, nil
}

func (c *EphemCleanRenew) HandleRep(ext common.Extension) (bool, error) {
	// TODO(roosd): sanity check -> correct ID, correct Idx, correct parameters ...
	// correct response
	ephem, ok := ext.(*sbextn.Ephemeral)
	if !ok {
		return false, common.NewBasicError("Extension is not ephemeral", nil)
	}
	// TODO(roosd): remove
	c.Info("Got cleaner reply", "rep", ephem.Request)
	switch r := ext.(*sbextn.Steady).Request.(type) {
	case *sbreq.EphemClean:
		return r.Accepted, nil
	default:
		return false, common.NewBasicError("Invalid response type", nil, "type", r.GetBase().Type)
	}
}
