// Copyright 2018 Anapaya Systems
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

// Simple application to do service requests for SCION.
// It can be used to check if SVC requests are forwarded
// to the corresponding services.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/cert_mgmt"
	"github.com/scionproto/scion/go/lib/ctrl/path_mgmt"
	"github.com/scionproto/scion/go/lib/infra/disp"
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/infra/transport"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/scrypto"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
)

const (
	DefaultPeriod  = 1 * time.Second
	DefaultTimeout = 2 * time.Second
	DefaultCount   = 1
)

var (
	flagSvc = SvcFlag(addr.SvcNone)
	Local   = &snet.Addr{}
	Remote  = &snet.Addr{}

	timeout      = flag.Duration("timeout", DefaultTimeout, "Timeout for svc requests")
	period       = flag.Duration("period", DefaultPeriod, "Period of sending svc requests")
	sciondPath   = flag.String("sciond", "", "Path to sciond socket")
	dispatcher   = flag.String("dispatcher", reliable.DefaultDispPath, "Path to dispatcher socket")
	sciondFromIA = flag.Bool("sciondFromIA", false, "SCIOND socket path from IA address")
	count        = flag.Int("count", DefaultCount, "Number of svc requests that are sent")

	msgr *messenger.Messenger
	svc  addr.HostSVC
)

func init() {
	// Set up flag vars
	flag.Var((*snet.Addr)(Local), "local", "(Mandatory) address to listen on")
	flag.Var((*snet.Addr)(Remote), "remote", "(Mandatory) address to connect to")
	flag.Var(&flagSvc, "svc", "Indicates the SVC type to send message to (overridden by remote)")
}

type SvcFlag addr.HostSVC

func (f *SvcFlag) String() string {
	return addr.HostSVC(*f).String()
}

func (f *SvcFlag) Set(s string) error {
	*f = SvcFlag(addr.HostSVCFromString(s))
	return nil
}

func main() {
	os.Exit(realMain())
}

func realMain() int {
	log.AddLogConsFlags()
	flag.Parse()
	if err := log.SetupFromFlags(""); err != nil {
		fmt.Fprintf(os.Stderr, "CRIT: Unable to setup loging err=%s\n", err)
		return 1
	}
	defer log.LogPanicAndExit()
	if err := validateFlags(); err != nil {
		log.Crit("Unable to validate flags", "err", err)
		return 1
	}
	if err := snet.Init(Local.IA, *sciondPath, *dispatcher); err != nil {
		log.Crit("Unable to init snet", "err", err)
		return 1
	}
	conn, err := snet.ListenSCION("udp4", Local)
	if err != nil {
		log.Crit("Unable to listen on SCION", "err", err)
		return 1
	}
	msgr = messenger.New(
		Local.IA,
		disp.New(
			transport.NewPacketTransport(conn),
			messenger.DefaultAdapter,
			log.Root(),
		),
		nil,
		log.Root(),
		nil,
	)
	if err := doRequests(); err != nil {
		log.Error("Unable to do requests", "err", err)
		return 1
	}
	log.Info("Successfully sent request and received response", "svc", svc)
	return 0
}

func validateFlags() error {
	if Local.Host == nil {
		return common.NewBasicError("Local host not set", nil, "local", Local)
	}
	if Remote.Host == nil {
		return common.NewBasicError("Remote host not set", nil, "remote", Remote)
	}
	if *sciondFromIA {
		if *sciondPath != "" {
			return common.NewBasicError("Both -sciond or -sciondFromIA are specified", nil)
		}
		*sciondPath = sciond.GetDefaultSCIONDPath(&Local.IA)
	} else if *sciondPath == "" {
		*sciondPath = sciond.GetDefaultSCIONDPath(nil)
	}
	var remoteSvc addr.HostSVC
	Remote, remoteSvc = getRemote()
	switch {
	case remoteSvc == addr.SvcNone && addr.HostSVC(flagSvc) == addr.SvcNone:
		return common.NewBasicError("Svc not set", nil, "remote", Remote, "svc", svc)
	case remoteSvc == addr.SvcNone:
		svc = addr.HostSVC(flagSvc)
	default:
		svc = remoteSvc
	}
	return nil
}

func getRemote() (*snet.Addr, addr.HostSVC) {
	if svc, ok := Remote.Host.L3.(addr.HostSVC); ok {
		return &snet.Addr{IA: Remote.IA, Host: addr.NewSVCUDPAppAddr(svc)}, svc
	}
	return Remote, addr.SvcNone
}

func doRequests() error {
	ticker := time.NewTicker(*period)
	defer ticker.Stop()
	for i := 0; i < *count; i++ {
		if err := doRequest(); err != nil {
			return err
		}
		if i == *count-1 {
			break
		}
		<-ticker.C
	}
	return nil
}

func doRequest() error {
	ctx, cancleF := context.WithTimeout(context.Background(), *timeout)
	defer cancleF()
	switch addr.HostSVC(svc) {
	case addr.SvcCS:
		return doCS(ctx)
	case addr.SvcPS:
		return doPS(ctx)
	default:
		return common.NewBasicError("Not implemented", nil, "svc", addr.HostSVC(svc))
	}
}

func doCS(ctx context.Context) error {
	req := &cert_mgmt.ChainReq{
		CacheOnly: true,
		RawIA:     Remote.IA.IAInt(),
		Version:   scrypto.LatestVer,
	}
	log.Info("Request to SVC: Chain request", "req", req, "remote", Remote, "svc", svc)
	rawChain, err := msgr.GetCertChain(ctx, req, Remote, messenger.NextId())
	if err != nil {
		return common.NewBasicError("Unable to get chain", err)
	}
	chain, err := rawChain.Chain()
	if err != nil {
		return common.NewBasicError("Unable to parse chain", err)
	}
	if !chain.Leaf.Subject.Eq(Remote.IA) {
		return common.NewBasicError("Invalid subject", nil,
			"expected", Remote.IA, "actual", chain.Leaf.Subject)
	}
	log.Info("Response from SVC: Correct chain", "chain", chain)
	return nil
}

func doPS(ctx context.Context) error {
	req := &path_mgmt.SegReq{
		RawSrcIA: Remote.IA.IAInt(),
		RawDstIA: Local.IA.IAInt(),
	}
	log.Info("Request to SVC: Segement request", "req", req, "remote", Remote, "svc", svc)
	reply, err := msgr.GetSegs(ctx, req, Remote, messenger.NextId())
	if err != nil {
		return err
	}
	log.Info("Response from SVC", "recs", reply.Recs)
	return nil
}
