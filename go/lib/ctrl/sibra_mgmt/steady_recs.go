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
	"github.com/scionproto/scion/go/proto"
)

var _ proto.Cerealizable = (*SteadyRecs)(nil)

type SteadyRecs struct {
	Entries []*BlockMeta
}

func (s *SteadyRecs) ParseRaw() error {
	for i, entry := range s.Entries {
		if err := entry.ParseRaw(); err != nil {
			return common.NewBasicError("Unable to parse entry", err, "entry_idx", i)
		}
	}
	return nil
}

func (s *SteadyRecs) ProtoId() proto.ProtoIdType {
	return proto.SibraSteadyRecs_TypeID
}

func (s *SteadyRecs) String() string {
	return fmt.Sprintf("SteadyRecs len: %d", len(s.Entries))
}

var _ proto.Cerealizable = (*SteadyReg)(nil)

type SteadyReg struct {
	*SteadyRecs
}
