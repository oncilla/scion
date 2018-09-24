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

package resvd

import (
	"time"

	"github.com/patrickmn/go-cache"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/l4"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbcreate"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/sibra_srv/adm"
	"github.com/scionproto/scion/go/sibra_srv/conf"
)

const (
	ErrorCreatePkt  = "Unable to create external packet"
	ErrorPrepareReq = "Unable to prepare request"
	ErrorHandleRep  = "Unable to handle reply"
	ErrorSendReq    = "Unable to send request"
)

type ReqstrI interface {
	CreateExtPkt() (*conf.ExtPkt, error)
	PrepareReq(pkt *conf.ExtPkt) error
	NotifyKey() []*conf.NotifyKey
	HandleRep(pkt *conf.ExtPkt) error
	OnError(err error)
	OnTimeout()
}

var _ ReqstrI = (*SteadySetup)(nil)
var _ ReqstrI = (*SteadyRenew)(nil)
var _ ReqstrI = (*ConfirmIndex)(nil)

type Reqstr struct {
	log.Logger
	errFunc  func(error, ReqstrI)
	timeFunc func(ReqstrI)
	succFunc func(ReqstrI)

	id      sibra.ID
	resvKey string
	stop    chan struct{}
	path    *spathmeta.AppPath
	srcHost addr.HostAddr
	dstHost addr.HostAddr
	block   *sbresv.Block
	timeout time.Duration
	idx     sibra.Index
}

func (r *Reqstr) Run(i ReqstrI) {
	pkt, err := i.CreateExtPkt()
	if err != nil {
		r.callErr(common.NewBasicError(ErrorCreatePkt, err), i)
		return
	}
	if err := i.PrepareReq(pkt); err != nil {
		r.callErr(common.NewBasicError(ErrorPrepareReq, err), i)
		return
	}
	notify := make(chan *conf.ExtPkt, 10)
	defer close(notify)
	for _, notifyKey := range i.NotifyKey() {
		master := conf.Get().RepMaster
		master.Register(notifyKey, notify)
		defer master.Deregister(notifyKey)
	}
	if err := r.sendPkt(pkt); err != nil {
		r.callErr(common.NewBasicError(ErrorSendReq, err), i)
		return
	}
	pkt = nil
	select {
	case pkt = <-notify:
		if err := i.HandleRep(pkt); err != nil {
			r.callErr(common.NewBasicError(ErrorHandleRep, err), i)
			return
		}
		if r.succFunc != nil {
			r.succFunc(i)
		}
	case <-time.After(r.timeout):
		r.callTimeOut(i)
	}
}

func (r *Reqstr) callErr(err error, i ReqstrI) {
	i.OnError(err)
	if r.errFunc != nil {
		r.errFunc(err, i)
	}
}

func (r Reqstr) callTimeOut(i ReqstrI) {
	i.OnTimeout()
	if r.timeFunc != nil {
		r.timeFunc(i)
	}
}

func (r *Reqstr) reversePkt(pkt *conf.ExtPkt) error {
	if err := pkt.Spkt.Reverse(); err != nil {
		return err
	}
	pkt.Spkt.SrcHost = pkt.Conf.PublicAddr.Host
	return nil
}

func (r *Reqstr) sendPkt(pkt *conf.ExtPkt) error {
	buf := make(common.RawBytes, pkt.Spkt.TotalLen())
	pktLen, err := hpkt.WriteScnPkt(pkt.Spkt, buf)
	if err != nil {
		return err
	}
	nextHopHost := r.path.Entry.HostInfo.Host()
	nextHopPort := r.path.Entry.HostInfo.Port
	appAddr := reliable.AppAddr{Addr: nextHopHost, Port: nextHopPort}
	written, err := pkt.Conf.Conn.WriteTo(buf[:pktLen], &appAddr)
	if err != nil {
		return err
	} else if written != pktLen {
		return common.NewBasicError("Wrote incomplete message", nil,
			"expected", pktLen, "actual", written)
	}
	// TODO(roods): remove
	r.Debug("Sent packet", "nextHopHost", nextHopHost, "port", nextHopPort)
	return nil
}

func (r *Reqstr) CreateExtPkt() (*conf.ExtPkt, error) {
	var err error
	pkt := &conf.ExtPkt{
		Conf: conf.Get(),
	}
	pkt.Steady, err = sbcreate.NewSteadyUse(r.id, r.block, !r.block.Info.PathType.Reversed())
	if err != nil {
		return nil, err
	}
	pkt.Spkt = &spkt.ScnPkt{
		DstIA:   r.path.Entry.Path.DstIA(),
		SrcIA:   r.path.Entry.Path.SrcIA(),
		DstHost: r.dstHost,
		SrcHost: r.srcHost,
		HBHExt:  []common.Extension{pkt.Steady},
		L4:      l4.L4Header(&l4.UDP{Checksum: make(common.RawBytes, 2)}),
		Pld:     make(common.RawBytes, 0),
	}
	return pkt, nil
}

