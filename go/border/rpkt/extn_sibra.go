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

package rpkt

import (
	"fmt"

	"github.com/scionproto/scion/go/border/rcmn"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/sibra/sbextn"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
)

var _ rExtension = (*rSibraExtn)(nil)

// rSibraExtn is the router's representation of the SIBRA extension.
type rSibraExtn struct {
	*sbextn.Base
	raw       common.RawBytes
	rp        *RtrPkt
	offBlocks int
	infoF     *sbresv.Info
	sofF      *sbresv.SOField
}

// rSibraExtnFromRaw creates an rSibraExtn instance from raw bytes,
// keeping a reference to the location in the packet's buffer.
func rSibraExtnFromRaw(rp *RtrPkt, start, end int) (*rSibraExtn, error) {
	raw := rp.Raw[start:end]
	base, err := sbextn.BaseFromRaw(raw)
	if err != nil {
		return nil, err
	}
	ext := &rSibraExtn{
		Base: base,
		raw:  raw,
		rp:   rp,
	}
	ext.parseResvIDs()
	return ext, nil
}

func (s *rSibraExtn) parseResvIDs() {
	off, end := 0, common.ExtnFirstLineLen
	if !s.Steady {
		off, end = end, end+sibra.EphemIDLen
		s.ParseID(s.raw[off:end])
	}
	for i := 0; i < s.TotalSteady; i++ {
		off, end = end, end+sibra.SteadyIDLen
		s.ParseID(s.raw[off:end])
	}
	s.offBlocks = s.ActiveBlockOff()
}

func (s *rSibraExtn) NextSOFIndex() error {
	if err := s.Base.NextSOFIndex(); err != nil {
		return err
	}
	s.raw[sbextn.OffSOFIndex] = s.SOFIndex
	s.sofF = nil
	s.infoF = nil
	return nil
}

func (s *rSibraExtn) Info() *sbresv.Info {
	if s.infoF != nil {
		return s.infoF
	}
	off := s.calcInfoOff()
	s.infoF = sbresv.NewInfoFromRaw(s.raw[off : off+sbresv.InfoLen])
	return s.infoF
}

func (s *rSibraExtn) IFCurr(consDir bool, dirFrom,
	dirTo rcmn.Dir) (HookResult, common.IFIDType, error) {

	var isIngress bool
	switch dirFrom {
	case rcmn.DirSelf, rcmn.DirLocal:
		isIngress = !consDir
	case rcmn.DirExternal:
		isIngress = consDir
	default:
		return HookError, 0, common.NewBasicError("DirFrom value unsupported", nil,
			"val", dirFrom)
	}
	if isIngress {
		return HookFinish, s.SOF().Ingress, nil
	}
	return HookFinish, s.SOF().Egress, nil
}

func (s *rSibraExtn) IFNext(consDir bool, dirFrom,
	dirTo rcmn.Dir) (HookResult, common.IFIDType, error) {

	var isEgress bool
	switch dirFrom {
	case rcmn.DirSelf, rcmn.DirLocal:
		isEgress = !consDir
	case rcmn.DirExternal:
		isEgress = consDir
	default:
		return HookError, 0, common.NewBasicError("DirFrom value unsupported", nil,
			"val", dirFrom)
	}
	if isEgress {
		return HookFinish, s.SOF().Egress, nil
	}
	return HookFinish, s.SOF().Ingress, nil
}

func (s *rSibraExtn) ConsDirFlag() (HookResult, bool, error) {
	return HookFinish, s.Forward, nil
}

func (s *rSibraExtn) SOF() *sbresv.SOField {
	if s.sofF != nil {
		return s.sofF
	}
	off := s.calcSOFOff()
	s.sofF, _ = sbresv.NewSOFieldFromRaw(s.raw[off : off+sbresv.SOFieldLen])
	return s.sofF
}

func (s *rSibraExtn) RawVerifyingSOF() common.RawBytes {
	diff := 0
	if s.Info().PathType.GenFwd() && s.RelSOFIdx != 0 {
		diff = -1
	} else if !s.Info().PathType.GenFwd() && s.RelSteadyHop != int(s.PathLens[s.CurrSteady])-1 {
		// We can take s.RelSteadyHop and s.CurrSteady here,
		// since ephemeral paths are gen forward.
		diff = 1
	} else {
		return nil
	}
	off := s.calcSOFOff() + sbresv.SOFieldLen*diff
	return s.raw[off : off+sbresv.SOFieldLen]
}

func (s *rSibraExtn) calcSOFOff() int {
	return s.calcInfoOff() + sbresv.InfoLen + int(s.RelSOFIdx)*sbresv.SOFieldLen
}

func (s *rSibraExtn) calcInfoOff() int {
	infoOff := s.offBlocks
	for i := 0; i < s.CurrBlock; i++ {
		infoOff += sbresv.InfoLen + int(s.PathLens[i])*sbresv.SOFieldLen
	}
	return infoOff
}

func (s *rSibraExtn) String() string {
	ext, err := s.GetExtn()
	if err != nil {
		return fmt.Sprintf("SIBRAExtn: %v", err)
	}
	return ext.String()
}

func (s *rSibraExtn) RegisterHooks(h *hooks) error {
	// Steady setup requests do not have an active reservation block.
	if !s.Setup {
		s.rp.ignorePath = true
		h.ConsDirFlag = append(h.ConsDirFlag, s.ConsDirFlag)
		h.IFCurr = append(h.IFCurr, s.IFCurr)
		h.IFNext = append(h.IFNext, s.IFNext)
		h.Validate = append(h.Validate, s.VerifySOF)
	}
	if !s.IsRequest {
		h.Route = append(h.Route, s.RouteSibraData)
	} else {
		h.Route = append(h.Route, s.RouteSibraRequest)
	}
	return nil
}

func (s *rSibraExtn) GetExtn() (common.Extension, error) {
	if s.Steady {
		return sbextn.SteadyFromRaw(s.raw)
	}
	return sbextn.EphemeralFromRaw(s.raw)
}
