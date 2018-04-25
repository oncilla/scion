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

// Simple application for SCION connectivity using the snet library.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"context"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/infra/modules/trust"
	"github.com/scionproto/scion/go/lib/infra/modules/trust/trustdb"
	"github.com/scionproto/scion/go/lib/log"
	sd "github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
	"github.com/scionproto/scion/go/sibrad/resvmgr"
)

const (
	DefaultInterval = 1 * time.Second
	DefaultTimeout  = 2 * time.Second
	MaxPings        = 1 << 16
	ReqMsg          = "ping!" // ReqMsg and ReplyMsg length need to be the same
	ReplyMsg        = "pong!"
	TSLen           = 8
	ModeServer      = "server"
	ModeClient      = "client"
)

var (
	local  snet.Addr
	remote snet.Addr
	file   = flag.String("file", "",
		"File containing the data to send, optional to test larger data (only client)")
	interactive = flag.Bool("i", false, "Interactive mode")
	flush       = flag.Bool("f", false, "flush paths")
	id          = flag.String("id", "pingpong", "Element ID")
	mode        = flag.String("mode", ModeClient, "Run in "+ModeClient+" or "+ModeServer+" mode")
	sciond      = flag.String("sciond", "", "Path to sciond socket")
	dispatcher  = flag.String("dispatcher", "", "Path to dispatcher socket")
	count       = flag.Int("count", 0,
		fmt.Sprintf("Number of pings, between 0 and %d; a count of 0 means infinity", MaxPings))
	timeout = flag.Duration("timeout", DefaultTimeout,
		"Timeout for the ping response")
	interval     = flag.Duration("interval", DefaultInterval, "time between pings")
	bwCls        = flag.Uint("bw", 5, "bandwidth class")
	verbose      = flag.Bool("v", false, "sets verbose output")
	sciondFromIA = flag.Bool("sciondFromIA", false,
		"SCIOND socket path from IA address:ISD-AS")
	fileData []byte
)

func init() {
	flag.Var((*snet.Addr)(&local), "local", "(Mandatory) address to listen on")
	flag.Var((*snet.Addr)(&remote), "remote", "(Mandatory for clients) address to connect to")
	rand.Seed(time.Now().UTC().UnixNano())
}

func main() {
	log.AddLogConsFlags()
	validateFlags()
	if err := log.SetupFromFlags(""); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s", err)
		flag.Usage()
		os.Exit(1)
	}
	defer log.LogPanicAndExit()
	initNetwork()
	switch *mode {
	case ModeClient:
		if remote.Host == nil {
			LogFatal("Missing remote address")
		}
		if remote.L4Port == 0 {
			LogFatal("Invalid remote port", "remote port", remote.L4Port)
		}
		c := newClient()
		setSignalHandler(c)
		c.run()
	case ModeServer:
		server{}.run()
	}

}

func validateFlags() {
	flag.Parse()
	if *mode != ModeClient && *mode != ModeServer {
		LogFatal("Unknown mode, must be either '" + ModeClient + "' or '" + ModeServer + "'")
	}
	if *mode == ModeClient && remote.Host == nil {
		LogFatal("Missing remote address")
	}
	if local.Host == nil {
		LogFatal("Missing local address")
	}
	if *sciondFromIA {
		if *sciond != "" {
			LogFatal("Only one of -sciond or -sciondFromIA can be specified")
		}
		if local.IA.IsZero() {
			LogFatal("-local flag is missing")
		}
		*sciond = sd.GetDefaultSCIONDPath(&local.IA)
	} else if *sciond == "" {
		*sciond = sd.GetDefaultSCIONDPath(nil)
	}
	if *count < 0 || *count > MaxPings {
		LogFatal("Invalid count", "min", 0, "max", MaxPings, "actual", *count)
	}
	if *file != "" {
		if *mode == ModeClient {
			var err error
			fileData, err = ioutil.ReadFile(*file)
			if err != nil {
				LogFatal("Could not read data file")
			}
		} else {
			log.Info("file argument is ignored for mode " + ModeServer)
		}
	}
}

func LogFatal(msg string, a ...interface{}) {
	log.Crit(msg, a...)
	os.Exit(1)
}

