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

package conf

import (
	"encoding/json"
	"io/ioutil"

	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/common"
	"github.com/scionproto/scion/go/lib/sibra"
	"github.com/scionproto/scion/go/lib/spath/spathmeta"
)

const (
	ResvName = "reservations.json"

	ErrorOpenResvs  = "Unable to open reservation map"
	ErrorParseResvs = "Unable to parse reservation map from JSON"
)

type ResvsMap map[string]*Resv

type Resv struct {
	IA            addr.IA
	PathPredicate *spathmeta.PathPredicate
	PathType      sibra.PathType
	MinSize       sibra.BwCls
	DesiredSize   sibra.BwCls
	MaxSize       sibra.BwCls
	RatioClass    sibra.RatioCls
}

func ReservationsFromRaw(b common.RawBytes) (ResvsMap, error) {
	var m ResvsMap
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, common.NewBasicError(ErrorOpenResvs, err)
	}
	return m, nil
}

func ReservationsFromFile(path string) (ResvsMap, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, common.NewBasicError(ErrorParseResvs, err, "path", path)
	}
	return ReservationsFromRaw(b)
}
