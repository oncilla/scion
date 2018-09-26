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
	"sync"

	"time"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

type EphemResvEntry struct {
	sync.RWMutex
	// Id is the reservation ID.
	Id sibra.ID
	// ActiveIdx is the currently active index.
	ActiveIdx EphemResvIdx
	// LastIdx is the index before the currently active index.
	// This is used during clean-up and re-requesting a failed index.
	LastIdx EphemResvIdx
	// TODO(roosd)
	SteadyEntry *SteadyResvEntry
}

func (e *EphemResvEntry) Expired(now time.Time) bool {
	e.RLock()
	defer e.RUnlock()
	return !e.ActiveIdx.Active(now)
}

func (e *EphemResvEntry) AddIdx(info *sbresv.Info, alloc uint64) error {
	e.Lock()
	defer e.Unlock()
	if !e.ActiveIdx.Active(time.Now()) {
		return common.NewBasicError("Inactive reservation", nil)
	}
	if e.ActiveIdx.Info.Index.Add(1) != info.Index {
		return common.NewBasicError("Non consecutive index", nil,
			"expected", e.ActiveIdx.Info.Index.Add(1), "actual", info.Index)
	}
	e.LastIdx = e.ActiveIdx
	e.ActiveIdx = EphemResvIdx{
		Info:      *info,
		Allocated: alloc,
	}
	return nil
}

func (e *EphemResvEntry) ReplaceIdx(newInfo, lastInfo *sbresv.Info, alloc uint64) (sbresv.Info, error) {
	if !e.ActiveIdx.Active(time.Now()) {
		return sbresv.Info{}, common.NewBasicError("Inactive reservation", nil)
	}
	if *lastInfo != e.LastIdx.Info {
		return sbresv.Info{}, common.NewBasicError("Last index does not match", nil,
			"expected", e.LastIdx.Info, "actual", *lastInfo)
	}
	if (e.LastIdx.Info.Index+1)%sibra.NumIndexes != newInfo.Index {
		return sbresv.Info{}, common.NewBasicError("Non consecutive index", nil,
			"expected", (e.LastIdx.Info.Index+1)%sibra.NumIndexes, "actual", newInfo.Index)
	}
	replaced := e.ActiveIdx.Info
	e.ActiveIdx = EphemResvIdx{
		Info:      *newInfo,
		Allocated: alloc,
	}
	return replaced, nil
}

func (e *EphemResvEntry) CleanUpIdx(failedInfo, lastInfo *sbresv.Info) (sbresv.Info, error) {
	if !e.ActiveIdx.Active(time.Now()) {
		return sbresv.Info{}, common.NewBasicError("Inactive reservation", nil)
	}
	if *lastInfo != e.LastIdx.Info {
		return sbresv.Info{}, common.NewBasicError("Last index does not match", nil,
			"expected", &e.LastIdx.Info, "actual", *lastInfo)
	}
	if !validateFailedInfo(failedInfo, &e.ActiveIdx.Info) {
		return sbresv.Info{}, common.NewBasicError("Failed index does not match", nil,
			"expected", &e.ActiveIdx.Info, "actual", failedInfo)
	}
	cleaned := e.ActiveIdx.Info
	e.ActiveIdx = e.LastIdx
	// FIXME(roosd): This might be abused with two clean-ups.
	e.LastIdx = EphemResvIdx{}
	return cleaned, nil
}

func validateFailedInfo(failedInfo, local *sbresv.Info) bool {
	return failedInfo.ExpTick == local.ExpTick && failedInfo.RLC == local.RLC &&
		failedInfo.Index == local.Index && failedInfo.PathType == local.PathType
}

type EphemResvIdx struct {
	// TODO(roosd): comments
	Info sbresv.Info
	// TODO(roosd): comment
	Allocated uint64
}

func (i *EphemResvIdx) Active(t time.Time) bool {
	return t.Before(i.Info.ExpTick.Time())
}
