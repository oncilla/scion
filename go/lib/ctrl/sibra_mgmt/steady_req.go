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

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/proto"
)

var _ proto.Cerealizable = (*SteadyReq)(nil)

type SteadyReq struct {
	RawStartIA addr.IAInt `capnp:"startIA"`
	RawEndIA   addr.IAInt `capnp:"endIA"`
	SegID      common.RawBytes
	PathType   sbresv.PathType
}

func (s *SteadyReq) StartIA() addr.IA {
	return s.RawStartIA.IA()
}

func (s *SteadyReq) EndIA() addr.IA {
	return s.RawEndIA.IA()
}

func (s *SteadyReq) ProtoId() proto.ProtoIdType {
	return proto.SibraSteadyReq_TypeID
}

func (s *SteadyReq) String() string {
	return fmt.Sprintf("start %s end %s segID %s type %s",
		s.StartIA(), s.EndIA(), s.SegID, s.PathType)
}
