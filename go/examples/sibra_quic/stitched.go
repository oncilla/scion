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
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/qerr"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/infra/modules/trust"
	"github.com/scionproto/scion/go/lib/infra/modules/trust/trustdb"
	"github.com/scionproto/scion/go/lib/log"
	sd "github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/squic"
	"github.com/scionproto/scion/go/lib/spath"
	"github.com/scionproto/scion/go/sibrad/resvmgr"
	"github.com/scionproto/scion/go/sibrad/syncresv"
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
	if err := squic.Init("", ""); err != nil {
		LogFatal("Unable to initialize QUIC/SCION", "err", err)
	}
	log.Debug("QUIC/SCION successfully initialized")
}

type message struct {
	PingPong  string
	Data      []byte
	Timestamp int64
}

func requestMsg() *message {
	return &message{
		PingPong: ReqMsg,
		Data:     fileData,
	}
}

func replyMsg(request *message) *message {
	return &message{
		ReplyMsg,
		request.Data,
		request.Timestamp,
	}
}

func (m *message) len() int {
	return len(m.PingPong) + len(m.Data) + 8
}

type quicStream struct {
	qstream quic.Stream
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func newQuicStream(qstream quic.Stream) *quicStream {
	return &quicStream{
		qstream,
		gob.NewEncoder(qstream),
		gob.NewDecoder(qstream),
	}
}

func (qs quicStream) WriteMsg(msg *message) error {
	return qs.encoder.Encode(msg)
}

func (qs quicStream) ReadMsg() (*message, error) {
	var msg message
	err := qs.decoder.Decode(&msg)
	if err != nil {
		return nil, err
	}
	return &msg, err
}

type signalStream struct {
	qstream quic.Stream
	encoder *gob.Encoder
	decoder *gob.Decoder
}

func newSignalStream(qstream quic.Stream) *signalStream {
	return &signalStream{
		qstream,
		gob.NewEncoder(qstream),
		gob.NewDecoder(qstream),
	}
}

func (qs signalStream) WriteSignal(msg *message) error {
	return qs.encoder.Encode(msg)
}

func (qs signalStream) ReadSignal() (*message, error) {
	var msg message
	err := qs.decoder.Decode(&msg)
	if err != nil {
		return nil, err
	}
	return &msg, err
}

type client struct {
	*quicStream
	qsess        quic.Session
	mgr          *resvmgr.Mgr
	signalStream *signalStream
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
	c.setupPath()
	defer c.Close()
	// Connect to remote address. Note that currently the SCION library
	// does not support automatic binding to local addresses, so the local
	// IP address needs to be supplied explicitly. When supplied a local
	// port of 0, DialSCION will assign a random free local port.
	var err error
	c.qsess, err = squic.DialSCION(nil, &local, &remote)
	if err != nil {
		LogFatal("Unable to dial", "err", err)
	}

	qstream, err := c.qsess.OpenStreamSync()
	if err != nil {
		LogFatal("quic OpenStream failed", "err", err)
	}
	c.quicStream = newQuicStream(qstream)
	log.Debug("Quic stream opened", "local", &local, "remote", &remote)
	qstream, err = c.qsess.OpenStreamSync()
	if err != nil {
		LogFatal("quic OpenStream signal failed", "err", err)
	}
	c.signalStream = newSignalStream(qstream)
	log.Debug("Quic signal stream opened", "local", &local, "remote", &remote)
	go c.send()
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
	if c.qstream != nil {
		err = c.qstream.Close()
	}
	if err == nil && c.qsess != nil {
		// Note closing the session here is fine since we know that all the traffic went through.
		// If you are not sure that this is the case you should probably not close the session.
		// E.g. if you are just sending something to a server and closing the session immediately
		// it might be that the server does not see the message.
		// See also: https://github.com/lucas-clemente/quic-go/issues/464
		err = c.qsess.Close(nil)
	}
	return err
}

func (c client) setupPath() {
	if !remote.IA.Eq(local.IA) {
		if *flush {
			if err := c.flushPath(); err != nil {
				LogFatal("Unable to flush", "err", err)
			}
		}
		pathEntry, syncResvs := c.choosePath(*interactive)
		if pathEntry == nil {
			LogFatal("No paths available to remote destination")
		}
		remote.Path = spath.New(pathEntry.Path.FwdPath)
		remote.Path.InitOffsets()
		remote.SibraResv = syncResvs
		remote.NextHopHost = pathEntry.HostInfo.Host()
		remote.NextHopPort = pathEntry.HostInfo.Port
		log.Info("sibrevs", "syncResvs", syncResvs, "remote", remote.SibraResv)
	}
}

func (c client) send() {
	for i := 0; i < *count || *count == 0; i++ {
		if i != 0 && *interval != 0 {
			time.Sleep(*interval)
		}
		log.Info("sent i", "i", i)
		reqMsg := requestMsg()
		// Send ping message to destination
		before := time.Now()
		reqMsg.Timestamp = before.UnixNano()
		err := c.WriteMsg(reqMsg)
		if err != nil {
			log.Error("Unable to write", "err", err)
			continue
		}
		if i == 6 {
			reqMsg := requestMsg()
			reqMsg.PingPong = "Hello"
			err := c.signalStream.WriteSignal(reqMsg)
			if err != nil {
				log.Error("Unable to write signal", "err", err)
				continue
			}
		}
	}
	// After sending the last ping, set a ReadDeadline on the stream
	err := c.qstream.SetReadDeadline(time.Now().Add(*timeout))
	if err != nil {
		LogFatal("SetReadDeadline failed", "err", err)
	}
}

func (c client) read() {
	// Receive pong message (with final timeout)
	for i := 0; i < *count || *count == 0; i++ {
		msg, err := c.ReadMsg()
		after := time.Now()
		log.Info("got i", "i", i)
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
		if msg.PingPong != ReplyMsg {
			log.Error("Received wrong pingpong", "expected", ReplyMsg, "actual", msg.PingPong)
		}
		if !bytes.Equal(msg.Data, fileData) {
			log.Error("Received different data than sent.")
			continue
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

type server struct {
	mgr *resvmgr.Mgr
}

// run listens on a SCION address and replies to any ping message.
// On any error, the server exits.
func (s server) run() {
	s.initResvMgr()
	// Listen on SCION address
	qsock, err := squic.ListenSCION(nil, &local)
	if err != nil {
		LogFatal("Unable to listen", "err", err)
	}
	log.Info("Listening", "local", qsock.Addr())
	for {
		qsess, err := qsock.Accept()
		if err != nil {
			log.Error("Unable to accept quic session", "err", err)
			// Accept failing means the socket is unusable.
			break
		}
		sibraMode := getSibraMode(qsess.RemoteAddr())
		log.Info("Quic session accepted", "src", qsess.RemoteAddr(), "SibraMode", sibraMode)
		go s.handleClient(qsess)
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

func (s server) handleClient(qsess quic.Session) {
	defer qsess.Close(nil)
	qstream, err := qsess.AcceptStream()
	if err != nil {
		log.Error("Unable to accept signal quic stream", "err", err)
		return
	}
	defer qstream.Close()

	signalStream, err := qsess.AcceptStream()
	if err != nil {
		log.Error("Unable to accept quic stream", "err", err)
		return
	}
	defer signalStream.Close()

	qs := newQuicStream(qstream)
	signal := newSignalStream(signalStream)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for {
			// Receive ping message
			msg, err := signal.ReadSignal()
			if err != nil {
				qer := qerr.ToQuicError(err)
				// We expect qerr.PeerGoingAway or io.EOF as normal termination conditions.
				if qer.ErrorCode == qerr.PeerGoingAway ||
					err == io.EOF {
					log.Info("Quic peer disconnected", "err", err)
					break
				}
				// NetworkIdleTimeOut if the peer exits ungracefully we will not be notified,
				// and the session will time out.
				if qer.ErrorCode == qerr.NetworkIdleTimeout {
					log.Error("Quic connection timed out", "err", err)
					break
				}
				log.Error("Unable to read", "err", err)
				break
			}
			log.Info("signal", msg.PingPong)
			//qsess.RemoteAddr().(*snet.Addr).Sibra = nil
			// Send pong message
			replyMsg := replyMsg(msg)
			replyMsg.PingPong = "Hello"
			err = signal.WriteSignal(replyMsg)
			if err != nil {
				log.Error("Unable to write", "err", err)
				break
			}
		}
		wg.Done()
	}()
	for {
		// Receive ping message
		msg, err := qs.ReadMsg()
		if err != nil {
			qer := qerr.ToQuicError(err)
			// We expect qerr.PeerGoingAway or io.EOF as normal termination conditions.
			if qer.ErrorCode == qerr.PeerGoingAway ||
				err == io.EOF {
				log.Info("Quic peer disconnected", "err", err)
				break
			}
			// NetworkIdleTimeOut if the peer exits ungracefully we will not be notified,
			// and the session will time out.
			if qer.ErrorCode == qerr.NetworkIdleTimeout {
				log.Error("Quic connection timed out", "err", err)
				break
			}
			log.Error("Unable to read", "err", err)
			break
		}

		// Send pong message
		replyMsg := replyMsg(msg)
		err = qs.WriteMsg(replyMsg)
		if err != nil {
			log.Error("Unable to write", "err", err)
			break
		}
	}
	wg.Wait()
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

func (c *client) choosePath(interactive bool) (*sd.PathReplyEntry, *syncresv.Store) {
	/*
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
		sibraResvs := make([]*syncresv.Store, len(pathSet))
		sibraKeys := make([]resvmgr.ResvKey, len(pathSet))
		i := 0
		wg := sync.WaitGroup{}
		wg.Add(len(pathSet))
		for k, p := range pathSet {
			paths = append(paths, p)
			kCopy := k
			iCopy := i
			go func() {
				defer wg.Done()
				sresv, skey, err := c.mgr.WatchSteady(syncPaths, kCopy)
				if err != nil {
					log.Error("Error fetching steady reservation for path", "key", kCopy, "err", err)
					return
				}
				resv := sresv.Load()
				var ext common.Extension
				if resv != nil {
					ext, _ = resv.GetExtn()
				}
				log.Info("found sresvs", "sresv", resv, "ext", ext)
				sibraResvs[iCopy] = sresv
				sibraKeys[iCopy] = skey
			}()
			i++
		}
		wg.Wait()
		log.Info("found sresvs", "sresvs", sibraResvs)
		if interactive {
			fmt.Printf("Available paths to %v\n", remote.IA)
			for i := range paths {
				fmt.Printf("[%2d] %s Sibra %t\n", i, paths[i].Entry.Path.String(), sibraResvs[i] != nil)
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
			for i, v := range sibraResvs {
				if v != nil {
					pathIdx = uint64(i)
					found = true
					break
				}
			}
			if !found {
				LogFatal("No SIBRA enabled path found")
			}
		}
		// Do ephemeral reservation
		params := &resvmgr.EphemConf{
			Paths:       syncPaths,
			PathKey:     paths[pathIdx].Key(),
			MaxBWCls:    resv.BwCls(*bwCls),
			MinBWCls:    1,
			Destination: remote.Host,
		}
		sresvs, _, err := c.mgr.WatchEphem(params)
		if err != nil {
			LogFatal("Error reserving ephemeral reservation", "err", err)
		}
		fmt.Printf("Using path:\n  %s\n", paths[pathIdx].Entry.Path.String())
		return paths[pathIdx].Entry, sresvs*/
	return nil, nil
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
