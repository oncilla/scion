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
	"strings"
	"sync"

	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/proto"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
)

type SibraState struct {
	sync.RWMutex
	// InIfids maps an egress interface to a set of ingress interfaces
	// which can possibly contain transit traffic to it.
	InIfids map[common.IFIDType]map[common.IFIDType]struct{}
	// Infos maps interfaces to bandwidth information.
	Infos map[common.IFIDType]IFInfo
	// TransitCap keeps track of the allocated bandwidth between two
	// interfaces.
	TransitCap map[sbalgo.IFTuple]*IFState
	// SteadyMap keeps track of steady reservations.
	SteadyMap *SteadyResvMap
	// TempTable stores temporary reservations.
	TempTable *TempTable
	// Delta is the factor for computing available bandwidth.
	Delta float64
	// Topo is the topology.
	Topo *topology.Topo
}

func NewSibraState(topo *topology.Topo, mat Matrix) (*SibraState, error) {
	s := &SibraState{
		InIfids:    map[common.IFIDType]map[common.IFIDType]struct{}{},
		Infos:      make(map[common.IFIDType]IFInfo),
		TransitCap: make(map[sbalgo.IFTuple]*IFState),
		TempTable:  NewTempTable(),
		Delta:      0.6,
		Topo:       topo,
	}
	s.SteadyMap = NewSteadyResvMap(s)
	// Set bandwidth for interface pairs
	for inIfid, inInfo := range topo.IFInfoMap {
		for egIfid, egInfo := range topo.IFInfoMap {
			s.addTransitCap(mat, inIfid, egIfid, inInfo.LinkType, egInfo.LinkType)
		}
	}
	// Set bandwidth for traffic to local AS
	for inIfid, inInfo := range topo.IFInfoMap {
		s.addTransitCap(mat, inIfid, 0, inInfo.LinkType, proto.LinkType_unset)
	}
	// Set bandwidth for traffic from local AS
	for egIfid, egInfo := range topo.IFInfoMap {
		s.addTransitCap(mat, 0, egIfid, proto.LinkType_unset, egInfo.LinkType)
	}
	return s, nil
}

func (s *SibraState) addTransitCap(mat Matrix, in, eg common.IFIDType, inT, egT proto.LinkType) {
	// For entries which are not in the matrix, this will return zero
	bps := sibra.Bps(mat[in][eg])
	ifids := sbalgo.IFTuple{
		InIfid: in,
		EgIfid: eg,
	}

	if _, ok := s.InIfids[eg]; !ok {
		s.InIfids[eg] = make(map[common.IFIDType]struct{})
	}
	if eg != in {
		s.InIfids[eg][in] = struct{}{}
	}
	// XXX(roosd): transit capacity is not respected currently, only the
	// ingress and egress capacities.
	s.TransitCap[ifids] = &IFState{
		Total:     bps,
		Unlimited: true,
	}
	s.addInCap(in, bps)
	s.addEgCap(eg, bps)
}

func (s *SibraState) addInCap(ifid common.IFIDType, bps sibra.Bps) {
	if _, ok := s.Infos[ifid]; !ok {
		s.Infos[ifid] = IFInfo{
			Ingress: &IFState{},
			Egress:  &IFState{},
		}
	}
	s.Infos[ifid].Ingress.Total += bps
}

func (s *SibraState) addEgCap(ifid common.IFIDType, bps sibra.Bps) {
	if _, ok := s.Infos[ifid]; !ok {
		s.Infos[ifid] = IFInfo{
			Ingress: &IFState{},
			Egress:  &IFState{},
		}
	}
	s.Infos[ifid].Egress.Total += bps
}

func (s *SibraState) infoString() string {
	var infoList []string
	for k, v := range s.Infos {
		infoList = append(infoList, fmt.Sprintf("Info ifid: %d {Ing: %v, Eg: %v}",
			k, v.Ingress, v.Egress))
	}
	return strings.Join(infoList, "\n")
}

func (s *SibraState) String() string {
	return fmt.Sprintf("Delta: %v Infos: \n%v", s.Delta, s.infoString())
}
