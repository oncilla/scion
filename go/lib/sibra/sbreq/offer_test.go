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

package sbreq

import (
	"encoding/hex"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/scionproto/scion/go/lib/common"
)

var rawOffer, _ = hex.DecodeString("010203")

func TestNewOfferFromRaw(t *testing.T) {
	Convey("Parse offer correctly", t, func() {
		SoMsg("offer", *NewOfferFromRaw(rawOffer), ShouldResemble, Offer{
			AllocBw: 1,
			MaxBw:   2,
			LineLen: 3,
		})
	})
}

func TestOffer_Write(t *testing.T) {
	Convey("Write offer correctly", t, func() {
		offer := Offer{
			AllocBw: 1,
			MaxBw:   2,
			LineLen: 3,
		}
		b := make(common.RawBytes, offer.Len())
		err := offer.Write(b)
		SoMsg("err", err, ShouldBeNil)
		parsed := NewOfferFromRaw(b)
		SoMsg("offer", *parsed, ShouldResemble, offer)
	})

}
