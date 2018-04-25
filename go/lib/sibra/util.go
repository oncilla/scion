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

import (
	"fmt"

	"github.com/scionproto/scion/go/lib/common"
)

const (
	BufferToShort = "Buffer to short"

	// macInputLen is the input length for SIBRA opaque field MAC computation.
	// sum of len(Ingress), len(Egress), len(Info), maxPathIDsLen, len(prev mac), padding.
	macInputLen = 2 + 2 + infoLen + maxPathIDsLen + MacLen + 10
	// MacLen is the SIBRA opaque field MAC length
	MacLen = 4
	// maxPathIDsLen is the maximum space required to write all path ids.
	maxPathIDsLen = 3*SteadyIDLen + EphemeralIDLen

	// EphemeralIDLen is the ephemeral path id length.
	EphemeralIDLen = 16
)

// PacketType indicates the type of packet that is sent.
type PacketType uint8

const (
	PacketTypeData PacketType = iota
	PacketTypeSetup
	PacketTypeRenewal
	PacketTypeTearDown
)

func (t PacketType) String() string {
	switch t {
	case PacketTypeData:
		return "Data"
	case PacketTypeSetup:
		return "Setup"
	case PacketTypeRenewal:
		return "Renewal"
	case PacketTypeTearDown:
		return "Teardown"
	}
	return fmt.Sprintf("UNKNOWN (%d)", t)
}

// PathType indicates the type of path the packet is sent on.
type PathType uint8

const (
	PathTypeDown PathType = iota
	PathTypeUp
	PathTypePeerDown
	PathTypePeerUp
	PathTypeEphemeral
	PathTypeCore
)

func (t PathType) String() string {
	switch t {
	case PathTypeDown:
		return "Down"
	case PathTypeUp:
		return "Up"
	case PathTypePeerDown:
		return "Peering-Down"
	case PathTypePeerUp:
		return "Peering-Up"
	case PathTypeEphemeral:
		return "Ephemeral"
	case PathTypeCore:
		return "Core"
	}
	return fmt.Sprintf("UNKNOWN (%d)", t)
}

// GenFwd indicates if the SIBRA Opaque field are generated in the forward direction.
func (t PathType) GenFwd() bool {
	return (t & 0x1) == 0
}

type ResvID common.RawBytes

func (p ResvID) Write(b common.RawBytes) error {
	if len(b) < len(p) {
		return common.NewBasicError(BufferToShort, nil, "method", "ResvID.Write",
			"min", len(p), "actual", len(b))
	}
	copy(b, p)
	return nil
}

func (p ResvID) Len() int {
	return len(p)
}

type Tick uint32

type BwClass uint8

type RttClass uint8

type Index uint8

type Interface uint16
