// Package config resolves how to reach a Datastore endpoint: CLI flags
// first, falling back to the same environment variables the official client
// libraries and gcloud emulator honor.
package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/krishnan/datastore-tui/datastore/client"
)

// Config holds the resolved project/endpoint the client.Client should use.
type Config struct {
	ProjectID string
	Endpoint  string
}

// Load parses CLI flags (falling back to DATASTORE_EMULATOR_HOST /
// GOOGLE_CLOUD_PROJECT / DATASTORE_PROJECT_ID) into a Config. Real-GCP
// authentication (ADC) is not wired up yet — Endpoint is expected to point
// at a local emulator, which performs no auth check.
func Load(args []string) (Config, error) {
	fs := flag.NewFlagSet("datastore-tui", flag.ContinueOnError)
	project := fs.String("project", os.Getenv("GOOGLE_CLOUD_PROJECT"), "GCP project ID (or GOOGLE_CLOUD_PROJECT/DATASTORE_PROJECT_ID env var)")
	endpoint := fs.String("endpoint", "", "Datastore REST endpoint, e.g. http://localhost:8081 (or DATASTORE_EMULATOR_HOST env var)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{ProjectID: *project, Endpoint: *endpoint}
	if cfg.ProjectID == "" {
		cfg.ProjectID = os.Getenv("DATASTORE_PROJECT_ID")
	}
	if cfg.ProjectID == "" {
		cfg.ProjectID = "test-project" // the emulator accepts any project ID
	}
	if cfg.Endpoint == "" {
		if host := os.Getenv("DATASTORE_EMULATOR_HOST"); host != "" {
			cfg.Endpoint = "http://" + host
		} else {
			cfg.Endpoint = "http://localhost:8081"
		}
	}
	return cfg, nil
}

// Client builds a datastore/client.Client for this Config. TokenSource is
// left nil (no auth), matching the local-emulator-only scope for now.
func (c Config) Client() *client.Client {
	return client.New(client.Config{ProjectID: c.ProjectID, Endpoint: c.Endpoint})
}

func (c Config) String() string {
	return fmt.Sprintf("project=%s endpoint=%s", c.ProjectID, c.Endpoint)
}
