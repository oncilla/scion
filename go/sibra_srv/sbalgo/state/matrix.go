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
	"io/ioutil"

	"gopkg.in/yaml.v2"

	"github.com/scionproto/scion/go/lib/common"
)

const (
	MatrixName = "matrix.yml"

	ErrorOpenMat  = "Unable to open matrix"
	ErrorParseMat = "Unable to parse matrix from YAML"
)

// Matrix is the traffic matrix which specifies how much bandwidth can be
// reserved from an ingress to an egress interface. The mapping is
// ingress->egress->bps.
type Matrix map[common.IFIDType]map[common.IFIDType]uint64

func MatrixFromRaw(b common.RawBytes) (Matrix, error) {
	var m Matrix
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, common.NewBasicError(ErrorParseMat, err)
	}
	return m, nil
}

func MatrixFromFile(path string) (Matrix, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, common.NewBasicError(ErrorOpenMat, err, "path", path)
	}
	return MatrixFromRaw(b)
}
