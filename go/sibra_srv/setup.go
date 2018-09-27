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

package main

import (
	"time"

	log "github.com/inconshreveable/log15"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/infra"
	"github.com/scionproto/scion/go/lib/infra/disp"
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/snet/rpt"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/sibra_srv/conf"
	"github.com/scionproto/scion/go/sibra_srv/dist"
	"github.com/scionproto/scion/go/sibra_srv/resvd"
)

const (
	ErrorConf = "Unable to load configuration"
	ErrorSNET = "Unable to create local SCION Network context"
)

// setup loads and sets the newest configuration. If needed, the
// snet/dispatcher are initialized.
//
// FIXME(roosd): Reloading is currently disabled. Thus, this function is currently
// only called once. Move to env/conf when it is done to enable reloading.
// Adapt conf/conf.go:ReloadConf to allow reloading.
func setup() error {
	oldConf := conf.Get()
	newConf, err := loadConf(oldConf)
	if err != nil {
		return common.NewBasicError(ErrorConf, err)
	}
	// Initialize snet with retries if not already initialized
	if oldConf == nil {
		return setupNewConf(newConf)
	}
	return nil
}

func setupNewConf(config *conf.Conf) error {
	var err error
	config.Conn, err = registerReliableConn(config)
	if err != nil {
		return common.NewBasicError("Unable to register reliable connection", err)
	}
	if err := initSNET(config.PublicAddr.IA, initAttempts, initInterval); err != nil {
		return common.NewBasicError(ErrorSNET, err)
	}
	conn, err := snet.ListenSCIONWithBindSVC("udp4", config.PublicAddr, config.BindAddr, addr.SvcSB)
	if err != nil {
		return err
	}
	config.Messenger = messenger.New(
		disp.New(
			rpt.New(conn, log.Root()),
			messenger.DefaultAdapter,
			log.Root(),
		),
		config.Store,
		log.Root(),
		nil,
	)
	config.Messenger.AddHandler(infra.SIBRAExtPkt, &ExternalHandler{})
	config.Messenger.AddHandler(infra.SIBRAEphemReq, &ExternalHandler{})
	config.Messenger.AddHandler(infra.SIBRAEphemRep, &ExternalHandler{})
	config.Messenger.AddHandler(infra.SIBRASteadyReq, &dist.SteadyReqHandler{})
	config.Messenger.AddHandler(infra.SIBRASteadyReg, &dist.SteadyRegHandler{})
	go func() {
		defer fatal("Messenger crashed")
		config.Messenger.ListenAndServe()
	}()

	config.RepMaster = &resvd.ResvMaster{
		Notify:     make(map[string]chan *conf.ExtPkt),
		ResvHandls: make(map[string]*resvd.Reserver),
	}
	defer func() { go config.RepMaster.Run() }()
	conf.Set(config)
	return nil
}

func registerReliableConn(conf *conf.Conf) (*reliable.Conn, error) {
	address := &reliable.AppAddr{Addr: conf.PublicAddr.Host}
	var bindAddress *reliable.AppAddr
	if conf.BindAddr != nil {
		bindAddress = &reliable.AppAddr{Addr: conf.BindAddr.Host}
	}
	conn, _, err := reliable.Register(*dispPath, conf.PublicAddr.IA, address,
		bindAddress, addr.SvcNone)
	return conn, err
}

// loadConf loads the newest configuration.
func loadConf(oldConf *conf.Conf) (*conf.Conf, error) {
	if oldConf != nil {
		return conf.ReloadConf(oldConf)
	}
	return conf.Load(*id, *confDir, *cacheDir, *stateDir)
}

// initSNET initializes snet. The number of attempts is specified, as well as the sleep duration.
// This is needed, since supervisord might take some time, until sciond is initialized.
func initSNET(ia addr.IA, attempts int, sleep time.Duration) (err error) {
	// Initialize SCION local networking module
	for i := 0; i < attempts; i++ {
		if err = snet.Init(ia, *sciondPath, *dispPath); err == nil {
			break
		}
		log.Error("Unable to initialize snet", "retry interval", sleep, "err", err)
		time.Sleep(sleep)
	}
	return err
}
