package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"time"

	"github.com/aybabtme/benchkit"
	"github.com/scionproto/scion/go/lib/topology"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/sibra"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/state"
)

const (
	AlgoFast Algo = iota
	AlgoSlow
)

type Algo int

func (a Algo) Algo(topo *topology.Topo, mat state.Matrix) (sibra.Algo, error) {
	if a == AlgoFast {
		return sbalgo.NewSibraFast(topo, mat)
	}
	return sbalgo.NewSibraSlow(topo, mat)
}

func (a Algo) State(algo sibra.Algo) *state.SibraState {
	if a == AlgoFast {
		return algo.(*sbalgo.AlgoFast).SibraState
	}
	return algo.(*sbalgo.AlgoSlow).SibraState
}

type DurSlice []time.Duration

func (d DurSlice) String() string {
	vals := make([]string, len(d))
	for i := range d {
		vals[i] = fmt.Sprintf("%d", d[i].Nanoseconds())
	}
	return fmt.Sprintf("[%s]", strings.Join(vals, ", "))
}

type TimeStep benchkit.TimeStep

func (t TimeStep) All(duration time.Duration) []float64 {
	v := reflect.ValueOf(t)
	y := v.FieldByName("all")
	l := y.Len()
	sl := y.Slice(0, l)
	a := make([]float64, sl.Len())
	for i := 0; i < sl.Len(); i++ {
		a[i] = float64(sl.Index(i).Int()) / float64(duration)
	}
	return a
}

func (t TimeStep) String() string {
	v := reflect.ValueOf(t)
	y := v.FieldByName("all")
	l := y.Len()
	str := fmt.Sprintf("%v", y.Slice(0, l))
	vals := strings.Replace(str, " ", ", ", -1)
	return fmt.Sprintf("min %s avg %s max %s sd %s\n%s\n%v", t.Min, t.Avg, t.Max, t.SD, vals,
		DurSlice(t.Significant))
}

func saveBench(par benchParams, res *benchkit.TimeResult) error {
	m := make(map[string]interface{})
	m["title"] = par.title
	m["label"] = par.label
	m["ops"] = par.numOps
	m["cutoff"] = par.cutoff
	m["runs"] = len(par.runs)
	var vals []float64
	var labels []string
	for i, ts := range res.Each {
		all := TimeStep(ts).All(time.Microsecond)[par.cutoff : par.numOps-par.cutoff]
		vals = append(vals, all...)
		label := par.runs[i].Label
		if label == "" {
			label = fmt.Sprintf("%d ASes", par.runs[i].Srcs)
		}
		labels = append(labels, repeat(label, len(all))...)
	}
	m["vals"] = vals
	m["labels"] = labels

	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	fname := fmt.Sprintf("bench/%s-%s.json", par.name, time.Now().Format("2006-01-02-15:04:05"))

	return ioutil.WriteFile(fname, b, 0644)
}

func repeat(str string, n int) []string {
	strs := make([]string, n)
	for i := range strs {
		strs[i] = str
	}
	return strs
}
