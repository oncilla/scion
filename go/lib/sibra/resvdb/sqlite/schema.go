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

const (
	// SchemaVersion is the version of the SQLite schema understood by this backend.
	// Whenever changes to the schema are made, this version number should be increased
	// to prevent data corruption between incompatible database schemas.
	SchemaVersion = 1
	// Schema is the SQLite database layout.
	Schema = `CREATE TABLE SteadyResvs (
		RowID INTEGER PRIMARY KEY AUTOINCREMENT,
		ResvID DATA UNIQUE NOT NULL,
		Block DATA NOT NULL,
		Creation INTEGER NOT NULL,
		Interfaces DATA NOT NULL,
		Signature DATA NOT NULL,
		WhiteList DATA NOT NULL,
		Mtu INTEGER NOT NULL,
		SegID DATA NOT NULL,
		LastUpdated INTEGER NOT NULL
	);
	CREATE TABLE IntfToSeg(
		IsdID INTEGER NOT NULL,
		AsID INTEGER NOT NULL,
		IntfID INTEGER NOT NULL,
		SteadyRowID INTEGER NOT NULL,
		FOREIGN KEY (SteadyRowID) REFERENCES SteadyResvs(RowID) ON DELETE CASCADE
	);
	CREATE TABLE StartsAt(
		IsdID INTEGER NOT NULL,
		AsID INTEGER NOT NULL,
		SteadyRowID INTEGER NOT NULL,
		FOREIGN KEY (SteadyRowID) REFERENCES SteadyResvs(RowID) ON DELETE CASCADE
	);
	CREATE TABLE EndsAt(
		IsdID INTEGER NOT NULL,
		AsID INTEGER NOT NULL,
		SteadyRowID INTEGER NOT NULL,
		FOREIGN KEY (SteadyRowID) REFERENCES SteadyResvs(RowID) ON DELETE CASCADE
	);`
	SteadyResvsTable = "SteadyResvs"
	IntfToSegTable   = "IntfToSeg"
	StartsAtTable    = "StartsAt"
	EndsAtTable      = "EndsAt"
)
