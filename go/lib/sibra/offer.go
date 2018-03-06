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

import "github.com/scionproto/scion/go/lib/common"

type ResvOffer struct {
	// raw is the underlying buffer.
	raw common.RawBytes
	// Info is the reservation info field.
	Info *Info
	// BWPairs are the offered bandwidth.
	BWPairs []BWPair
}

func ResvOfferFromRaw(raw common.RawBytes, numHops int) (*ResvOffer, error) {
	if calcResvBlockLen(numHops) != len(raw) {
		return nil, common.NewBasicError("Invalid offer block length", nil, "numHops",
			numHops, "expected", calcResvBlockLen(numHops), "actual", len(raw))
	}
	off, end := 0, infoLen
	info := NewInfoFromRaw(raw[:end])
	block := &ResvOffer{
		raw:     raw,
		Info:    info,
		BWPairs: make([]BWPair, 0, numHops-int(info.FailHop)),
	}
	for i := 0; i < numHops; i++ {
		off, end = end, end+bwPairLen
		block.BWPairs = append(block.BWPairs, BWPairFromRaw(raw[off:end]))
	}
	return block, nil
}

// ResvOfferFromBlock transforms a reservation request block to an offer block.
// The ownership of the underlying buffer is transferred to the created Offer.
// After calling this function, the ResvBlock (or its fields) shall no longer be used.
func ResvOfferFromReq(resv *ResvBlock, currHop int, offer BWPair) *ResvOffer {
	block := &ResvOffer{
		raw:     resv.raw,
		Info:    resv.Info,
		BWPairs: []BWPair{offer},
	}
	resv.raw = nil
	resv.Info = nil
	resv.OpFields = nil
	block.Info.FailHop = uint8(currHop)
	block.write()
	return block
}

func (r *ResvOffer) write() {
	r.Info.write()
	for i, pair := range r.BWPairs {
		r.raw[infoLen+i*2] = uint8(pair.Fwd)
		r.raw[infoLen+i*2+1] = uint8(pair.Rev)
	}
}

func (r *ResvOffer) Len() int {
	return len(r.raw)
}