func (r *Reqstr) OnError(err error) {
	r.Info("Error occurred", "err", err)
}

func (r *Reqstr) OnTimeout() {
	r.Info("Timed out")
}

type ResvReqstr struct {
	*Reqstr
	min sibra.BwCls
	max sibra.BwCls
}

func (r *ResvReqstr) handleRep(pkt *conf.ExtPkt) error {
	if err := r.validate(pkt); err != nil {
		return common.NewBasicError("Invalid reply", err)
	}
	if !pkt.Steady.Request.GetBase().Accepted {
		return common.NewBasicError("Reservation not accepted", nil, "req", pkt.Steady.Request)
	}
	if err := adm.PromoteToSOFCreated(pkt); err != nil {
		return common.NewBasicError("Failed to promote", err)
	}
	block := pkt.Steady.Request.(*sbreq.SteadySucc).Block
	r.Debug("Reservation has been accepted", "info", block.Info)
	e := &conf.LocalResvEntry{
		Id:       r.id.Copy(),
		State:    sibra.StateTemp,
		Block:    block,
		Creation: time.Now(),
	}
	conf.Get().LocalResvs.Set(r.id, r.idx, e, cache.DefaultExpiration)
	return nil
}

func (r *ResvReqstr) validate(pkt *conf.ExtPkt) error {
	if pkt.Steady.Request == nil {
		return common.NewBasicError("No request present", nil)
	}
	if !pkt.Steady.ReqID.Eq(r.id) {
		return common.NewBasicError("Invalid reservation id", nil,
			"expected", r.id, "actual", pkt.Steady.ReqID)
	}
	var info *sbresv.Info
	switch r := pkt.Steady.Request.(type) {
	case *sbreq.SteadyReq:
		info = r.Info
	case *sbreq.SteadySucc:
		info = r.Block.Info
	default:
		return common.NewBasicError("Invalid request type", nil, "type", r.GetBase().Type)
	}
	if info.Index != r.idx {
		return common.NewBasicError("Invalid index", nil, "expected", r.idx, "actual", info.Index)
	}
	return nil
}

type SteadySetup struct {
	*ResvReqstr
	path *spathmeta.AppPath
	pt   sibra.PathType
}

func (s *SteadySetup) probeRtt() (sibra.RttCls, error) {
	// FIXME(roosd): calculate RttCls
	s.timeout = sibra.RttCls(10).Duration()
	return 10, nil
}

func (s *SteadySetup) CreateExtPkt() (*conf.ExtPkt, error) {
	var err error
	pkt := &conf.ExtPkt{
		Conf: conf.Get(),
	}
	pLen := uint8((len(s.path.Entry.Path.Interfaces) + 2) / 2)
	rtt, err := s.probeRtt()
	if err != nil {
		return nil, common.NewBasicError("Unable to probe rtt class", err)
	}
	info := &sbresv.Info{
		ExpTick:  sibra.CurrentTick() + sibra.MaxSteadyTicks,
		BwCls:    s.max,
		RttCls:   rtt,
		PathType: s.pt,
		Index:    s.idx,
	}
	pkt.Steady, err = sbcreate.NewSteadySetup(sbreq.NewSteadyReq(
		sbreq.RSteadySetup, info, s.min, s.max, pLen), s.id)
	if err != nil {
		return nil, err
	}
	sPath := spath.New(s.path.Entry.Path.FwdPath)
	if err := sPath.InitOffsets(); err != nil {
		return nil, err
	}
	pkt.Spkt = &spkt.ScnPkt{
		DstIA:   s.path.Entry.Path.DstIA(),
		SrcIA:   s.path.Entry.Path.SrcIA(),
		DstHost: s.dstHost,
		SrcHost: s.srcHost,
		Path:    sPath,
		HBHExt:  []common.Extension{pkt.Steady},
		L4:      l4.L4Header(&l4.UDP{Checksum: make(common.RawBytes, 2)}),
		Pld:     make(common.RawBytes, 0),
	}
	return pkt, nil
}

func (s *SteadySetup) PrepareReq(pkt *conf.ExtPkt) error {
	resvReq := pkt.Steady.Request.(*sbreq.SteadyReq)
	if err := adm.AdmitSteadyResv(pkt, resvReq); err != nil {
		return common.NewBasicError("Unable to admit reservation", err)
	}
	if !resvReq.Accepted {
		return common.NewBasicError("Not enough bandwidth", nil)
	}
	if err := pkt.Steady.NextSOFIndex(); err != nil {
		return err
	}
	return nil
}

