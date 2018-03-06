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

package sibra

import "github.com/scionproto/scion/go/lib/common"

const (
	// Version is the SIBRA Version number.
	// It is a 2bit value to be used as (SCION ver, SIBRA ver).
	Version = 0

	// macInputLen is the input length for SIBRA opaque field MAC computation.
	// sum of len(Ingress), len(Egress), len(Info), maxPathIDsLen, len(prev mac), padding.
	macInputLen = 2 + 2 + infoLen + maxPathIDsLen + MacLen + 10
	// MacLen is the SIBRA opaque field MAC length
	MacLen = 4
	// maxPathIDsLen is the maximum space required to write all path ids.
	maxPathIDsLen = 3*SteadyIDLen + EphemeralIDLen

	// bwPairLen is the BWPair length.
	bwPairLen = 2
	// infoLen is the Info length.
	infoLen = common.LineLen
	// opFieldLen is the OpField length.
	opFieldLen = common.LineLen

	// EphemeralIDLen is the ephemeral path id length.
	EphemeralIDLen = 16
	// SteadyIDLen is the steady path id length
	SteadyIDLen = 8
)

type Tick uint32

type BWPair struct {
	Fwd BWClass
	Rev BWClass
}

func BWPairFromRaw(raw common.RawBytes) BWPair {
	return BWPair{Fwd: BWClass(raw[0]), Rev: BWClass(raw[1])}
}

type BWClass uint8

type Index uint8

type PathID common.RawBytes

func IndexFromUint8(b uint8) Index {
	return Index(b >> 4)
}

func (i Index) ToUint8() uint8 {
	return uint8(i) << 4
}

type Interface uint16
