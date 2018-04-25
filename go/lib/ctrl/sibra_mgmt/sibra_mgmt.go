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
	"strings"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/proto"
)

type union struct {
	Which        proto.SibraMgmt_Which
	ExternPkt    *ExternalPkt  `capnp:"sibraExternalPkt"`
	EphemReq     *EphemReq     `capnp:"sibraEphemReq"`
	EphemRep     *EphemRep     `capnp:"sibraEphemRep"`
	Instruct     *Instruct     `capnp:"sibraInstruct"`
	SteadyReq    *SteadyReq    `capnp:"sibraSteadyReq"`
	SteadyReg    *SteadyReg    `capnp:"sibraSteadyReg"`
	SteadyRegRep *SteadyRegRep `capnp:"sibraSteadyRegRep"`
	SteadyRep    *SteadyRep    `capnp:"sibraSteadyRep"`
}

func (u *union) set(c proto.Cerealizable) error {
	switch p := c.(type) {
	case *ExternalPkt:
		u.Which = proto.SibraMgmt_Which_sibraExternalPkt
		u.ExternPkt = p
	case *EphemReq:
		u.Which = proto.SibraMgmt_Which_sibraEphemReq
		u.EphemReq = p
	case *EphemRep:
		u.Which = proto.SibraMgmt_Which_sibraEphemRep
		u.EphemRep = p
	case *Instruct:
		u.Which = proto.SibraMgmt_Which_sibraInstruct
		u.Instruct = p
	case *SteadyReq:
		u.Which = proto.SibraMgmt_Which_sibraSteadyReq
		u.SteadyReq = p
	case *SteadyRep:
		u.Which = proto.SibraMgmt_Which_sibraSteadyRep
		u.SteadyRep = p
	case *SteadyReg:
		u.Which = proto.SibraMgmt_Which_sibraSteadyReg
		u.SteadyReg = p
	case *SteadyRegRep:
		u.Which = proto.SibraMgmt_Which_sibraSteadyRegRep
		u.SteadyRegRep = p
	default:
		return common.NewBasicError("Unsupported sibra mgmt union type (set)", nil,
			"type", common.TypeOf(c))
	}
	return nil
}

func (u *union) get() (proto.Cerealizable, error) {
	switch u.Which {
	case proto.SibraMgmt_Which_sibraExternalPkt:
		return u.ExternPkt, nil
	case proto.SibraMgmt_Which_sibraEphemReq:
		return u.EphemReq, nil
	case proto.SibraMgmt_Which_sibraEphemRep:
		return u.EphemRep, nil
	case proto.SibraMgmt_Which_sibraInstruct:
		return u.Instruct, nil
	case proto.SibraMgmt_Which_sibraSteadyReq:
		return u.SteadyReq, nil
	case proto.SibraMgmt_Which_sibraSteadyRep:
		return u.SteadyRep, nil
	case proto.SibraMgmt_Which_sibraSteadyReg:
		return u.SteadyReg, nil
	case proto.SibraMgmt_Which_sibraSteadyRegRep:
		return u.SteadyRegRep, nil
	}
	return nil, common.NewBasicError("Unsupported sibr mgmt union type (get)", nil, "type", u.Which)
}

var _ proto.Cerealizable = (*Pld)(nil)

type Pld struct {
	union
	*Data
}

// NewPld creates a new sibra mgmt payload, containing the supplied Cerealizable instance.
func NewPld(u proto.Cerealizable, d *Data) (*Pld, error) {
	p := &Pld{Data: d}
	return p, p.union.set(u)
}

func (p *Pld) Union() (proto.Cerealizable, error) {
	return p.union.get()
}

func (p *Pld) ProtoId() proto.ProtoIdType {
	return proto.SibraMgmt_TypeID
}

func (p *Pld) String() string {
	desc := []string{"SibraMgmt: Union:"}
	u, err := p.Union()
	if err != nil {
		desc = append(desc, err.Error())
	} else {
		desc = append(desc, fmt.Sprintf("%+v", u))
	}
	return strings.Join(desc, " ")
}

type Data struct {
	// For passing any future non-union data.
}
