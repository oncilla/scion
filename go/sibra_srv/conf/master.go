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

package conf

import (
	"strconv"

	"github.com/scionproto/scion/go/lib/infra"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spkt"
)

type ExtPkt struct {
	Req    *infra.Request
	Addr   *snet.Addr
	Spkt   *spkt.ScnPkt
	Steady *sbextn.Steady
	Ephem  *sbextn.Ephemeral
	Pld    *sbreq.Pld
	Conf   *Conf
}

type NotifyKey struct {
	Id      sibra.ID
	Idx     sibra.Index
	ReqType sbreq.DataType
}

func (n *NotifyKey) String() string {
	return n.Id.String() + strconv.Itoa(int(n.Idx)) + n.ReqType.String()
}

type RepMaster interface {
	Run()
	Handle(pkt *ExtPkt) error
	Register(key *NotifyKey, c chan *ExtPkt) error
	Deregister(key *NotifyKey)
}
