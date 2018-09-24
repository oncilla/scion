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
	offEphemCleanFlags  = common.LineLen - 1
	flagEphemCleanSetup = 0x01
)

var _ Data = (*EphemClean)(nil)

// EphemClean is the request to cleanup unsuccessful ephemeral reservations.
// In case the cleanup request is for a setup reservation, the request contains
// the reservation id.
//
// 0B       1        2        3        4        5        6        7
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | flags  | Ephemeral ID (opt)										   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | ...																   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
// | Info												                   |
// +--------+--------+--------+--------+--------+--------+--------+--------+
type EphemClean struct {
	// ID is the requested ephemeral id in failed setup requests.
	ID sibra.ID
	// Info is the reservation info of the failed request.
	Info *sbresv.Info
	// Setup indicates if this is a cleanup message for a failed setup request.
	Setup bool
}

func EphemCleanFromRaw(raw common.RawBytes) (*EphemClean, error) {
	if len(raw) < 1 {
		return nil, common.NewBasicError("Invalid ephemeral cleanup length", nil,
			"min", 1, "actual", len(raw))
	}

	c := &EphemClean{
		Setup: (raw[offEphemCleanFlags] & flagEphemCleanSetup) != 0,
	}
	l := 1 + sbresv.InfoLen
	if c.Setup {
		l += sibra.EphemIDLen
	}
	if len(raw) < l {
		return nil, common.NewBasicError("Invalid ephemeral cleanup length", nil,
			"min", l, "actual", len(raw))
	}
	off := 1
	if c.Setup {
		c.ID = sibra.ID(raw[off : off+sibra.EphemIDLen])
		off += sibra.EphemIDLen
	}
	c.Info = sbresv.NewInfoFromRaw(raw[off : off+sbresv.InfoLen])
	return c, nil
}

func (c *EphemClean) Steady() bool {
	return false
}

func (c *EphemClean) Len() int {
	if c.Setup {
		return 1 + sbresv.InfoLen + sibra.EphemIDLen
	}
	return 1 + sbresv.InfoLen
}

func (c *EphemClean) Type() DataType {
	return REphmCleanUp
}

func (c *EphemClean) Write(b common.RawBytes) error {
	if len(b) < c.Len() {
		return common.NewBasicError("Buffer to short", nil, "method", "sbreq.EphemClean.Write",
			"min", c.Len(), "actual", len(b))
	}
	b[offEphemCleanFlags] = c.packFlags()
	off, end := 0, common.LineLen
	if c.Setup {
		off, end = end, end+sibra.EphemIDLen
		c.ID.Write(b[off:end])
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
		flags |= flagEphemCleanSetup
	}
	return flags
}

func (c *EphemClean) Reverse() (Data, error) {
	r := &EphemClean{
		Setup: c.Setup,
		Info:  c.Info,
		ID:    c.ID,
	}
	return r, nil
}

func (c *EphemClean) String() string {
	return fmt.Sprintf("Setup: %t Info: [%s] ID %s", c.Setup, c.Info, c.ID)

}
