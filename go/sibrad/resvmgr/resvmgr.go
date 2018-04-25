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
	"sync"
	"time"

	"net"

	"context"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/infra"
	"github.com/scionproto/scion/go/lib/infra/disp"
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/infra/modules/trust"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/rpt"
	"github.com/scionproto/scion/go/proto"
	"github.com/scionproto/scion/go/sibrad/syncresv"
)

// Timers is used to customize the timers for a reservation manager.
type Timers struct {
	// Wait time after a successful reservation operation.
	NormalRefire time.Duration
	// Wait time after a failed reservation operation.
	ErrorRefire time.Duration
}

const (
	// Default wait time after a successful reservation operation.
	DefaultNormalRefire = sbresv.TickDuration / 2
	// Default wait time after a failed reservation operation.
	DefaultErrorRefire = 100 * time.Millisecond
)

func setDefaultTimers(timers *Timers) {
	if timers.NormalRefire == 0 {
		timers.NormalRefire = DefaultNormalRefire
	}
	if timers.ErrorRefire == 0 {
		timers.ErrorRefire = DefaultErrorRefire
	}
}

// Mgr takes care of the SIRBA reservation setup and management. A client
// can request the manager to manage a best effort steady reservation to a
// remote destination or an ephemeral reservation with guaranteed bandwidth.
// The manager communicates important events to the client through the
// watch state.
type Mgr struct {
	log.Logger
	mu sync.Mutex
	// conn is the connection for the messenger.
	conn *snet.Conn
	// msgr is the messenger used to communicate with the SIBRA service.
	msgr *messenger.Messenger
	// sciondConn is the sciondConn state.
	sciondConn *sciondConn
	// ephemRepHandler handles all replies for ephemeral requests.
	ephemRepHandler *ephemRepHandler
	// timers are the timers for refiring.
	timers *Timers
	// whitelist is a whitelist of allowed subnets. Only connections in the
	// whitelist are accepted.
	whitelist *whitelist
	// stopChans keeps track of the stop channels to the resolvers.
	stopChans map[ResvKey]chan struct{}
	// quitChan allows resolvers to notify the Mgr that they quit.
	quitChan chan quitSignal
	// closeChan allows the reservation manager to be gracefully shut down.
	closeChan chan struct{}
	// store holds the state of all reservations that are being watched.
	store *store
}

func New(srvc sciond.Service, conn *snet.Conn, store *trust.Store,
	timers *Timers) (*Mgr, error) {

	if conn.LocalAddr().(*snet.Addr).L4Port != sibra.Port {
		return nil, common.NewBasicError("Invalid port for reservation manager", nil,
			"expected", sibra.Port, "actual", conn.LocalAddr().(*snet.Addr).L4Port)
	}
	sdConn, err := srvc.Connect()
	if err != nil {
		// Let external code handle initial failure
		return nil, common.NewBasicError("Unable to connect to SCIOND", err)
	}
	if timers == nil {
		timers = &Timers{}
	}
	setDefaultTimers(timers)
	logger := log.New("mgrID", log.RandId(4))
	mgr := &Mgr{
		Logger:    logger,
		conn:      conn,
		store:     newStore(),
		timers:    timers,
		whitelist: &whitelist{},
		stopChans: make(map[ResvKey]chan struct{}),
		quitChan:  make(chan quitSignal, quitChanCap),
		sciondConn: &sciondConn{
			Srvc: srvc,
			Conn: sdConn,
		},
		ephemRepHandler: &ephemRepHandler{
			notify: make(map[string]chan common.Extension),
			Logger: logger.New("handles", "ephemRep"),
		},
		msgr: messenger.New(
			disp.New(
				rpt.New(conn, logger),
				messenger.DefaultAdapter,
				logger,
			),
			store,
			logger,
			nil,
		),
	}
	reqHandler := &ephemReqHandler{
		whitelist: mgr.whitelist,
		Logger:    logger.New("handles", "ephemReq"),
	}
	mgr.msgr.AddHandler(infra.SIBRAEphemReq, reqHandler)
	mgr.msgr.AddHandler(infra.SIBRAEphemRep, mgr.ephemRepHandler)
	go mgr.msgr.ListenAndServe()
	go mgr.listenQuits()
	go mgr.updateLocalSvcSB()
	return mgr, nil
}

// AllowConnection allows a ipnet to establish connections with this host.
// 0 will be treated as wildcard for both ISD and AS identifiers.
func (r *Mgr) AllowConnection(ia addr.IA, ipNet *net.IPNet) {
	r.whitelist.insert(ia, ipNet)
}

// WatchSteady starts a resolver that periodically updates the steady extension
// in the watch state. The extension can be used to send best effort traffic.
func (r *Mgr) WatchSteady(ctx context.Context, pconf *PathConf) (*WatchState, error) {
	syncResvs := syncresv.NewStore(nil, nil)
	entry := &resvEntry{
		paths:     pconf.Paths,
		pathKey:   pconf.Key,
		syncResv:  syncResvs,
		fixedPath: true,
	}
	resvKey, err := r.store.addResv(entry)
	if err != nil {
		return nil, err
	}
	return r.startResolverAndWait(ctx, syncResvs, resvKey)
}

