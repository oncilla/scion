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

package main

import (
	"github.com/scionproto/scion/go/lib/crypto/trc"
	"io/ioutil"
	"github.com/scionproto/scion/go/lib/crypto/cert"
	"fmt"
	"os"
)

var (
	trcFileErr = 0
	trcParseErr = 0
	trcTotal = 0
	trcSucc = 0
	crtFileErr = 0
	crtParseErr = 0
	crtTotal = 0
	crtSucc = 0
)

func testTRC(file string) {
	trcTotal++
	b, err := ioutil.ReadFile(file)
	if err != nil {
		trcFileErr++
		return
	}
	if _, err = trc.TRCFromRaw(b, false); err != nil {
		trcParseErr++
		return
	}
	trcSucc++
}

func testChain(file string) {
	crtTotal++
	b, err := ioutil.ReadFile(file)
	if err != nil {
		crtFileErr++
		return
	}
	if _, err = cert.ChainFromRaw(b, false); err != nil {
		crtParseErr++
		return
	}
	crtSucc++
}


func main() {
	f := testTRC
	for _, file := range os.Args[1:] {
		switch file {
		case "-t":
			f = testTRC
			continue
		case "-c":
			f = testChain
			continue
		}
		f(file)
	}
	fmt.Printf("TRC:\tSucc %d\tFileErr %d\tParseErr %d\tTotal %d\n", trcSucc, trcFileErr, trcParseErr, trcTotal)
	fmt.Printf("CRT:\tSucc %d\tFileErr %d\tParseErr %d\tTotal %d\n", crtSucc, crtFileErr, crtParseErr, crtTotal)
}