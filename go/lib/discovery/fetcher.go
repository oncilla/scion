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

package discovery

import (
	"context"
	"net/http"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/healthpool/svcinstance"
	"github.com/scionproto/scion/go/lib/periodic"
	"github.com/scionproto/scion/go/lib/topology"
)

var _ periodic.Task = (*Fetcher)(nil)

// Callbacks are used to inform the client. The functions are called when
// an associated event occurs. If the function is nil, it is ignored.
type Callbacks struct {
	// Raw is called with the raw body from the discovery service response and the parsed topology.
	Raw func(common.RawBytes, *topology.Topo)
	// Update is called with the parsed topology from the discovery service response.
	Update func(*topology.Topo)
	// Error is called with any error that occurs.
	Error func(error)
}

// Fetcher is used to fetch a new topology file from the discovery service.
type Fetcher struct {
	// Pool is a Pool of discovery services
	Pool *svcinstance.Pool
	// Params contains the parameters for fetching the topology.
	Params FetchParams
	// Callbacks contains the callbacks.
	Callbacks Callbacks
	// Client is the http Client. If nil, the default Client is used.
	Client *http.Client
}

// Run fetches a new topology file from the discovery service and calls the
// appropriate callback functions to notify the caller. RawF and UpdateF are
// only called if no error has occurred and the topology was parsed correctly.
// Otherwise ErrorF is called.
func (f *Fetcher) Run(ctx context.Context) {
	if err := f.run(ctx); err != nil && f.Callbacks.Error != nil {
		f.Callbacks.Error(err)
	}
}

func (f *Fetcher) run(ctx context.Context) error {
	if f.Pool == nil {
		return common.NewBasicError("Pool not initialized", nil)
	}
	// Choose a DS server.
	ds, err := f.Pool.Choose()
	if err != nil {
		return err
	}
	topo, raw, err := FetchTopoRaw(ctx, f.Params, ds.Addr(), f.Client)
	if err != nil {
		ds.Fail()
		return err
	}
	// Notify the client.
	if f.Callbacks.Raw != nil {
		f.Callbacks.Raw(raw, topo)
	}
	if f.Callbacks.Update != nil {
		f.Callbacks.Update(topo)
	}
	return nil
}
