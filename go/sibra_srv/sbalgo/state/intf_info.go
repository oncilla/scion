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

package state

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
)

type IFState struct {
	Total     sibra.Bps
	Reserved  sibra.Bps
	Unlimited bool
}

func (b *IFState) SetTotal(total sibra.Bps) error {
	if total < b.Reserved {
		return common.NewBasicError("New total smaller than reserved", nil,
			"reserved", b.Reserved, "total", total)
	}
	b.Total = total
	b.Unlimited = false
	return nil
}

// Alloc tries allocating bandwidth. If successful it returns the allocated value
// and true. If Unsuccessful it returns the max value and false.
func (b *IFState) Alloc(bw sibra.Bps) error {
	if bw < 0 {
		panic("Negative bw allocation not permitted")
	}
	if free := b.Free(); free < bw && !b.Unlimited {
		return common.NewBasicError("Not enough bandwidth", nil)
	}
	b.Reserved += bw
	return nil
}

func (b *IFState) Free() sibra.Bps {
	return b.Total - b.Reserved
}

func (b *IFState) Dealloc(bw sibra.Bps) error {
	if bw < 0 {
		panic("Negative bw deallocation not permitted")
	}
	if bw > b.Reserved {
		return common.NewBasicError("Unable to dealloc more bandwidth than reserved", nil,
			"max", b.Reserved, "actual", bw)
	}
	b.Reserved -= bw
	return nil
}

type IFInfo struct {
	Ingress *IFState
	Egress  *IFState
}
