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
	"fmt"
	"sort"
	"strconv"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

func Test_BWProvider(t *testing.T) {
	Convey("Reservation is kept correctly", t, func() {
		Convey("Add and remove expiring", func() {
			currTick := sbresv.CurrentTick()
			b := &BWProvider{
				Total:    100,
				Reserved: 0,
				deallocRing: deallocRing{
					currTick: currTick,
					freeRing: make([]uint64, sbresv.MaxEphemTicks*2),
				},
			}
			allocated, ok, err := b.AllocExpiring(50, currTick.Add(1))
			SoMsg("err", err, ShouldBeNil)
			SoMsg("ok", ok, ShouldBeTrue)
			SoMsg("allocated", allocated, ShouldEqual, 50)

			allocated, ok, err = b.AllocExpiring(49, currTick.Add(2))
			SoMsg("err", err, ShouldBeNil)
			SoMsg("ok", ok, ShouldBeTrue)
			SoMsg("allocated", allocated, ShouldEqual, 49)

			allocated, ok, err = b.AllocExpiring(50, currTick.Add(1))
			SoMsg("err", err, ShouldBeNil)
			SoMsg("ok", ok, ShouldBeFalse)
			SoMsg("allocated", allocated, ShouldEqual, 1)

			err = b.DeallocExpiring(49, currTick.Add(2))
			SoMsg("err", err, ShouldBeNil)
			SoMsg("free", b.Free(), ShouldEqual, 50)

			//time.Sleep(sbresv.TickDuration)
			//SoMsg("free", b.Free(), ShouldEqual, 100)

		})
	})
}

func Test_deallocRing(t *testing.T) {
	Convey("Dealloc keeps track of bandwidth", t, func() {
		Convey("Only add", func() {

			currTick := sbresv.CurrentTick()
			ring := &deallocRing{
				currTick: currTick,
				freeRing: make([]uint64, sbresv.MaxEphemTicks*2),
			}

			resvMap := map[int]uint64{
				1: 100,
				3: 400,
				5: 1500,
				6: 100,
				7: 400,
			}
			checkMap := map[int]uint64{
				1: 100,
				2: 0,
				4: 400,
				5: 1500,
				8: 500,
			}

			resvF(ring, currTick, resvMap)
			checkF(ring, currTick, checkMap)
		})

		Convey("Add and unreserve", func() {

			currTick := sbresv.CurrentTick()
			ring := &deallocRing{
				currTick: currTick,
				freeRing: make([]uint64, sbresv.MaxEphemTicks*2),
			}

			resvMap := map[int]uint64{
				1: 100,
				3: 400,
				5: 1500,
				6: 100,
				7: 400,
			}
			unresvMap := map[int]uint64{
				3: 400,
				5: 1500,
			}
			checkMap := map[int]uint64{
				1: 100,
				2: 0,
				4: 0,
				5: 0,
				8: 500,
			}

			resvF(ring, currTick, resvMap)
			unresvF(ring, currTick, unresvMap)
			checkF(ring, currTick, checkMap)
		})

		Convey("Add and unreserve with tick increase", func() {

			currTick := sbresv.CurrentTick()
			ring := &deallocRing{
				currTick: currTick,
				freeRing: make([]uint64, sbresv.MaxEphemTicks*2),
			}

			resvMap := map[int]uint64{
				1: 100,
				3: 400,
				5: 1500,
				6: 100,
				7: 400,
			}
			resvF(ring, currTick, resvMap)

			dealloc := ring.cleanUp(currTick.Add(4))
			SoMsg("Dealloc middle 4", dealloc, ShouldEqual, 500)

			unresvMap := map[int]uint64{
				5: 1500,
			}
			checkMap := map[int]uint64{
				8: 500,
			}
			unresvF(ring, currTick, unresvMap)
			checkF(ring, currTick, checkMap)
		})

	})
}

func Test_tickOffset(t *testing.T) {
	Convey("Tick offset is correct", t, func() {
		//	Convey("Only add", func() {

		currTick := sbresv.CurrentTick()
		ring := &deallocRing{
			currTick: currTick,
			freeRing: make([]uint64, 10),
			offset:   3,
		}
		_ = ring
		for i := 1; i < 10; i++ {
			off, err := ring.tickOffset(currTick.Add(i))
			SoMsg("err", err, ShouldBeNil)
			SoMsg(fmt.Sprintf("Tick off %d", i), off, ShouldEqual, (3+i)%10)
		}

	})
}

func resvF(ring *deallocRing, currTick sbresv.Tick, resvMap map[int]uint64) {
	for _, tickOff := range sortedKeys(resvMap) {
		err := ring.reserve(currTick.Add(tickOff), resvMap[tickOff])
		SoMsg(fmt.Sprintf("Err off %d", tickOff), err, ShouldBeNil)
	}
}

func unresvF(ring *deallocRing, currTick sbresv.Tick, unresvMap map[int]uint64) {
	for _, tickOff := range sortedKeys(unresvMap) {
		err := ring.unreserve(currTick.Add(tickOff), unresvMap[tickOff])
		SoMsg(fmt.Sprintf("Err off %d", tickOff), err, ShouldBeNil)
	}
}

func checkF(ring *deallocRing, currTick sbresv.Tick, checkMap map[int]uint64) {
	for _, tickOff := range sortedKeys(checkMap) {
		dealloc := ring.cleanUp(currTick.Add(tickOff))
		SoMsg("Tick "+strconv.Itoa(tickOff), dealloc, ShouldEqual, checkMap[tickOff])

	}
}

func sortedKeys(m map[int]uint64) []int {
	keys := []int(nil)
	for key := range m {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	return keys
}
