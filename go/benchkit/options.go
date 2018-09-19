package main

import (
	"github.com/aybabtme/benchkit"
	"github.com/scionproto/scion/go/sibra_srv/sbalgo/sibra"
)

type runParams struct {
	Srcs  int
	RpS   int
	Label string
	bench func(idx int, s sibra.Algo, p runParams, ops int, each benchkit.BenchEach)
}

func (r runParams) Run(idx int, s sibra.Algo, ops int, each benchkit.BenchEach) {
	r.bench(idx, s, r, ops, each)
}

type benchParams struct {
	title  string
	name   string
	label  string
	algo   Algo
	numOps int
	cutoff int
	runs   []runParams
}

var BenchFastToken = benchParams{
	title:  "",
	name:   "fast-ephemAdmission",
	label:  "Number of ASes",
	algo:   AlgoFast,
	numOps: 150,
	cutoff: 10,
	runs: []runParams{
		{
			Srcs:  1000,
			RpS:   10,
			Label: "Steady Create",
			bench: steadyTokenCreate,
		},
		{
			Srcs:  1000,
			RpS:   10,
			Label: "Steady Verify",
			bench: steadyTokenAuth,
		},
		{
			Srcs:  1000,
			RpS:   10,
			Label: "Ephemeral Create",
			bench: ephemTokenCreate,
		},
		{
			Srcs:  1000,
			RpS:   10,
			Label: "Ephemeral Verify",
			bench: ephemTokenAuth,
		},
	},
}

var BenchFastEphem = benchParams{
	title:  "",
	name:   "fast-ephemAdmission",
	label:  "Number of ASes",
	algo:   AlgoFast,
	numOps: 150,
	cutoff: 10,
	runs: []runParams{
		{
			Srcs:  1000,
			RpS:   10,
			Label: "Setup Request",
			bench: ephemRunSetup,
		},
		{
			Srcs:  1000,
			RpS:   10,
			Label: "Renewal Request",
			bench: ephemRunRenew,
		},
	},
}

var BenchSlowFixedReqPerSrc = benchParams{
	title:  "",
	name:   "slow-fixedRPS",
	label:  "Number of ASes",
	algo:   AlgoSlow,
	numOps: 10,
	cutoff: 1,
	runs: []runParams{
		{
			Srcs:  10,
			RpS:   10,
			bench: steadyRunSetup,
		},
		{
			Srcs:  50,
			RpS:   10,
			bench: steadyRunSetup,
		},
		{
			Srcs:  100,
			RpS:   10,
			bench: steadyRunSetup,
		},
		{
			Srcs:  150,
			RpS:   10,
			bench: steadyRunSetup,
		},
		{
			Srcs:  200,
			RpS:   10,
			bench: steadyRunSetup,
		},
	},
}

var BenchFastSetup = benchParams{
	title:  "",
	name:   "fast-steady-setup",
	label:  "Number of ASes",
	algo:   AlgoFast,
	numOps: 150,
	cutoff: 10,
	runs: []runParams{
		{
			Srcs:  1000,
			RpS:   100,
			bench: steadyRunSetup,
		},
		{
			Srcs:  5000,
			RpS:   100,
			bench: steadyRunSetup,
		},
		{
			Srcs:  10000,
			RpS:   100,
			bench: steadyRunSetup,
		},
		{
			Srcs:  15000,
			RpS:   100,
			bench: steadyRunSetup,
		},
		{
			Srcs:  20000,
			RpS:   100,
			bench: steadyRunSetup,
		},
	},
}

var BenchFastRenew = benchParams{
	title:  "",
	name:   "fast-steady-renew",
	label:  "Number of ASes",
	algo:   AlgoFast,
	numOps: 1000,
	cutoff: 10,
	runs: []runParams{
		{
			Srcs:  1000,
			RpS:   100,
			bench: steadyRunRenew,
		},
		{
			Srcs:  5000,
			RpS:   100,
			bench: steadyRunRenew,
		},
		{
			Srcs:  10000,
			RpS:   100,
			bench: steadyRunRenew,
		},
		{
			Srcs:  15000,
			RpS:   100,
			bench: steadyRunRenew,
		},
		{
			Srcs:  20000,
			RpS:   100,
			bench: steadyRunRenew,
		},
	},
}

var BenchFastSetupSmall = benchParams{
	title:  "",
	name:   "fast-fixedRPS-small",
	label:  "Number of ASes",
	algo:   AlgoFast,
	numOps: 200,
	cutoff: 10,
	runs: []runParams{
		{
			Srcs:  5000,
			RpS:   100,
			bench: steadyRunSetup,
		},
		{
			Srcs:  10000,
			RpS:   100,
			bench: steadyRunSetup,
		},
	},
}