func (s *SteadySetup) NotifyKey() []*conf.NotifyKey {
	return []*conf.NotifyKey{{Id: s.id, Idx: s.idx, ReqType: sbreq.RSteadySetup}}
}

func (s *SteadySetup) HandleRep(pkt *conf.ExtPkt) error {
	if err := s.handleRep(pkt); err != nil {
		return err
	}
	block := pkt.Steady.Request.(*sbreq.SteadySucc).Block
	c := &ConfirmIndex{
		Reqstr: &Reqstr{
			Logger:  s.Logger.New("sub", "ConfirmIndex", "state", sibra.StatePending),
			id:      s.id,
			idx:     s.idx,
			resvKey: s.resvKey,
			stop:    s.stop,
			path:    s.path,
			srcHost: s.srcHost,
			dstHost: pkt.Spkt.SrcHost,
			block:   block,
			timeout: block.Info.RttCls.Duration(),
		},
		state: sibra.StatePending,
	}
	go c.Run(c)
	return nil
}

type SteadyRenew struct {
	*ResvReqstr
}

func (s *SteadyRenew) PrepareReq(pkt *conf.ExtPkt) error {
	info := &sbresv.Info{
		ExpTick:  sibra.CurrentTick() + sibra.MaxSteadyTicks,
		BwCls:    s.max,
		RttCls:   s.block.Info.RttCls,
		PathType: s.block.Info.PathType,
		Index:    s.idx,
	}
	resvReq := sbreq.NewSteadyReq(sbreq.RSteadyRenewal, info, s.min, s.max,
		uint8(s.block.NumHops()))
	err := pkt.Steady.ToRequest(resvReq)
	if err != nil {
		return err
	}
	if err := adm.AdmitSteadyResv(pkt, resvReq); err != nil {
		return common.NewBasicError("Unable to admit reservation", err)
	}
	if !resvReq.Accepted {
		return common.NewBasicError("Not enough bandwidth", nil)
	}
	return nil
}

func (s *SteadyRenew) NotifyKey() []*conf.NotifyKey {
	return []*conf.NotifyKey{{Id: s.id, Idx: s.idx, ReqType: sbreq.RSteadyRenewal}}
}

func (s *SteadyRenew) HandleRep(pkt *conf.ExtPkt) error {
	if err := s.handleRep(pkt); err != nil {
		return err
	}
	c := &ConfirmIndex{
		Reqstr: &Reqstr{
			Logger:  s.Logger.New("sub", "ConfirmIndex", "state", sibra.StatePending),
			id:      s.id,
			idx:     s.idx,
			resvKey: s.resvKey,
			stop:    s.stop,
			path:    s.path,
			srcHost: s.srcHost,
			dstHost: pkt.Spkt.SrcHost,
			block:   s.block,
			timeout: s.block.Info.RttCls.Duration(),
		},
		state: sibra.StatePending,
	}
	go c.Run(c)
	return nil
}

type ConfirmIndex struct {
	*Reqstr
	state sibra.State
}

func (c *ConfirmIndex) PrepareReq(pkt *conf.ExtPkt) error {
	r, err := sbreq.NewConfirmIndex(c.block.NumHops(), c.idx, c.state)
	if err != nil {
		return err
	}
	if err := pkt.Steady.ToRequest(r); err != nil {
		return err
	}
	return adm.Promote(pkt, r)
}

func (c *ConfirmIndex) NotifyKey() []*conf.NotifyKey {
	return []*conf.NotifyKey{{Id: c.id, Idx: c.idx, ReqType: sbreq.RSteadyConfIndex}}
}

func (c *ConfirmIndex) HandleRep(pkt *conf.ExtPkt) error {
	if err := c.validate(pkt); err != nil {
		return err
	}
	// correct response
	if !pkt.Steady.Request.GetBase().Accepted {
		c.Info("Index not accepted")
		// FIXME(roosd): Start clean up requester
	} else {
		conf.Get().LocalResvs.Get(c.id, c.idx).State = c.state
		c.Info("Index accepted")
	}
	return nil
}

func (c *ConfirmIndex) validate(pkt *conf.ExtPkt) error {
	if pkt.Steady.Request == nil {
		return common.NewBasicError("No request present", nil)
	}
	if !pkt.Steady.ReqID.Eq(c.id) {
		return common.NewBasicError("Invalid reservation id", nil,
			"expected", c.id, "actual", pkt.Steady.ReqID)
	}
	r, ok := pkt.Steady.Request.(*sbreq.ConfirmIndex)
	if !ok {
		return common.NewBasicError("Invalid request type", nil, "type", r.GetBase().Type)
	}
	if r.Idx != c.idx {
		return common.NewBasicError("Invalid index", nil, "expected", c.idx, "actual", r.Idx)
	}
	if r.State != c.state {
		return common.NewBasicError("Invalid state", nil, "expected", c.state, "actual", r.State)
	}
	return nil
}