// WatchEphem starts a resolver that periodically updates the steady and ephemeral
// extension in the watch state. The steady extension can be used to send best effort
// traffic. The ephemeral extension can be used to send guaranteed and best effort
// traffic.
func (r *Mgr) WatchEphem(ctx context.Context, econf *EphemConf) (*WatchState, error) {

	syncResvs := syncresv.NewStore(nil, nil)
	entry := &resvEntry{
		paths:     econf.Paths,
		pathKey:   econf.Key,
		syncResv:  syncResvs,
		fixedPath: true,
		ephemMeta: &ephemMeta{
			remote:   econf.Destination,
			minBwCls: econf.MinBWCls,
			maxBwCls: econf.MaxBWCls,
		},
	}
	resvKey, err := r.store.addResv(entry)
	if err != nil {
		return nil, err
	}
	return r.startResolverAndWait(ctx, syncResvs, resvKey)
}

func (r *Mgr) startResolverAndWait(ctx context.Context, sresvs *syncresv.Store,
	key ResvKey) (*WatchState, error) {

	return r.waitInitSetup(ctx, sresvs, key, r.startResolver(key))
}

func (r *Mgr) startResolver(key ResvKey) chan *Event {
	events := make(chan *Event, eventChanCap)
	stop := make(chan struct{})
	res := &resolver{
		Logger:     r.New("resolver", key),
		sciondConn: r.sciondConn,
		conn:       r.conn,
		msgr:       r.msgr,
		repMaster:  r.ephemRepHandler,
		timers:     r.timers,
		key:        key,
		stop:       stop,
		quitChan:   r.quitChan,
		events:     events,
		store:      r.store,
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopChans[key] = stop
	// start resolver
	go res.run()
	return events
}

func (r *Mgr) waitInitSetup(ctx context.Context, sresvs *syncresv.Store,
	key ResvKey, events chan *Event) (*WatchState, error) {

	var err error
	select {
	case event := <-events:
		if event.Error != nil {
			err = event.Error
		}
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		r.stopResolver(key)
		return nil, err
	}
	ws := &WatchState{
		resvKey:  key,
		Events:   events,
		SyncResv: sresvs,
	}
	return ws, nil
}

// Unwatch stops the resolver associated with the WatchState.
func (r *Mgr) Unwatch(ws *WatchState) error {
	return r.stopResolver(ws.resvKey)
}

// listenQuits listens for any quitting resolver and removes it from the mapping.
func (r *Mgr) listenQuits() {
	logger := r.New("goroutine", "resvmgr.listenQuits")
	for {
		select {
		case <-r.closeChan:
			return
		case qs := <-r.quitChan:
			if qs.err != nil {
				logger.Info("Resolver quit with error", "err", qs.err)
			}
			r.stopResolver(qs.key)
		}
	}
}
func (r *Mgr) stopResolver(key ResvKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if stop, ok := r.stopChans[key]; ok {
		select {
		case <-stop:
			// Channel is already closed
			return nil
		default:
			close(stop)
			delete(r.stopChans, key)
			return r.store.removeResv(key)
		}

	}
	return common.NewBasicError("Resolver already stopped or not found", nil, "key", key)
}

func (r *Mgr) updateLocalSvcSB() {
	logger := r.New("goroutine", "resvmgr.updateLocalSvcSB")
	for {
		select {
		case <-r.closeChan:
			return
		default:
			svcSB, err := r.getLocalSvcSB(logger)
			if err != nil {
				logger.Error("[Mgr] Unable to update local SIBRA service address", "err", err)
				continue
			}
			r.sciondConn.Lock()
			r.sciondConn.SvcSB = svcSB
			r.sciondConn.Unlock()
			time.Sleep(svcSbUpdateInterval)
		}
	}
}

func (r *Mgr) getLocalSvcSB(logger log.Logger) (*snet.Addr, error) {
	info, err := r.sciondConn.Conn.SVCInfo([]proto.ServiceType{proto.ServiceType_sb})
	if err != nil || len(info.Entries) < 1 {
		if err != nil {
			r.reconnect(logger)
		}
		return nil, common.NewBasicError("Sibra service info not present", nil)
	}
	saddr := &snet.Addr{
		IA:     snet.DefNetwork.IA(),
		Host:   info.Entries[0].HostInfos[0].Host(),
		L4Port: info.Entries[0].HostInfos[0].Port,
	}
	return saddr, nil
}

func (r *Mgr) reconnect(logger log.Logger) {
	for {
		select {
		case <-r.closeChan:
			return
		default:
			sciondConn, err := r.sciondConn.Srvc.Connect()
			if err != nil {
				logger.Error("Unable to connect to sciond", "err", err)
				// wait for three seconds before trying again
				time.Sleep(reconnectInterval)
				continue
			}
			r.sciondConn.Lock()
			r.sciondConn.Conn = sciondConn
			r.sciondConn.Unlock()
			return
		}
	}
}

func (r *Mgr) Close() error {
	// Protect against concurrent Close calls
	r.mu.Lock()
	defer r.mu.Unlock()
	select {
	case <-r.closeChan:
		// Already closed, so do nothing
	default:
		close(r.closeChan)
		for _, c := range r.stopChans {
			close(c)
		}
		r.stopChans = nil
		r.sciondConn.Conn.Close()
		r.msgr.CloseServer()
	}
	return nil
}
