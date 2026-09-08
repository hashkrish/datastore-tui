// Package config resolves how to reach a Datastore endpoint: CLI flags
// first, falling back to the same environment variables the official client
// libraries and gcloud emulator honor.
package config

import (
	"context"
	"flag"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/krishnan/datastore-tui/datastore/client"
)

// datastoreScope is the OAuth scope requested for real-GCP Application
// Default Credentials lookups.
const datastoreScope = "https://www.googleapis.com/auth/datastore"

// Config holds the resolved project/endpoint/credentials the
// client.Client should use.
type Config struct {
	ProjectID string
	Endpoint  string

	// TokenSource authenticates requests against a real GCP project. It is
	// nil when targeting the local emulator, which performs no auth check.
	TokenSource oauth2.TokenSource

	// ReadOnly disables every mutating action in the UI (new/delete entity,
	// retype/add/delete property, save) — recommended when browsing a real
	// project you don't want to risk accidentally modifying.
	ReadOnly bool
}

// Load parses CLI flags (falling back to DATASTORE_EMULATOR_HOST /
// GOOGLE_CLOUD_PROJECT / DATASTORE_PROJECT_ID) into a Config.
//
// Setting -endpoint or DATASTORE_EMULATOR_HOST selects the local,
// unauthenticated emulator, matching the official client libraries'
// convention. Otherwise Load targets the real Cloud Datastore API and
// resolves Application Default Credentials (ADC) via ctx: a service account
// key (GOOGLE_APPLICATION_CREDENTIALS), `gcloud auth application-default
// login`, or GCE/GKE metadata.
func Load(ctx context.Context, args []string) (Config, error) {
	fs := flag.NewFlagSet("datastore-tui", flag.ContinueOnError)
	project := fs.String("project", os.Getenv("GOOGLE_CLOUD_PROJECT"), "GCP project ID (or GOOGLE_CLOUD_PROJECT/DATASTORE_PROJECT_ID env var)")
	endpoint := fs.String("endpoint", "", "Datastore REST endpoint, e.g. http://localhost:8081 (or DATASTORE_EMULATOR_HOST env var); if unset, targets real GCP via Application Default Credentials")
	readOnly := fs.Bool("read-only", false, "disable all mutating actions (new/delete entity, retype/add/delete property, save)")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	cfg := Config{ProjectID: *project, Endpoint: *endpoint, ReadOnly: *readOnly}
	if cfg.ProjectID == "" {
		cfg.ProjectID = os.Getenv("DATASTORE_PROJECT_ID")
	}

	emulatorHost := os.Getenv("DATASTORE_EMULATOR_HOST")
	if cfg.Endpoint == "" && emulatorHost != "" {
		cfg.Endpoint = "http://" + emulatorHost
	}

	if cfg.Endpoint != "" {
		// Emulator mode: no auth, any project ID is accepted.
		if cfg.ProjectID == "" {
			cfg.ProjectID = "test-project" // the emulator accepts any project ID
		}
		return cfg, nil
	}

	// Real-GCP mode: resolve Application Default Credentials.
	cfg.Endpoint = "https://datastore.googleapis.com"
	creds, err := google.FindDefaultCredentials(ctx, datastoreScope)
	if err != nil {
		return Config{}, fmt.Errorf("config: find Application Default Credentials (run `gcloud auth application-default login`, set GOOGLE_APPLICATION_CREDENTIALS, or set DATASTORE_EMULATOR_HOST to use the local emulator instead): %w", err)
	}
	cfg.TokenSource = creds.TokenSource

	if cfg.ProjectID == "" {
		cfg.ProjectID = creds.ProjectID
	}
	if cfg.ProjectID == "" {
		return Config{}, fmt.Errorf("config: no GCP project ID: pass -project, set GOOGLE_CLOUD_PROJECT/DATASTORE_PROJECT_ID, or set DATASTORE_EMULATOR_HOST to use the local emulator instead")
	}

	return cfg, nil
}

// Client builds a datastore/client.Client for this Config.
func (c Config) Client() *client.Client {
	return client.New(client.Config{ProjectID: c.ProjectID, Endpoint: c.Endpoint, TokenSource: c.TokenSource})
}

func (c Config) String() string {
	return fmt.Sprintf("project=%s endpoint=%s", c.ProjectID, c.Endpoint)
}
