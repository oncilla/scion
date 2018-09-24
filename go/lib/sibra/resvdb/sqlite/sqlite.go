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

package sqlite

import (
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/resvdb/conn"
	"github.com/scionproto/scion/go/lib/sibra/resvdb/query"
	"github.com/scionproto/scion/go/lib/sqlite"
)

type resvMeta struct {
	RowID       int64
	SegID       common.RawBytes
	Creation    time.Time
	LastUpdated time.Time
}

var _ conn.Conn = (*Backend)(nil)

type Backend struct {
	sync.RWMutex
	db *sql.DB
	tx *sql.Tx
}

// New returns a new SQLite backend opening a database at the given path. If
// no database exists a new database is be created. If the schema version of the
// stored database is different from the one in schema.go, an error is returned.
func New(path string) (*Backend, error) {
	db, err := sqlite.New(path, Schema, SchemaVersion)
	if err != nil {
		return nil, err
	}
	return &Backend{
		db: db,
	}, nil
}

func (b *Backend) begin() error {
	if b.tx != nil {
		return common.NewBasicError("A transaction already exists", nil)
	}
	var err error
	if b.tx, err = b.db.Begin(); err != nil {
		return common.NewBasicError("Failed to create transaction", err)
	}
	return nil
}

func (b *Backend) commit() error {
	if b.tx == nil {
		return common.NewBasicError("No transaction to commit", nil)
	}
	if err := b.tx.Commit(); err != nil {
		b.tx = nil
		return common.NewBasicError("Failed to commit transaction", err)
	}
	b.tx = nil
	return nil
}

func (b *Backend) Insert(bmeta *sibra_mgmt.BlockMeta) (int, error) {
	b.Lock()
	defer b.Unlock()
	if b.db == nil {
		return 0, common.NewBasicError("No database open", nil)
	}
	// Check if we already have a reservation.
	meta, err := b.get(bmeta.Id)
	if err != nil {
		return 0, err
	}
	if meta != nil {
		// Check if the new reservation is more recent.
		if bmeta.Creation().After(meta.Creation) {
			if err := b.updateExisting(bmeta, meta); err != nil {
				return 0, err
			}
			return 1, nil
		}
		return 0, nil
	}
	// Do full insert.
	if err = b.insert(bmeta); err != nil {
		return 0, err
	}
	return 1, nil
}

func (b *Backend) insert(meta *sibra_mgmt.BlockMeta) error {
	// Create new transaction
	if err := b.begin(); err != nil {
		return err
	}
	// Update segment.
	if err := b.insertFull(meta); err != nil {
		b.tx.Rollback()
		return err
	}
	// Commit transaction
	if err := b.commit(); err != nil {
		return err
	}
	return nil
}

