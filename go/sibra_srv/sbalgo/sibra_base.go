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

package sbalgo

import (
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra/sbreq"
	"github.com/scionproto/scion/go/lib/sibra/sbresv"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/sibra"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

var _ sibra.EphemAdm = (*AlgoSlow)(nil)

// AlgoSlow implements the SIBRA algorithm.
type algoBase struct {
	*ephemAdm
}

func (s *algoBase) prevBw(p sibra.AdmParams) sbresv.BwCls {
	minMax := p.Req.MaxBw
	// Take minimum of all previously max offers.
	if p.Req.Info.PathType.Reversed() {
		// If the path is established in the reverse order, the
		// higher opaque fields are set.
		for i := p.Extn.TotalHops - 1; i > int(p.Extn.SOFIndex); i-- {
			if p.Req.OfferFields[i].MaxBw < minMax {
				minMax = p.Req.OfferFields[i].MaxBw
			}
		}
		return minMax
	}
	for i := 0; i < int(p.Extn.SOFIndex); i++ {
		if p.Req.OfferFields[i].MaxBw < minMax {
			minMax = p.Req.OfferFields[i].MaxBw
		}
	}
	return minMax
}

func (s *algoBase) PromoteToSOFCreated(ifids sibra.IFTuple, id sbresv.ID, info *sbresv.Info) error {
	s.Lock()
	defer s.Unlock()
	stEntry, ok := s.SteadyMap.Get(id)
	if !ok {
		return common.NewBasicError("Steady reservation not found", nil, "id", id)
	}
	if err := s.validateIfids(ifids, stEntry); err != nil {
		return err
	}
	// PromoteToSOFCreated validates that the info header is consistent with the entry in the
	// reservation map.
	if err := stEntry.PromoteToSOFCreated(info); err != nil {
		return common.NewBasicError("Unable to promote SOFCreated", err, "id", id)
	}
	return nil
}

func (s *algoBase) PromoteToPending(ifs sibra.IFTuple, id sbresv.ID, c *sbreq.ConfirmIndex) error {
	s.Lock()
	defer s.Unlock()
	stEntry, ok := s.SteadyMap.Get(id)
	if !ok {
		return common.NewBasicError("Reservation does not exist", nil, "id", id)
	}
	if err := s.validateIfids(ifs, stEntry); err != nil {
		return err
	}
	if err := stEntry.PromoteToPending(c.Idx); err != nil {
		return common.NewBasicError("Unable to promote", err, "id", id)
	}
	s.TempTable.Delete(id, c.Idx)
	return nil
}
func (s *algoBase) PromoteToActive(ifids sibra.IFTuple, id sbresv.ID, info *sbresv.Info,
	c *sbreq.ConfirmIndex) error {

	s.Lock()
	defer s.Unlock()
	stEntry, ok := s.SteadyMap.Get(id)
	if !ok {
		return common.NewBasicError("Reservation does not exist", nil, "id", id)
	}
	if err := s.validateIfids(ifids, stEntry); err != nil {
		return err
	}
	if err := stEntry.PromoteToActive(c.Idx, info); err != nil {
		return common.NewBasicError("Unable to promote to active", err, "id", id)
	}
	return nil
}

func (s *algoBase) validateIfids(ifids sibra.IFTuple, entry *state.SteadyResvEntry) error {
	// Validate that the packet traverses the correct interfaces
	if ifids.InIfid != entry.Ifids.InIfid || ifids.EgIfid != entry.Ifids.EgIfid {
		return common.NewBasicError("Invalid interfaces", nil,
			"expected ingress", entry.Ifids.InIfid, "actual ingress", ifids.InIfid,
			"expected egress", entry.Ifids.EgIfid, "actual egress", ifids.EgIfid)
	}
	return nil
}

func minBps(a, b sbresv.Bps) sbresv.Bps {
	if a > b {
		return b
	}
	return a
}
