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

package sbreq

import (
	"fmt"

	"github.com/scionproto/scion/go/lib/common"
)

type Data interface {
	Steady() bool
	Len() int
	Type() DataType
	Write(common.RawBytes) error
	Reverse() (Data, error)
	fmt.Stringer
}

// DataType indicates the type of the request.
type DataType uint8

const (
	RSteadySetup DataType = iota
	RSteadySetupTelescope
	RSteadyRenewal
	RSteadyTearDown
	RSteadyConfIndex
	RSteadyCleanUp
	REphmSetup
	REphmRenewal
	REphmCleanUp
)

// Steady indicates if the request is related to a steady reservation.
func (t DataType) Steady() bool {
	return t <= RSteadyCleanUp
}

func (t DataType) String() string {
	switch t {
	case RSteadySetup:
		return "Steady Setup"
	case RSteadySetupTelescope:
		return "Steady Setup Telescope"
	case RSteadyRenewal:
		return "Steady Renewal"
	case RSteadyTearDown:
		return "Steady Teardown"
	case RSteadyConfIndex:
		return "Index Confirmation"
	case RSteadyCleanUp:
		return "Steady Clean-Up"
	case REphmSetup:
		return "Ephemeral Setup"
	case REphmRenewal:
		return "Ephemeral Renewal"
	case REphmCleanUp:
		return "Ephemeral Clean-Up"
	}
	return fmt.Sprintf("UNKNOWN (%d)", t)
}

// FailCode indicates the reason a reservation failed
type FailCode uint8

const (
	// Precedence of errors in ascending order
	FailCodeNone FailCode = iota
	ClientDenied
	BwExceeded
	EphemExists
	EphemNotExists
	SteadyOutdated
	SteadyNotExists
	InvalidInfo
)

func (f FailCode) String() string {
	switch f {
	case FailCodeNone:
		return "None"
	case ClientDenied:
		return "Denied by client"
	case BwExceeded:
		return "Bandwidth exceeded"
	case EphemExists:
		return "Ephemeral already exists"
	case SteadyOutdated:
		return "Steady is outdated"
	case SteadyNotExists:
		return "Steady does not exist"
	case InvalidInfo:
		return "Invalid info"
	}
	return fmt.Sprintf("UNKNOWN(%d)", f)
}
