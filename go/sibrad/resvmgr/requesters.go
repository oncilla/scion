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
	Register(key *notifyKey, c chan notifyEvent) error
	Deregister(key *notifyKey)
}

type notifyEvent struct {
	extn common.Extension
	pld  *sbreq.Pld
}

type notifyKey struct {
	Id      sibra.ID
	Idx     sibra.Index
	ReqType sbreq.DataType
}

func (n *notifyKey) String() string {
	return n.Id.String() + strconv.Itoa(int(n.Idx)) + n.ReqType.String()
}

type reqstrI interface {
	PrepareRequest() (common.Extension, *sbreq.Pld, error)
	GetExtn() common.Extension
	GetPld() *sbreq.Pld
	NotifyKeys() []*notifyKey
	HandleRep(event notifyEvent) (bool, error)
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
	pld        *sbreq.Pld
}

func (r *reqstr) Run(i reqstrI) {
	r.Debug("Starting requester")
	var err error
	r.extn, r.pld, err = i.PrepareRequest()
	if err != nil {
		r.callErr(common.NewBasicError(ErrorPrepareRequest, err), i)
		return
	}
	notify := make(chan notifyEvent, 10)
	defer close(notify)
	for _, notifyKey := range i.NotifyKeys() {
		r.repMaster.Register(notifyKey, notify)
		defer r.repMaster.Deregister(notifyKey)
	}
	if err := r.sendRequest(); err != nil {
		r.callErr(common.NewBasicError(ErrorSendReq, err), i)
		return
	}
	select {
	case event := <-notify:
		succ, err := i.HandleRep(event)
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

func (r *reqstr) GetPld() *sbreq.Pld {
	return r.pld
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

func (r *reqstr) sendRequest() error {
	pkt := &spkt.ScnPkt{
		DstIA:   r.dstIA,
		SrcIA:   r.srcIA,
		DstHost: r.dstHost,
		SrcHost: r.srcHost,
		HBHExt:  []common.Extension{r.extn},
		L4:      l4.L4Header(&l4.UDP{Checksum: make(common.RawBytes, 2)}),
		Pld:     r.pld,
	}
	buf := make(common.RawBytes, pkt.TotalLen())
	_, err := hpkt.WriteScnPkt(pkt, buf)
	if err != nil {
		return err
	}
	ctx, cancelF := context.WithTimeout(context.Background(), r.timeout)
	defer cancelF()
	req := &sibra_mgmt.EphemReq{
		ExternalPkt: &sibra_mgmt.ExternalPkt{
			RawPkt: buf,
		},
	}
	err = r.msgr.SendSibraEphemReq(ctx, req, r.localSvcSB, requestID.Next())
	return nil
}

type reserver struct {
	*reqstr
	bwCls sibra.BwCls
}

func (r *reserver) validate(id sibra.ID, pld *sbreq.Pld) error {
	if pld.Data == nil {
		return common.NewBasicError("No data present", nil, "pld", pld)
	}
	var info *sbresv.Info
	switch d := pld.Data.(type) {
	case *sbreq.EphemReq:
		info = d.Block.Info
		if id == nil {
			id = d.ID
		}
	case *sbreq.EphemFailed:
		info = d.Info
		if id == nil {
			id = d.ID
		}
	default:
		return common.NewBasicError("Invalid payload type", nil, "type", pld.Type)
	}
	if !id.Eq(r.id) {
		return common.NewBasicError("Invalid ephemeral ID", nil,
			"expected", r.id, "actual", id)
	}
	if !info.Eq(r.pld.Data.(*sbreq.EphemReq).Block.Info) {
		return common.NewBasicError("Info has been modified", nil,
			"expected", r.pld.Data.(*sbreq.EphemReq).Block.Info, "actual", info)
	}
	return nil
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

func (s *EphemSetup) PrepareRequest() (common.Extension, *sbreq.Pld, error) {
	s.entry.Lock()
	defer s.entry.Unlock()
	steady := s.entry.syncResv.Load().Steady
	if steady == nil {
		return nil, nil, common.NewBasicError("Steady extension not available", nil)
	}
	steady = steady.Copy().(*sbextn.Steady)
	info := &sbresv.Info{
		ExpTick:  sibra.CurrentTick() + sibra.MaxEphemTicks,
		BwCls:    s.bwCls,
		PathType: sibra.PathTypeEphemeral,
		RLC:      sibra.DurationToRLC(combineRLC(steady), true),
	}
	pld := &sbreq.Pld{
		Type:      sbreq.REphmSetup,
		NumHops:   uint8(steady.TotalHops),
		TimeStamp: uint32(time.Now().Unix()),
		Accepted:  true,
		Auths:     make([]common.RawBytes, steady.TotalHops),
		Data: &sbreq.EphemReq{
			ID:    s.id,
			Block: sbresv.NewBlock(info, steady.TotalHops),
		},
	}
	if err := steady.ToRequest(pld); err != nil {
		return nil, nil, err
	}
	return steady, pld, nil
}

func (s *EphemSetup) NotifyKeys() []*notifyKey {
	return []*notifyKey{{Id: s.id, Idx: s.idx, ReqType: sbreq.REphmSetup}}
}

func (s *EphemSetup) HandleRep(event notifyEvent) (bool, error) {
	if _, ok := event.extn.(*sbextn.Steady); !ok {
		return false, common.NewBasicError("Extension is not steady", nil)
	}
	if err := s.validate(nil, event.pld); err != nil {
		return false, err
	}
	steady := event.extn.(*sbextn.Steady)
	s.entry.Lock()
	defer s.entry.Unlock()
	switch r := event.pld.Data.(type) {
	case *sbreq.EphemReq:
		ids := []sibra.ID{r.ID}
		ids = append(ids, steady.IDs...)
		ephem, err := sbcreate.NewEphemUse(ids, steady.PathLens, r.Block, true)
		if err != nil {
			return false, err
		}
		s.entry.syncResv.UpdateEphem(ephem)
		s.entry.ephemMeta.timestamp = time.Now()
		s.entry.ephemMeta.state = ephemExists
	case *sbreq.EphemFailed:
		s.entry.ephemMeta.lastFailCode = r.FailCode
		s.entry.ephemMeta.lastMaxBw = r.MinOffer()
		s.entry.ephemMeta.timestamp = time.Now()
		return false, nil
	}
	return true, nil
}

type EphemRenew struct {
	*reserver
}

func (r *EphemRenew) PrepareRequest() (common.Extension, *sbreq.Pld, error) {
	r.entry.Lock()
	defer r.entry.Unlock()
	ephem := r.entry.syncResv.Load().Ephemeral
	if ephem == nil {
		return nil, nil, common.NewBasicError("Ephemeral extension not available", nil)
	}
	ephem = ephem.Copy().(*sbextn.Ephemeral)
	if ephem.ActiveBlocks[0].Info.Index.Add(1) != r.idx {
		return nil, nil, common.NewBasicError("Indexes out of sync", nil, "existing",
			ephem.ActiveBlocks[0].Info.Index, "next", r.idx)
	}
	r.id = ephem.IDs[0]
	info := &sbresv.Info{
		ExpTick:  sibra.CurrentTick() + sibra.MaxEphemTicks,
		BwCls:    r.bwCls,
		PathType: sibra.PathTypeEphemeral,
		RLC:      ephem.ActiveBlocks[0].Info.RLC,
		Index:    r.idx,
	}
	pld := &sbreq.Pld{
		Type:      sbreq.REphmRenewal,
		NumHops:   uint8(ephem.TotalHops),
		TimeStamp: uint32(time.Now().Unix()),
		Accepted:  true,
		Auths:     make([]common.RawBytes, ephem.TotalHops),
		Data: &sbreq.EphemReq{
			Block: sbresv.NewBlock(info, ephem.TotalHops),
		},
	}
	if err := ephem.ToRequest(pld); err != nil {
		return nil, nil, err
	}
	return ephem, pld, nil
}

func (r *EphemRenew) NotifyKeys() []*notifyKey {
	return []*notifyKey{{Id: r.id, Idx: r.idx, ReqType: sbreq.REphmRenewal}}
}

func (r *EphemRenew) HandleRep(event notifyEvent) (bool, error) {
	if _, ok := event.extn.(*sbextn.Ephemeral); !ok {
		return false, common.NewBasicError("Extension is not ephemeral", nil)
	}
	ephem := event.extn.(*sbextn.Ephemeral)
	if err := r.validate(ephem.IDs[0], event.pld); err != nil {
		return false, err
	}
	r.entry.Lock()
	defer r.entry.Unlock()
	switch request := event.pld.Data.(type) {
	case *sbreq.EphemReq:
		ephem, err := sbcreate.NewEphemUse(ephem.IDs, ephem.PathLens, request.Block, true)
		if err != nil {
			return false, err
		}
		r.entry.syncResv.UpdateEphem(ephem)
		r.entry.ephemMeta.timestamp = time.Now()
		r.entry.ephemMeta.state = ephemExists
	case *sbreq.EphemFailed:
		r.entry.ephemMeta.lastFailCode = request.FailCode
		r.entry.ephemMeta.lastMaxBw = request.MinOffer()
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

func (c *cleaner) validate(pld *sbreq.Pld) error {
	if pld.Data == nil {
		return common.NewBasicError("No data present", nil, "pld", pld)
	}
	switch d := pld.Data.(type) {
	case *sbreq.EphemClean:
		if d.Setup && !d.ID.Eq(c.id) {
			return common.NewBasicError("Invalid ephemeral ID", nil,
				"expected", c.id, "actual", d.ID)
		}
		if !d.Info.Eq(c.pld.Data.(*sbreq.EphemClean).Info) {
			return common.NewBasicError("Info has been modified", nil,
				"expected", c.pld.Data.(*sbreq.EphemClean).Info, "actual", d.Info)
		}
	default:
		return common.NewBasicError("Invalid payload type", nil, "type", pld.Type)
	}
	return nil
}

func (c *cleaner) NotifyKeys() []*notifyKey {
	return []*notifyKey{{Id: c.id, Idx: c.idx, ReqType: sbreq.REphmCleanUp}}
}

type EphemCleanSetup struct {
	*cleaner
}

func (c *EphemCleanSetup) PrepareRequest() (common.Extension, *sbreq.Pld, error) {
	c.entry.Lock()
	defer c.entry.Unlock()
	steady := c.entry.syncResv.Load().Steady
	if steady == nil {
		return nil, nil, common.NewBasicError("Steady extension not available", nil)
	}
	steady = steady.Copy().(*sbextn.Steady)
	pld := &sbreq.Pld{
		Type:      sbreq.REphmCleanUp,
		NumHops:   uint8(steady.TotalHops),
		TimeStamp: uint32(time.Now().Unix()),
		Accepted:  true,
		Auths:     make([]common.RawBytes, steady.TotalHops),
		Data: &sbreq.EphemClean{
			Setup: true,
			ID:    c.id,
			Info:  c.FailedInfo,
		},
	}
	if err := steady.ToRequest(pld); err != nil {
		return nil, nil, err
	}
	return steady, pld, nil
}

func (c *EphemCleanSetup) HandleRep(event notifyEvent) (bool, error) {
	if err := c.validate(event.pld); err != nil {
		return false, common.NewBasicError("Invalid response", nil, "type", event.pld.Type)
	}
	return event.pld.Accepted, nil
}

type EphemCleanRenew struct {
	*cleaner
}

func (c *EphemCleanRenew) PrepareRequest() (common.Extension, *sbreq.Pld, error) {
	c.entry.Lock()
	defer c.entry.Unlock()
	ephem := c.entry.syncResv.Load().Ephemeral
	if ephem == nil {
		return nil, nil, common.NewBasicError("Ephemeral extension not available", nil)
	}
	ephem = ephem.Copy().(*sbextn.Ephemeral)
	pld := &sbreq.Pld{
		Type:      sbreq.REphmCleanUp,
		NumHops:   uint8(ephem.TotalHops),
		TimeStamp: uint32(time.Now().Unix()),
		Accepted:  true,
		Auths:     make([]common.RawBytes, ephem.TotalHops),
		Data: &sbreq.EphemClean{
			Info: c.FailedInfo,
		},
	}
	if err := ephem.ToRequest(pld); err != nil {
		return nil, nil, err
	}
	return ephem, pld, nil
}

func (c *EphemCleanRenew) HandleRep(event notifyEvent) (bool, error) {
	if err := c.validate(event.pld); err != nil {
		return false, common.NewBasicError("Invalid response", nil, "type", event.pld.Type)
	}
	return event.pld.Accepted, nil
}