func initNetwork() {
	// Initialize default SCION networking context
	if err := snet.Init(local.IA, *sciond, *dispatcher); err != nil {
		LogFatal("Unable to initialize SCION network", "err", err)
	}
	log.Debug("SCION network successfully initialized")
}

type message struct {
	PingPong  string
	Timestamp int64
}

func requestMsg() *message {
	return &message{
		PingPong: ReqMsg,
	}
}

func replyMsg(request *message) *message {
	return &message{
		ReplyMsg,
		request.Timestamp,
	}
}

func (m *message) len() int {
	return len(m.PingPong) + +8
}

func (m *message) pack() common.RawBytes {
	b := make(common.RawBytes, m.len())
	common.Order.PutUint64(b[:8], uint64(m.Timestamp))
	copy(b[8:], m.PingPong)
	return b
}

func unpack(b common.RawBytes) *message {
	return &message{
		Timestamp: int64(common.Order.Uint64(b[:8])),
		PingPong:  string([]byte(b[8:])),
	}
}

type client struct {
	sess *snet.Conn
	mgr  *resvmgr.Mgr
}

func newClient() *client {
	return &client{}
}

// run dials to a remote SCION address and repeatedly sends ping messages
// while receiving pong messages. For each successful ping-pong, a message
// with the round trip time is printed.
func (c *client) run() {

	c.initResvMgr()
	// Needs to happen before DialSCION, as it will 'copy' the remote to the connection.
	// If remote is not in local AS, we need a path!
	ws := c.setupPath()
	defer c.Close()
	// Connect to remote address. Note that currently the SCION library
	// does not support automatic binding to local addresses, so the local
	// IP address needs to be supplied explicitly. When supplied a local
	// port of 0, DialSCION will assign a random free local port.
	var err error
	c.sess, err = snet.DialSCION("udp4", &local, &remote)
	if err != nil {
		LogFatal("Unable to dial", "err", err)
	}
	quit := make(chan struct{})
	if ws != nil {
		ws.DrainEvents(quit)
	}
	go c.send(quit)
	c.read()
}

func (c *client) initResvMgr() {
	saddr := local.Copy()
	saddr.L4Port = sibra.Port
	conn, err := snet.ListenSCIONWithBindSVC("udp4", saddr, nil, addr.SvcNone)
	if err != nil {
		LogFatal("Unable to listen snet", "err", err)
	}
	db, err := trustdb.New("testdb.sql")
	if err != nil {
		LogFatal("Unable to initialize trustdb", "err", err)
	}
	store, err := trust.NewStore(db, local.IA, 0, nil, log.Root())
	if err != nil {
		LogFatal("Unable to create trust store", "err", err)
	}
	c.mgr, err = resvmgr.New(snet.DefNetwork.Sciond(), conn, store, nil)
	if err != nil {
		LogFatal("Unable to start reservation manager", err)
	}
}

func (c *client) Close() error {
	var err error
	if c.sess != nil {
		err = c.sess.Close()
	}
	return err
}

func (c client) setupPath() *resvmgr.WatchState {
	if !remote.IA.Eq(local.IA) {
		if *flush {
			if err := c.flushPath(); err != nil {
				LogFatal("Unable to flush", "err", err)
			}
		}
		pathEntry, ws := c.choosePath(*interactive)
		if pathEntry == nil {
			LogFatal("No paths available to remote destination")
		}
		remote.Path = spath.New(pathEntry.Path.FwdPath)
		remote.Path.InitOffsets()
		remote.SibraResv = ws.SyncResv
		remote.NextHopHost = pathEntry.HostInfo.Host()
		remote.NextHopPort = pathEntry.HostInfo.Port
		log.Info("sibrevs", "watchState", ws, "remote", remote.SibraResv)
		return ws
	}
	return nil
}

func (c client) send(quit chan struct{}) {
	for i := 0; i < *count || *count == 0; i++ {
		if i != 0 && *interval != 0 {
			time.Sleep(*interval)
		}
		select {
		case <-quit:
			log.Error("Resolver quit")
			return
		default:
			reqMsg := requestMsg()
			// Send ping message to destination
			before := time.Now()
			reqMsg.Timestamp = before.UnixNano()
			_, err := c.sess.Write(reqMsg.pack())
			if err != nil {
				log.Error("Unable to write", "err", err)
				continue
			}
		}

	}
}

