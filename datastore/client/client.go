// Package client is a thin REST client for the Cloud Datastore API v1
// (https://cloud.google.com/datastore/docs/reference/data/rest), used
// directly rather than the gRPC cloud.google.com/go/datastore client so that
// the no-auth local emulator and a real authenticated GCP project are both
// just a Config away.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

// Config points the client at a Datastore endpoint (the local emulator or
// the real GCP API) and supplies credentials.
type Config struct {
	ProjectID string

	// Endpoint is the API base URL, e.g. "http://localhost:8081" for the
	// emulator or "https://datastore.googleapis.com" for a real project.
	Endpoint string

	// TokenSource supplies an Authorization: Bearer token on every request.
	// Leave nil for the emulator, which performs no auth check.
	TokenSource oauth2.TokenSource

	// HTTPClient overrides the default http.Client; mainly for tests.
	HTTPClient *http.Client
}

// Client is a Cloud Datastore REST API v1 client.
type Client struct {
	cfg Config
}

// New builds a Client from cfg, defaulting HTTPClient to http.DefaultClient
// when unset.
func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Client{cfg: cfg}
}

func (c *Client) methodURL(method string) string {
	return fmt.Sprintf("%s/v1/projects/%s:%s", c.cfg.Endpoint, c.cfg.ProjectID, method)
}

// post issues a POST to the given Datastore RPC method (e.g. "lookup",
// "runQuery", "commit") with body marshaled as JSON, and unmarshals the JSON
// response into out (which may be nil to discard the body).
func (c *Client) post(ctx context.Context, method string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("client: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.methodURL(method), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.cfg.TokenSource != nil {
		tok, err := c.cfg.TokenSource.Token()
		if err != nil {
			return fmt.Errorf("client: fetch auth token: %w", err)
		}
		tok.SetAuthHeader(req)
	}

	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("client: %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("client: %s: read response: %w", method, err)
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{Method: method, StatusCode: resp.StatusCode, Body: respBody}
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("client: %s: unmarshal response: %w", method, err)
	}
	return nil
}

// APIError is returned when the Datastore API responds with a non-200
// status; Body holds the raw (typically JSON) error payload for display.
type APIError struct {
	Method     string
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("client: %s: HTTP %d: %s", e.Method, e.StatusCode, string(e.Body))
}
