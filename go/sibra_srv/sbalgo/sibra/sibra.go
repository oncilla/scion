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
	"sync"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

type Algo interface {
	sync.Locker
	SteadyAdm
	EphemAdm
}

type IFTuple struct {
	InIfid common.IFIDType
	EgIfid common.IFIDType
}

func (i IFTuple) Reverse() IFTuple {
	return IFTuple{
		InIfid: i.EgIfid,
		EgIfid: i.InIfid,
	}
}

type AdmParams struct {
	Ifids IFTuple
	Extn  *sbextn.Steady
	Req   *sbreq.SteadyReq
	Src   addr.IA
}

type CleanParams struct {
	Ifids   IFTuple
	Src     addr.IA
	Id      sbresv.ID
	LastMax sbresv.Bps
	CurrMax sbresv.Bps
	Dealloc sbresv.Bps
	Remove  bool
}

type SteadyAdm interface {
	AdmitSteady(params AdmParams) (SteadyRes, error)
	Ideal(params AdmParams) sbresv.Bps
	Available(ifids IFTuple, id sbresv.ID) sbresv.Bps
	AddSteadyResv(params AdmParams, alloc sbresv.BwCls) error
	CleanSteadyResv(c CleanParams)
	PromoteToSOFCreated(ifids IFTuple, id sbresv.ID, info *sbresv.Info) error
	PromoteToPending(ifids IFTuple, id sbresv.ID, c *sbreq.ConfirmIndex) error
	PromoteToActive(ifids IFTuple, id sbresv.ID, info *sbresv.Info, c *sbreq.ConfirmIndex) error
}

type EphemAdm interface {
	AdmitEphemSetup(steady *sbextn.Steady) (EphemRes, error)
	AdmitEphemRenew(ephem *sbextn.Ephemeral) (EphemRes, error)
	CleanEphemSetup(steady *sbextn.Steady) error
	CleanEphemRenew(ephem *sbextn.Ephemeral) error
}

type SteadyRes struct {
	// AllocBw is the allocated bandwidth
	AllocBw sbresv.BwCls
	// MaxBw is the maximum acceptable bandwidth in case admission fails.
	MaxBw sbresv.BwCls
	// MinBw is the minimal acceptable bandwidth in case admission fails.
	MinBw sbresv.BwCls
	// Accepted indicates if the reservation is accepted.
	Accepted bool
}

type EphemRes struct {
	// AllocBw is the allocated bandwidth
	AllocBw sbresv.BwCls
	// MaxBw is the maximum acceptable bandwidth in case admission fails.
	MaxBw sbresv.BwCls
	// FailCode indicates the failure code when admission fails.
	FailCode sbreq.FailCode
}
