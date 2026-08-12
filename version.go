package main

import "fmt"

// version, commit, and date are set at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X main.version=v1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// goreleaser populates these automatically for tagged releases; local
// `go build`/`go run` leave them at their "dev"/"none"/"unknown" defaults.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	return fmt.Sprintf("datastore-tui %s (commit %s, built %s)", version, commit, date)
}
