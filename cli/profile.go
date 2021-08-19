// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package cli

import (
	"github.com/pkg/profile"
)

type (
	// Profiling is a struct that can be embedded in any kong cli to enable
	// easy and convenient profiling for performance analysis. The flag is
	// hidden from the help message such that we can use it in public cli.
	//
	// The supported values are:
	//   - "cpu":    Enables CPU profiling.
	//   - "memory": Enables heap memory profiling.
	//   - "block":  Enables block (contention) profiling.
	//   - "mutex":  Enables mutex profiling.
	//   - "trace":  Enables trace profiling.
	//
	// The profiling file path will be shown to stderr and can be opened with
	// `go tool pprof $file` or `go tool pprof -http localhost:8080 $file`
	//
	// In order to enable profiling, one should use the command like this:
	// ```
	// stopProfiling := cli.Profiling.Start()
	// defer stopProfiling()
	// ```
	Profiling struct {
		Profiling string `opt:"" hidden:"true" default:""`
	}
)

// Start starts the profiling operation. It returns a function that needs to be
// called when the profiling should stop.
func (p *Profiling) Start() func() {
	path := profile.ProfilePath(".")
	switch p.Profiling {
	case "cpu":
		return profile.Start(path, profile.CPUProfile).Stop
	case "memory":
		return profile.Start(path, profile.MemProfile).Stop
	case "block":
		return profile.Start(path, profile.BlockProfile).Stop
	case "mutex":
		return profile.Start(path, profile.MutexProfile).Stop
	case "trace":
		return profile.Start(path, profile.TraceProfile).Stop
	default:
		return func() {}
	}
}