func (c client) read() {
	// Receive pong message (with final timeout)
	for i := 0; i < *count || *count == 0; i++ {
		buf := make(common.RawBytes, 2<<16)
		n, err := c.sess.Read(buf)
		after := time.Now()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				log.Debug("ReadDeadline missed", "err", err)
				// ReadDeadline is only set after we are done writing
				// and we don't want to wait indefinitely for the remaining responses
				break
			}
			log.Error("Unable to read", "err", err)
			continue
		}
		msg := unpack(buf[:n])
		if msg.PingPong != ReplyMsg {
			log.Error("Received wrong pingpong", "expected", ReplyMsg, "actual", msg.PingPong)
		}
		before := time.Unix(0, int64(msg.Timestamp))
		elapsed := after.Sub(before).Round(time.Microsecond)
		if *verbose {
			fmt.Printf("[%s]\tReceived %d bytes from %v: seq=%d RTT=%s\n",
				before.Format(common.TimeFmt), msg.len(), &remote, i, elapsed)
		} else {
			fmt.Printf("Received %d bytes from %v: seq=%d RTT=%s\n",
				msg.len(), &remote, i, elapsed)
		}
	}
}

type packet struct {
	addr *snet.Addr
	raw  common.RawBytes
}

type server struct {
	sync.Mutex
	mgr      *resvmgr.Mgr
	sock     *snet.Conn
	sessions map[string]chan *packet
}

// run listens on a SCION address and replies to any ping message.
// On any error, the server exits.
func (s server) run() {
	s.sessions = make(map[string]chan *packet)
	s.initResvMgr()
	// Listen on SCION address
	var err error
	s.sock, err = snet.ListenSCION("udp4", &local)
	if err != nil {
		LogFatal("Unable to listen", "err", err)
	}
	log.Info("Listening", "local", s.sock.LocalAddr())
	for {
		b := make(common.RawBytes, 2<<16)
		n, raddr, err := s.sock.ReadFromSCION(b)
		if err != nil {
			log.Error("Unable to read", "err", err)
			// Accept failing means the socket is unusable.
			break
		}
		var c chan *packet
		var ok bool
		s.Mutex.Lock()
		if c, ok = s.sessions[raddr.String()]; !ok {
			c = make(chan *packet, 100)
			s.sessions[raddr.String()] = c
			go s.handleClient(raddr.String(), c)
			sibraMode := getSibraMode(raddr)
			log.Info("New session accepted", "src", raddr, "SibraMode", sibraMode)
		}
		s.Mutex.Unlock()
		c <- &packet{
			addr: raddr,
			raw:  b[:n],
		}
	}
}

func (s *server) initResvMgr() {
	saddr := local.Copy()
	saddr.L4Port = sibra.Port
	conn, err := snet.ListenSCIONWithBindSVC("udp4", saddr, nil, addr.SvcNone)
	if err != nil {
		LogFatal("Unable to listen snet", "err", err)
	}
	db, err := trustdb.New("testdb.sql")
	if err != nil {
		LogFatal("Unable to initialize trustdb", "err", err)
	}
	store, err := trust.NewStore(db, local.IA, 0, nil, log.Root())
	if err != nil {
		LogFatal("Unable to create trust store", "err", err)
	}
	s.mgr, err = resvmgr.New(snet.DefNetwork.Sciond(), conn, store, nil)
	if err != nil {
		LogFatal("Unable to start reservation manager", err)
	}
	// FIXME(roosd): unhardcode this
	_, ip4Net, _ := net.ParseCIDR("0.0.0.0/0")
	_, ip6Net, _ := net.ParseCIDR("::/0")
	s.mgr.AllowConnection(addr.IA{}, ip4Net)
	s.mgr.AllowConnection(addr.IA{}, ip6Net)
}

func getSibraMode(a net.Addr) string {
	saddr, ok := a.(*snet.Addr)
	if !ok {
		return "Non-sibra traffic"
	}
	_, ok = saddr.Sibra.(*sbextn.Steady)
	if ok {
		return "Steady best-effort traffic"
	}
	_, ok = saddr.Sibra.(*sbextn.Ephemeral)
	if ok {
		return "Ephemeral traffic"
	}
	return "Non-sibra traffic"

}

