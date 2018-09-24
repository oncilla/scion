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
	"time"

	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/proto"
)

var _ proto.Cerealizable = (*Instruct)(nil)

type Instruct struct {
	ResvID     sibra.ID `capnp:"id"`
	RawExpTime uint64   `capnp:"expTime"`
}

func (i *Instruct) ExpTime() time.Time {
	return time.Unix(0, int64(i.RawExpTime*1000))
}

func (i *Instruct) SetExpTime(t time.Time) {
	i.RawExpTime = uint64(t.UnixNano()) / 1000
}

func (i *Instruct) ProtoId() proto.ProtoIdType {
	return proto.SibraInstruct_TypeID
}

func (i *Instruct) String() string {
	return fmt.Sprintf("ResvID: %s ExpTime: %s", i.ResvID, i.ExpTime())
}
