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

/*
import (
	"fmt"

	"github.com/scionproto/scion/go/lib/common"
)

var _ common.Extension = (*SteadyExtn)(nil)

// EphemeralExtn is the SIBRA Ephemeral path extension header.
//
// Ephemeral paths are short-lived (by default) paths set up by endhosts to
// provide bandwidth availability guarantees for connections to a specified
// destination. Ephemeral paths are built on top of steady paths, and hence
// have multiple path IDs associated with them: an ephemeral path ID to
// identify this reservation, and up to 3 steady path IDs to identify the
// steady up/core/down paths that it is built on. This also means that
// ephemeral setup packets contain an active block from each of the steady
// paths they traverse.
type EphemeralExtn struct {
	*BaseExtn
}

func EphemeralExtnFromRaw(raw common.RawBytes) (*EphemeralExtn, error) {
	base, err := BaseExtnFromRaw(raw)
	if err != nil {
		return nil, err
	}
	return EphemeralExtnFromBase(base)
}

func EphemeralExtnFromBase(base *BaseExtn) (*EphemeralExtn, error) {
	e := &EphemeralExtn{base}
	off, end := common.ExtnFirstLineLen, common.ExtnFirstLineLen+EphemeralIDLen
	e.ResvIDs = append(e.ResvIDs, ResvID(e.raw[off:end]))
	var i int
	for i = 0; i < len(e.PathLens) && e.PathLens[i] != 0; i++ {
		off, end = end, end+SteadyIDLen
		e.ResvIDs = append(e.ResvIDs, ResvID(e.raw[off:end]))
	}
	for j := i + 1; j < len(e.PathLens); j++ {
		if e.PathLens[j] != 0 {
			return nil, common.NewBasicError(InvalidPathLens, nil, "P0", e.PathLens[0],
				"P1", e.PathLens[1], "P2", e.PathLens[2])
		}
	}
	if err := e.updateIndices(); err != nil {
		return nil, err
	}
	off = end
	// There is exactly one active ephemeral block, if this is not a setup request.
	if !e.Setup {
		end += calcResvBlockLen(e.totalHops)
		if err := e.parseActiveResvBlock(e.raw[off:end], e.totalHops); err != nil {
			return nil, err
		}
		return e, e.parseEnd(e.raw[end:])
	}
	// There are multiple active steady blocks, if this is a setup request.
	for i := 0; i < len(e.PathLens) && e.PathLens[i] != 0; i++ {
		off, end = end, end+calcResvBlockLen(int(e.PathLens[i]))
		if err := e.parseActiveResvBlock(e.raw[off:end], int(e.PathLens[i])); err != nil {
			return nil, err
		}
	}
	return e, e.parseEnd(e.raw[end:])
}

func (e *EphemeralExtn) updateIndices() error {
	if !e.Setup {
		return e.BaseExtn.updateIndices()
	}
	// There are 'num(steady path ids) - 1' block switches.
	e.totalHops = int(e.PathLens[0]+e.PathLens[1]+e.PathLens[2]) - len(e.ResvIDs) + 2
	bid, sid := 0, int(e.SOFIndex)
	// Find the current block and the relative SOF index inside that block
	for ; bid < len(e.PathLens) && sid >= int(e.PathLens[bid]); bid++ {
		sid -= int(e.PathLens[bid])
	}
	if bid >= len(e.PathLens) {
		return common.NewBasicError("Invalid SOF index", nil, "expected<", e.totalHops,
			"actual", e.SOFIndex)
	}
	e.blockIndex = bid
	e.relSOFIndex = sid
	e.currHop = int(e.SOFIndex) - bid
	return nil
}

func (e *EphemeralExtn) Copy() common.Extension {
	raw := make(common.RawBytes, len(e.raw))
	e.Write(raw)
	n, _ := EphemeralExtnFromRaw(raw)
	return n
}

func (e *EphemeralExtn) String() string {
	return fmt.Sprintf("sibra.EphemeralExtn (%dB)", e.Len())
}
*/
