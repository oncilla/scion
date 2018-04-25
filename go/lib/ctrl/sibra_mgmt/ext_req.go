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

package sibra_mgmt

import (
	"fmt"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/hpkt"
	"github.com/scionproto/scion/go/lib/spkt"
	"github.com/scionproto/scion/go/proto"
)

var _ proto.Cerealizable = (*ExternalPkt)(nil)

type ExternalPkt struct {
	RawPkt common.RawBytes `capnp:"rpkt"`
}

func (c *ExternalPkt) ScnPkt() (*spkt.ScnPkt, error) {
	pkt := &spkt.ScnPkt{}
	err := hpkt.ParseScnPkt(pkt, c.RawPkt)
	return pkt, err
}

func (c *ExternalPkt) ProtoId() proto.ProtoIdType {
	return proto.SibraExternalPkt_TypeID
}

func (c *ExternalPkt) String() string {
	pkt, err := c.ScnPkt()
	if err != nil {
		return fmt.Sprintf("Invalid ScnPkt: %v", err)
	}
	return fmt.Sprintf("%v", pkt)
}

var _ proto.Cerealizable = (*EphemReq)(nil)

type EphemReq struct {
	*ExternalPkt
}

var _ proto.Cerealizable = (*EphemRep)(nil)

type EphemRep struct {
	*ExternalPkt
}