func (b *Backend) get(id sibra.ID) (*resvMeta, error) {
	rows, err := b.db.Query("SELECT RowID, Creation, SegID, LastUpdated  FROM SteadyResvs "+
		"WHERE ResvID=?", common.RawBytes(id))
	if err != nil {
		return nil, common.NewBasicError("Failed to lookup reservation", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m resvMeta
		var create int
		var lastUp int
		err = rows.Scan(&m.RowID, &create, &m.SegID, &lastUp)
		if err != nil {
			return nil, common.NewBasicError("Failed to extract data", err)
		}
		m.Creation = time.Unix(int64(create), 0)
		m.LastUpdated = time.Unix(int64(lastUp), 0)
		return &m, nil
	}
	return nil, nil
}

func (b *Backend) updateExisting(bmeta *sibra_mgmt.BlockMeta, rmeta *resvMeta) error {
	segID := bmeta.SegID()
	// Create new transaction
	if err := b.begin(); err != nil {
		return err
	}
	// The underlying segment has changed
	if bytes.Compare(segID, rmeta.SegID) != 0 {
		if err := b.replaceBlock(bmeta, rmeta.RowID); err != nil {
			b.tx.Rollback()
			return err
		}
	} else {
		if err := b.updateBlock(bmeta, rmeta.RowID); err != nil {
			b.tx.Rollback()
			return err
		}
	}
	// Commit transaction
	if err := b.commit(); err != nil {
		return err
	}
	return nil
}

func (b *Backend) replaceBlock(meta *sibra_mgmt.BlockMeta, row int64) error {
	_, err := b.tx.Exec("DELETE FROM SteadyResvs WHERE RowID=?", row)
	if err != nil {
		b.tx.Rollback()
		return common.NewBasicError("Failed to delete during replace", err)
	}
	return b.insertFull(meta)
}

func (b *Backend) updateBlock(meta *sibra_mgmt.BlockMeta, row int64) error {
	stmtStr := `UPDATE SteadyResvs SET Creation=?, Signature=?, WhiteList=?, LastUpdated=?, 
		Block=? WHERE RowID=?`
	_, err := b.tx.Exec(stmtStr, meta.RawCreation, meta.Signature, meta.WhiteList, time.Now().Unix(),
		meta.RawBlock, row)
	if err != nil {
		return common.NewBasicError("Failed to update steady reservation", err)
	}
	return nil
}

func (b *Backend) insertFull(meta *sibra_mgmt.BlockMeta) error {
	// Insert steady reservation.
	inst := `INSERT INTO SteadyResvs (ResvID, Block, Creation, Interfaces, Signature, WhiteList, 
		Mtu, SegID, LastUpdated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := b.tx.Exec(inst, common.RawBytes(meta.Id), meta.RawBlock, meta.RawCreation,
		meta.RawInterfaces, meta.Signature, meta.WhiteList, meta.Mtu, meta.SegID(),
		time.Now().Unix())
	if err != nil {
		return common.NewBasicError("Failed to insert path steady reservation", err)
	}
	rowID, err := res.LastInsertId()
	if err != nil {
		return common.NewBasicError("Failed to retrieve rowID of inserted steady reservation", err)
	}
	// Insert all interfaces.
	if err = b.insertInterfaces(meta.Interfaces, rowID); err != nil {
		return err
	}
	// Insert ISD-AS to StartsAt.
	if err = b.insertStartOrEnd(meta.StartIA(), rowID, StartsAtTable); err != nil {
		return err
	}
	// Insert ISD-AS to EndsAt.
	if err = b.insertStartOrEnd(meta.EndIA(), rowID, EndsAtTable); err != nil {
		return err
	}
	return nil
}

func (b *Backend) insertInterfaces(intfs []sibra_mgmt.PathInterface, rowID int64) error {
	for _, intf := range intfs {
		if intf.IfID == 0 {
			continue
		}
		ia := intf.ISD_AS()
		stmtStr := `INSERT INTO IntfToSeg (IsdID, ASID, IntfID, SteadyRowID) VALUES (?, ?, ?, ?)`
		_, err := b.tx.Exec(stmtStr, ia.I, ia.A, intf.IfID, rowID)
		if err != nil {
			return common.NewBasicError("Failed to insert into IntfToSeg", err)
		}
	}
	return nil
}

func (b *Backend) insertStartOrEnd(ia addr.IA, rowID int64,
	tableName string) error {
	stmtStr := "INSERT INTO " + tableName + " (IsdID, AsID, SteadyRowID) VALUES (?, ?, ?)"
	_, err := b.tx.Exec(stmtStr, ia.I, ia.A, rowID)
	if err != nil {
		return common.NewBasicError(fmt.Sprintf("Failed to insert into %s", tableName), err)
	}
	return nil
}

func (b *Backend) Delete(resvID sibra.ID) (int, error) {
	b.Lock()
	defer b.Unlock()
	if b.db == nil {
		return 0, common.NewBasicError("No database open", nil)
	}
	// Create new transaction
	if err := b.begin(); err != nil {
		return 0, err
	}
	res, err := b.tx.Exec("DELETE FROM SteadyResvs WHERE ResvID=?", common.RawBytes(resvID))
	if err != nil {
		b.tx.Rollback()
		return 0, common.NewBasicError("Failed to delete segment", err)
	}
	// Commit transaction
	if err := b.commit(); err != nil {
		return 0, err
	}
	deleted, _ := res.RowsAffected()
	return int(deleted), nil
}

func (b *Backend) DeleteWithIntf(intf query.IntfSpec) (int, error) {
	b.Lock()
	defer b.Unlock()
	if b.db == nil {
		return 0, common.NewBasicError("No database open", nil)
	}
	// Create new transaction
	if err := b.begin(); err != nil {
		return 0, err
	}
	delStmt := `DELETE FROM SteadyResvs WHERE EXISTS (
		SELECT * FROM IntfToSeg WHERE IsdID=? AND AsID=? AND IntfID=?)`
	res, err := b.tx.Exec(delStmt, intf.IA.I, intf.IA.A, intf.IfID)
	if err != nil {
		b.tx.Rollback()
		return 0, common.NewBasicError("Failed to delete segments", err)
	}
	// Commit transaction
	if err := b.commit(); err != nil {
		return 0, err
	}
	deleted, _ := res.RowsAffected()
	return int(deleted), nil
}

func (b *Backend) Get(params *query.Params) ([]*query.Result, error) {
	b.RLock()
	defer b.RUnlock()
	if b.db == nil {
		return nil, common.NewBasicError("No database open", nil)
	}
	stmt := b.buildQuery(params)
	rows, err := b.db.Query(stmt)
	if err != nil {
		return nil, common.NewBasicError("Error looking up path segment", err, "q", stmt)
	}
	defer rows.Close()
	res := []*query.Result{}
	prevID := -1
	var curRes *query.Result
	for rows.Next() {
		bmeta := &sibra_mgmt.BlockMeta{}
		var rowID int
		err = rows.Scan(&rowID, &bmeta.Id, &bmeta.RawBlock, &bmeta.RawCreation,
			&bmeta.RawInterfaces, &bmeta.Signature, &bmeta.WhiteList, &bmeta.Mtu)
		if err != nil {
			return nil, common.NewBasicError("Error reading DB response", err)
		}
		// Check if we have a new reservation.
		if rowID != prevID {
			if curRes != nil {
				res = append(res, curRes)
			}
			curRes = &query.Result{
				BlockMeta: bmeta,
			}
			if err := bmeta.ParseRaw(); err != nil {
				return nil, common.NewBasicError("Error unmarshalling blockmeta", err)
			}
		}
		prevID = rowID
	}
	if curRes != nil {
		res = append(res, curRes)
	}
	return res, nil
}

func (b *Backend) buildQuery(params *query.Params) string {
	query := []string{
		"SELECT DISTINCT s.RowID, s.ResvID, s.Block, s.Creation, s.Interfaces, " +
			"s.Signature, s.WhiteList, s.Mtu FROM SteadyResvs s",
	}

	if params == nil {
		return strings.Join(query, "\n")
	}
	joins := []string{}
	where := []string{}
	if len(params.ResvID) > 0 {
		where = append(where, fmt.Sprintf("s.ResvID=x'%s'", common.RawBytes(params.ResvID)))
	}
	if len(params.SegID) > 0 {
		where = append(where, fmt.Sprintf("s.SegID=x'%s'", params.SegID))
	}
	if len(params.Intfs) > 0 {
		joins = append(joins, "JOIN IntfToSeg i ON i.SteadyRowID=s.RowID")
		subQ := []string{}
		for _, spec := range params.Intfs {
			subQ = append(subQ, fmt.Sprintf("(i.IsdID='%d' AND i.AsID='%d' AND i.IntfID='%d')",
				spec.IA.I, spec.IA.A, spec.IfID))
		}
		where = append(where, fmt.Sprintf("(%s)", strings.Join(subQ, " OR ")))
	}
	if len(params.StartsAt) > 0 {
		joins = append(joins, "JOIN StartsAt st ON st.SteadyRowID=s.RowID")
		subQ := []string{}
		for _, as := range params.StartsAt {
			subQ = append(subQ, fmt.Sprintf("(st.IsdID='%d' AND st.AsID='%d')", as.I, as.A))
		}
		where = append(where, fmt.Sprintf("(%s)", strings.Join(subQ, " OR ")))
	}
	if len(params.EndsAt) > 0 {
		joins = append(joins, "JOIN EndsAt e ON e.SteadyRowID=s.RowID")
		subQ := []string{}
		for _, as := range params.EndsAt {
			subQ = append(subQ, fmt.Sprintf("(e.IsdID='%d' AND e.AsID='%d')", as.I, as.A))
		}
		where = append(where, fmt.Sprintf("(%s)", strings.Join(subQ, " OR ")))
	}
	// Assemble the query.
	if len(joins) > 0 {
		query = append(query, strings.Join(joins, "\n"))
	}
	if len(where) > 0 {
		query = append(query, fmt.Sprintf("WHERE %s", strings.Join(where, " AND\n")))
	}
	return strings.Join(query, "\n")
}