func (s server) handleClient(key string, c chan *packet) {
	for {
		select {
		case pkt := <-c:
			// Receive ping message
			msg := unpack(pkt.raw)
			// Send pong message
			replyMsg := replyMsg(msg)
			_, err := s.sock.WriteToSCION(replyMsg.pack(), pkt.addr)
			if err != nil {
				log.Error("Unable to write", "err", err)
				break
			}
		case <-time.After(5 * time.Second):
			log.Info("Client has disconnected", "addr", key)
			s.Lock()
			defer s.Unlock()
			delete(s.sessions, key)
			close(c)
			return
		}

	}
}

func (c *client) flushPath() error {
	sciondConn, err := snet.DefNetwork.Sciond().Connect()
	if err != nil {
		return err
	}
	rep, err := sciondConn.Paths(remote.IA, local.IA, 5, sd.PathReqFlags{Refresh: true})
	if err != nil {
		return common.NewBasicError("Unable to flush paths", err)
	}
	if rep.ErrorCode != sd.ErrorOk {
		return common.NewBasicError("Path flush failed", nil, "code", rep.ErrorCode)
	}
	return nil
}

func (c *client) choosePath(interactive bool) (*sd.PathReplyEntry, *resvmgr.WatchState) {
	var paths []*spathmeta.AppPath
	var pathIdx uint64

	pathMgr := snet.DefNetwork.PathResolver()

	syncPaths, err := pathMgr.Watch(local.IA, remote.IA)
	if err != nil {
		return nil, nil
	}
	pathSet := syncPaths.Load().APS
	if len(pathSet) == 0 {
		return nil, nil
	}
	sibraEnabled := make([]bool, len(pathSet))
	i := 0
	wg := sync.WaitGroup{}
	wg.Add(len(pathSet))
	for k, p := range pathSet {
		paths = append(paths, p)
		kCopy := k
		iCopy := i
		go func() {
			defer wg.Done()
			ctx, cancelF := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelF()
			ws, err := c.mgr.WatchSteady(ctx, &resvmgr.PathConf{
				Paths: syncPaths,
				Key:   kCopy,
			})
			if err != nil {
				log.Debug("Unable to fetch steady reservation", "key", iCopy, "err", err)
				return
			}
			sibraEnabled[iCopy] = ws.SyncResv.Load() != nil
			c.mgr.Unwatch(ws)

		}()
		i++
	}
	wg.Wait()
	if interactive {
		fmt.Printf("Available paths to %v\n", remote.IA)
		for i := range paths {
			fmt.Printf("[%2d] %s Sibra %t\n", i, paths[i].Entry.Path.String(), sibraEnabled[i])
		}
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Printf("Choose path: ")
			pathIndexStr, _ := reader.ReadString('\n')
			var err error
			pathIdx, err = strconv.ParseUint(pathIndexStr[:len(pathIndexStr)-1], 10, 64)
			if err == nil && int(pathIdx) < len(paths) {
				break
			}
			fmt.Fprintf(os.Stderr, "ERROR: Invalid path index, valid indices range: [0, %v]\n", len(paths))
		}
	} else {
		var found bool
		for i, v := range sibraEnabled {
			if v {
				pathIdx = uint64(i)
				found = true
				break
			}
		}
		if !found {
			LogFatal("No SIBRA enabled path found")
		}
	}
	// Setup ephemeral reservation
	params := &resvmgr.EphemConf{
		PathConf: &resvmgr.PathConf{
			Paths: syncPaths,
			Key:   paths[pathIdx].Key(),
		},
		MaxBWCls:    sbresv.BwCls(*bwCls),
		MinBWCls:    1,
		Destination: remote.Host,
	}
	ctx, cancelF := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelF()
	ws, err := c.mgr.WatchEphem(ctx, params)
	if err != nil {
		LogFatal("Error reserving ephemeral reservation", "err", err)
	}
	fmt.Printf("Using path:\n  %s\n", paths[pathIdx].Entry.Path.String())
	return paths[pathIdx].Entry, ws
}

func setSignalHandler(closer io.Closer) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		closer.Close()
		os.Exit(1)
	}()
}
