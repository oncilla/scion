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

package conf

import (
	"path/filepath"
	"sync"
	"sync/atomic"

	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/as_conf"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/crypto"
	"github.com/scionproto/scion/go/lib/infra/messenger"
	"github.com/scionproto/scion/go/lib/infra/modules/trust"
	"github.com/scionproto/scion/go/lib/infra/modules/trust/trustdb"
	"github.com/scionproto/scion/go/lib/log"
	"github.com/scionproto/scion/go/lib/sibra/resvdb"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/sock/reliable"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/lib/util"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/impl"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

const (
	ErrorAddr    = "Unable to load addresses"
	ErrorConfNil = "Unable to reload conf from nil value"
	ErrorStore   = "Unable to load TrustStore"
	ErrorTopo    = "Unable to load topology"
	ErrorTrustDB = "Unable to load trust DB"

	ResvDBName = "resvdb.sqlite"
	SibraDir   = "sibra"
)

var conf atomic.Value

// Get returns a pointer to the current configuration.
func Get() *Conf {
	c := conf.Load()
	if c != nil {
		return c.(*Conf)
	}
	return nil
}

// Set updates the current configuration.
func Set(c *Conf) {
	conf.Store(c)
}

type Conf struct {
	// ID is the element ID.
	ID string
	// CacheDir is the cache directory.
	CacheDir string
	// ConfDir is the configuration directory.
	ConfDir string
	// StateDir is the state directory.
	StateDir string
	// ASConf is the local AS configuration
	ASConf *as_conf.ASConf
	// Topo contains the names of all local infrastructure elements,
	// a map of interface IDs to routers, and the actual topology.
	Topo *topology.Topo
	// BindAddr is the local bind address.
	BindAddr *snet.Addr
	// PublicAddr is the public address.
	PublicAddr *snet.Addr
	// TrustDB stores the trust objects.
	TrustDB *trustdb.DB
	// Store is the trust store.
	Store *trust.Store
	// Reservations holds information about the reservation the
	// service will automatically establish.
	Reservations ResvsMap
	// Matrix holds the reservable bandwidth between interfaces.
	Matrix state.Matrix
	// SibraAlgo holds the algorithm with state for handling SIBRA requests.
	SibraAlgo sbalgo.Algo
	// SOFMacPool is the pool for SIBRA OpField MAC generation.
	SOFMacPool *sync.Pool
	// ResvDB holds established SIBRA reservations.
	ResvDB *resvdb.DB
	// LocalResvs caches state of the reservations established by this service.
	LocalResvs *LocalResvs
	// Conn is the connection used to send traffic to the border router.
	Conn *reliable.Conn
	// RepMaster handles replies for SIBRA requests sent by this service.
	RepMaster RepMaster
	// Messenger handles all SIBRA messages received from the border router.
	Messenger *messenger.Messenger
}

// Load initializes the configuration by loading it from confDir.
func Load(id string, confDir string, cacheDir string, stateDir string) (*Conf, error) {
	c := &Conf{
		ID:         id,
		ConfDir:    confDir,
		CacheDir:   cacheDir,
		StateDir:   stateDir,
		LocalResvs: NewLocalResvs(),
	}
	if err := c.loadTopo(); err != nil {
		return nil, err
	}
	if err := c.loadMatrix(); err != nil {
		return nil, err
	}
	if err := c.loadResvDB(); err != nil {
		return nil, err
	}
	if err := c.loadReservations(); err != nil {
		return nil, err
	}
	if err := c.loadSibraState(); err != nil {
		return nil, err
	}
	if err := c.loadTrustDB(); err != nil {
		return nil, err
	}
	if err := c.loadStore(); err != nil {
		return nil, err
	}
	if err := c.loadASConf(); err != nil {
		return nil, err
	}
	if err := c.loadSOFMacPool(); err != nil {
		return nil, err
	}
	return c, nil
}

// ReloadConf loads a new configuration based on the old one.
func ReloadConf(oldConf *Conf) (*Conf, error) {
	if oldConf == nil {
		return nil, common.NewBasicError(ErrorConfNil, nil)
	}
	// FIXME(roosd): Support reloading of configuration.
	return nil, common.NewBasicError("Reload not supported yet", nil)
}

// loadTopo loads the topology information.
func (c *Conf) loadTopo() (err error) {
	path := filepath.Join(c.ConfDir, topology.CfgName)
	if c.Topo, err = topology.LoadFromFile(path); err != nil {
		return common.NewBasicError(ErrorTopo, err)
	}
	// load public and bind address
	topoAddr, ok := c.Topo.SB[c.ID]
	if !ok {
		return common.NewBasicError(ErrorAddr, nil, "err", "Element ID not found", "id", c.ID)
	}
	publicInfo := topoAddr.PublicAddrInfo(c.Topo.Overlay)
	c.PublicAddr = &snet.Addr{IA: c.Topo.ISD_AS, Host: addr.HostFromIP(publicInfo.IP),
		L4Port: uint16(publicInfo.L4Port)}
	bindInfo := topoAddr.BindAddrInfo(c.Topo.Overlay)
	tmpBind := &snet.Addr{IA: c.Topo.ISD_AS, Host: addr.HostFromIP(bindInfo.IP),
		L4Port: uint16(bindInfo.L4Port)}
	if !tmpBind.EqAddr(c.PublicAddr) {
		c.BindAddr = tmpBind
	}
	return nil
}

// loadTrustDB loads the trustdb.
func (c *Conf) loadTrustDB() error {
	var err error
	if c.TrustDB, err = trustdb.New(filepath.Join(c.StateDir, trustdb.Path)); err != nil {
		return common.NewBasicError(ErrorTrustDB, err)
	}
	return nil
}

// loadStore loads the trust store.
func (c *Conf) loadStore() error {
	var err error
	c.Store, err = trust.NewStore(
		c.TrustDB,
		c.Topo.ISD_AS,
		crypto.RandUint64(),
		&trust.Config{
			MustHaveLocalChain: true,
		},
		log.Root(),
	)
	if err != nil {
		return common.NewBasicError(ErrorStore, err)
	}
	if err := c.Store.LoadAuthoritativeTRC(filepath.Join(c.ConfDir, "certs")); err != nil {
		return err
	}
	if err := c.Store.LoadAuthoritativeChain(filepath.Join(c.ConfDir, "certs")); err != nil {
		return err
	}
	return nil
}

func (c *Conf) loadResvDB() (err error) {
	c.ResvDB, err = resvdb.New(filepath.Join(c.ConfDir, SibraDir, ResvDBName), "sqlite")
	return err
}

func (c *Conf) loadMatrix() (err error) {
	c.Matrix, err = state.MatrixFromFile(filepath.Join(c.ConfDir, SibraDir, state.MatrixName))
	return err
}
func (c *Conf) loadReservations() (err error) {
	c.Reservations, err = ReservationsFromFile(filepath.Join(c.ConfDir, SibraDir, ResvName))
	return err
}

func (c *Conf) loadSibraState() (err error) {
	c.SibraAlgo, err = impl.NewSibraFast(c.Topo, c.Matrix)
	return err
}

func (c *Conf) loadASConf() error {
	if err := as_conf.Load(filepath.Join(c.ConfDir, as_conf.CfgName)); err != nil {
		return err
	}
	c.ASConf = as_conf.CurrConf
	return nil
}

func (c *Conf) loadSOFMacPool() error {
	sofGenKey := pbkdf2.Key(c.ASConf.MasterASKey, []byte("Derive SOF Key"), 1000, 16, sha256.New)
	if _, err := util.InitMac(sofGenKey); err != nil {
		return err
	}
	c.SOFMacPool = &sync.Pool{
		New: func() interface{} {
			mac, _ := util.InitMac(sofGenKey)
			return mac
		},
	}
	return nil
}
