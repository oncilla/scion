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
)

type BWProvider struct {
	sync.RWMutex
	// Total is the total amount of bandwidth (in bps).
	Total uint64
	// Reserved is the reserved amount of bandwidth (in bps).
	Reserved uint64
	// deallocRing is used to deallocate expired bandwidth
	deallocRing deallocRing
}

func (b *BWProvider) SetTotal(total uint64) error {
	b.Lock()
	defer b.Unlock()
	b.cleanUp(time.Now())
	if total < b.Reserved {
		return common.NewBasicError("New total smaller than reserved", nil,
			"reserved", b.Reserved, "total", total)
	}
	b.Total = total
	return nil
}

// Alloc tries allocating bandwidth. If successful it returns the allocated value
// and true. If Unsuccessful it returns the max value and false.
func (b *BWProvider) Alloc(bw uint64) (uint64, bool) {
	b.Lock()
	defer b.Unlock()
	return b.alloc(bw, time.Now())
}

// AllocExpiring tries allocating expiring bandwidth. If successful it returns the
// allocated value and true. If Unsuccessful it returns the max value and false.
// An error is thrown if the expTick is outside the range of the dealloc ring.
func (b *BWProvider) AllocExpiring(bw uint64, expTick sibra.Tick) (uint64, bool, error) {
	b.Lock()
	defer b.Unlock()
	bw, ok := b.alloc(bw, time.Now())
	if !ok {
		return bw, false, nil
	}
	if err := b.deallocRing.reserve(expTick, bw); err != nil {
		b.Reserved -= bw
		return 0, false, err
	}
	return bw, true, nil
}

func (b *BWProvider) ExchangeExpiring(newBw, oldBw uint64, newTick, oldTick sibra.Tick) (
	uint64, bool, error) {

	b.Lock()
	defer b.Unlock()
	var additional uint64
	allocated := oldBw
	if newBw > oldBw {
		additional = newBw - oldBw
		allocated = newBw
		if bw, ok := b.alloc(additional, time.Now()); !ok {
			return bw + oldBw, false, nil
		}
	} else {
		b.cleanUp(time.Now())
	}
	if err := b.deallocRing.exchange(newTick, oldTick, allocated, oldBw); err != nil {
		b.Reserved -= additional
		return 0, false, err
	}
	return allocated, true, nil
}

func (b *BWProvider) alloc(bw uint64, t time.Time) (uint64, bool) {
	free := b.free(t)
	if free < bw {
		return free, false
	}
	b.Reserved += bw
	return bw, true
}

func (b *BWProvider) Free() uint64 {
	b.RLock()
	defer b.RUnlock()
	return b.free(time.Now())
}

func (b *BWProvider) free(t time.Time) uint64 {
	b.cleanUp(t)
	return b.Total - b.Reserved
}

func (b *BWProvider) Dealloc(bw uint64) error {
	b.Lock()
	defer b.Unlock()
	return b.dealloc(bw)
}

func (b *BWProvider) DeallocExpiring(bw uint64, expTick sibra.Tick) error {
	b.Lock()
	defer b.Unlock()
	if err := b.deallocRing.unreserve(expTick, bw); err != nil {
		return err
	}
	return b.dealloc(bw)
}

// UndoExchangeExpiring undoes the operations carried out by ExchangeExpiring. The parameters must
// be the same as provided to ExchangeExpiring.
func (b *BWProvider) UndoExchangeExpiring(newBw, oldBw uint64, newTick, oldTick sibra.Tick) error {

	b.Lock()
	defer b.Unlock()
	var additional uint64
	allocated := oldBw
	if newBw > oldBw {
		additional = newBw - oldBw
		allocated = newBw
	}
	if err := b.deallocRing.exchange(oldTick, newTick, oldBw, allocated); err != nil {
		return err
	}
	return b.dealloc(additional)
}

func (b *BWProvider) dealloc(bw uint64) error {
	if bw > b.Reserved {
		return common.NewBasicError("Unable to dealloc more bandwidth than reserved", nil,
			"max", b.Reserved, "actual", bw)
	}
	b.Reserved -= bw
	return nil
}

func (b *BWProvider) CleanUp() {
	b.Lock()
	defer b.Unlock()
	b.cleanUp(time.Now())
}

func (b *BWProvider) cleanUp(t time.Time) {
	if b.deallocRing.freeRing == nil {
		return
	}
	b.Reserved -= b.deallocRing.cleanUp(sibra.TimeToTick(t))
}

type deallocRing struct {
	// offset indicates the offset in the ring.
	offset int
	// currTick indicates the SIBRA tick offset points to.
	currTick sibra.Tick
	// freeRing indicates the amount of bandwidth that is freed at the respective SIBRA tick.
	freeRing []uint64
}

func (r *deallocRing) cleanUp(newCurr sibra.Tick) (dealloc uint64) {
	diff := newCurr.Sub(r.currTick)
	if diff < 1 {
		return 0
	} else if diff > len(r.freeRing) {
		diff = len(r.freeRing)
	}
	for i := 0; i < diff; i++ {
		r.offset = (r.offset + 1) % len(r.freeRing)
		dealloc += r.freeRing[r.offset]
		r.freeRing[r.offset] = 0
	}
	r.currTick = newCurr
	return dealloc
}

func (r *deallocRing) reserve(deallocTick sibra.Tick, bw uint64) error {
	off, err := r.tickOffset(deallocTick)
	if err != nil {
		return err
	}
	r.freeRing[off] += bw
	return nil
}

func (r *deallocRing) unreserve(deallocTick sibra.Tick, bw uint64) error {
	off, err := r.tickOffset(deallocTick)
	if err != nil {
		return err
	}
	if r.freeRing[off] < bw {
		return common.NewBasicError("Dealloc more than reserved", nil,
			"max", r.freeRing[off], "actual", bw)
	}
	r.freeRing[off] -= bw
	return nil
}

func (r *deallocRing) exchange(newTick, oldTick sibra.Tick, newBw, oldBw uint64) error {
	newOff, err := r.tickOffset(newTick)
	if err != nil {
		return err
	}
	oldOff, err := r.tickOffset(oldTick)
	if r.freeRing[oldOff] < oldBw {
		return common.NewBasicError("Dealloc more than reserved", nil,
			"max", r.freeRing[oldOff], "actual", oldBw)
	}
	r.freeRing[oldOff] -= oldBw
	r.freeRing[newOff] += newBw
	return nil
}

func (r *deallocRing) tickOffset(deallocTick sibra.Tick) (int, error) {
	diff := deallocTick.Sub(r.currTick)
	if diff > len(r.freeRing) {
		return 0, common.NewBasicError("Dealloc tick to far in future", nil,
			"maxTick", r.currTick.Add(len(r.freeRing)), "actual", deallocTick)
	}
	if diff <= 0 {
		return 0, common.NewBasicError("Dealloc tick already passed", nil,
			"currTick", r.currTick)
	}
	return (r.offset + diff) % len(r.freeRing), nil
}
