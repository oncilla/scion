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

package query

import (
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/ctrl/sibra_mgmt"
	"github.com/scionproto/scion/go/lib/sibra"
)

type IntfSpec struct {
	IA   addr.IA
	IfID uint64
}

type Params struct {
	ResvID   sibra.ID
	SegID    common.RawBytes
	Intfs    []*IntfSpec
	StartsAt []addr.IA
	EndsAt   []addr.IA
}

type Result struct {
	BlockMeta *sibra_mgmt.BlockMeta
}
