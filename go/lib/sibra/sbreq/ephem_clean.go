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
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

const (
	offFlags = common.LineLen - 1

	flagSetup = 0x01
)

var _ Request = (*EphemClean)(nil)

// EphemClean is the request to cleanup unsuccessful ephemeral reservations.
// In case the cleanup request is for a setup reservation, the request contains
// the reservation id.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | base   |          padding                        			  | flags  |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Ephemeral ID (opt)													   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Info												                   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type EphemClean struct {
	*Base
	// ReqID is the requested ephemeral id in failed setup requests.
	ReqID sibra.ID
	// Info is the reservation info of the failed request.
	Info *sbresv.Info
	// numHops keeps track of how many hops there are.
	numHops int
	// Setup indicates if this is a cleanup message for a failed setup request.
	Setup bool
}

func EphemCleanFromRaw(raw common.RawBytes, numHops int) (*EphemClean, error) {
	b, err := BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return EphemCleanFromBase(b, raw, numHops)
}

func EphemCleanFromBase(b *Base, raw common.RawBytes, numHops int) (*EphemClean, error) {
	if b.Type != REphmCleanUp {
		return nil, common.NewBasicError("Invalid request type", nil,
			"expected", REphmCleanUp, "actual", b.Type)
	}
	l := 2 * common.LineLen
	if len(raw) < l {
		return nil, common.NewBasicError("Invalid ephemeral cleanup request length", nil,
			"min", l, "actual", len(raw))
	}
	c := &EphemClean{
		Base:    b,
		numHops: numHops,
		Setup:   (raw[offFlags] & flagSetup) != 0,
	}
	if c.Setup {
		l += sibra.EphemIDLen
	}
	if len(raw) != l {
		return nil, common.NewBasicError("Invalid ephemeral reservation reply length", nil,
			"expected", int(raw[offEphmLen])*common.LineLen, "actual", len(raw))
	}
	off := common.LineLen
	if c.Setup {
		c.ReqID = sibra.ID(raw[off : off+sibra.EphemIDLen])
		off += sibra.EphemIDLen
	}
	c.Info = sbresv.NewInfoFromRaw(raw[off : off+sbresv.InfoLen])
	return c, nil
}

// NewEphemClean creates a new request to clean up a failed reservation.
// To clean up a failed setup, a reservation id has to be provided. To clean up a failed
// ephemeral reservation, id must be nil.
func NewEphemClean(id sibra.ID, info *sbresv.Info, numhops int) *EphemClean {
	c := &EphemClean{
		Base: &Base{
			Type:     REphmCleanUp,
			Accepted: true,
		},
		ReqID:   id,
		Info:    info,
		numHops: numhops,
		Setup:   id != nil,
	}
	return c
}

func (c *EphemClean) EphemID() sibra.ID {
	return c.ReqID
}

func (c *EphemClean) Steady() bool {
	return false
}

func (c *EphemClean) NumHops() int {
	return c.numHops
}

func (c *EphemClean) Len() int {
	if c.Setup {
		return common.LineLen + sbresv.InfoLen + sibra.EphemIDLen
	}
	return common.LineLen + sbresv.InfoLen
}

func (c *EphemClean) Write(b common.RawBytes) error {
	if len(b) < c.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.EphemClean.Write",
			"min", c.Len(), "actual", len(b))
	}
	if err := c.Base.Write(b); err != nil {
		return err
	}
	b[offFlags] = c.packFlags()
	off, end := 0, common.LineLen
	if c.Setup {
		off, end = end, end+sibra.EphemIDLen
		c.ReqID.Write(b[off:end])
	}
	off, end = end, end+c.Info.Len()
	if err := c.Info.Write(b[off:end]); err != nil {
		return err
	}
	return nil
}

func (c *EphemClean) packFlags() uint8 {
	var flags uint8
	if c.Setup {
		flags |= flagSetup
	}
	return flags
}

func (c *EphemClean) Reverse() (Request, error) {
	c.Response = !c.Response
	return c, nil
}

func (c *EphemClean) String() string {
	return fmt.Sprintf("Base: [%s] Setup: %t Info: [%s]", c.Base, c.Setup, c.Info)

}
