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

import "github.com/scionproto/scion/go/lib/common"

// Parse parses the sibra request contained in raw.
func Parse(raw common.RawBytes, numHops int) (Request, error) {
	b, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	switch b.Type {
	case RSteadySetup, RSteadyRenewal:
		return parseSteadyResv(b, raw, numHops)
	case RSteadyConfIndex:
		return ConfirmIndexFromBase(b, raw, numHops)
	case REphmSetup, REphmRenewal:
		return parseEphemResv(b, raw, numHops)
	case REphmCleanUp:
		return EphemCleanFromBase(b, raw, numHops)
	case RSteadyTearDown, RSteadyCleanUp:
		return nil, common.NewBasicError("Parsing not implemented", nil, "type", b.Type)
	default:
		return nil, common.NewBasicError("Unknown request type", nil, "type", b.Type)
	}
}

func parseSteadyResv(b *Base, raw common.RawBytes, numHops int) (Request, error) {
	if b.Response && b.Accepted {
		return SteadySuccFromBase(b, raw, numHops)
	}
	return SteadyReqFromBase(b, raw, numHops)
}

func parseEphemResv(b *Base, raw common.RawBytes, numHops int) (Request, error) {
	if b.Accepted {
		return EphemReqFromBase(b, raw, numHops)
	}
	return EphemFailedFromBase(b, raw, numHops)
}
