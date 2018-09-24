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

package resvdb

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/resvdb/conn"
	"github.com/scionproto/scion/go/lib/sibra/resvdb/query"
	"github.com/scionproto/scion/go/lib/sibra/resvdb/sqlite"
)

type DB struct {
	conn conn.Conn
}

// New creates a new or open an existing PathDB at a given path using the
// given backend.
func New(path string, backend string) (*DB, error) {
	db := &DB{}
	var err error
	switch backend {
	case "sqlite":
		db.conn, err = sqlite.New(path)
	default:
		return nil, common.NewBasicError("Unknown backend", nil, "backend", backend)
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Insert inserts or updates a steady reservation. It returns the number of rows affected.
func (db *DB) Insert(bmeta *sibra_mgmt.BlockMeta) (int, error) {
	return db.conn.Insert(bmeta)
}

// Delete deletes a steady reservation with a given ID. Returns the number of deleted
// reservations (0 or 1).
func (db *DB) Delete(resvID sibra.ID) (int, error) {
	return db.conn.Delete(resvID)
}

// DeleteWithIntf deletes all reservations that contain a given interface. Returns
// the number of path segments deleted.
func (db *DB) DeleteWithIntf(intf query.IntfSpec) (int, error) {
	return db.conn.DeleteWithIntf(intf)
}

// Get returns all reservations matching the parameters specified.
func (db *DB) Get(params *query.Params) ([]*query.Result, error) {
	return db.conn.Get(params)
}
