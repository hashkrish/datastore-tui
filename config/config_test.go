package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fakeServiceAccountKey is a syntactically valid (but non-functional)
// service account JSON key, sufficient for google.FindDefaultCredentials to
// parse and build a TokenSource without making any network calls.
const fakeServiceAccountKey = `{
  "type": "service_account",
  "project_id": "adc-project",
  "private_key_id": "fake-key-id",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC5+F3lE7cJlLXn\nZL1w1nnFtBu1zYrO/vzgH1RC6JJRJ6O9NNwR8lArTuoOXGwt8vjyIzsD9rgqvzy1\nRWmKgV6ijV+Vv2n5Qc8SgH3T+dP3xBQZo5/8vPqYQuU7X0y2t/hRzTZTOAV/l/RI\nDrbFEuVQmM1TytLxjOsdRZ+LMSZjAgMBAAECggEAAv0MFVQrxKmXOJt2W+aI4T1H\nfRgQVjT8qWlBEIfSAJ/tqZk8rjK4nCk1xzsMngeaWFCiE1RcvbFC66yPRRO60QGm\n5xJPBWTKq7d3vc/qm1WQJi93P0zc4EmoRXfxIkZ+LhTqRT0mL7kOEyRxKhwUuc3G\ndKB0KpqtEQKBgQDidY0BqgO+2ScLDCVCPIRcC5jFcQBcXW7ZuUvQyEfvthN6ZwSj\nZa/L2CtHDPIWjbGpKrN/rQKBgQDR8mWnHW1J1MHKgUUmZC1exVjX4bx3iu4mI4iH\nQ4EI8IlIYokzC7fbWSC5RRAg1ExOawKBgFAr36J0GBpq2PWXeKO/lJK7EIkeAYNn\nGm0AMonMc2gEtWXmL5+g1eZZTPHFbCUC7XFn8SfCz5rGoWvCYzRQrgn9AoGBAM8h\n8b3wjkwDgKw1e5dJv1pKtLxfKQjRt7hFVfQ0X0R5PIYkVoAiOB9Tg8p9ZvXqxSpZ\nl9dP0PdKKY1JAoGAaZ82iInX2fJ4iAlj+DPzX9Zk+1FVlwnLbjjNSHiUpuP1cA0j\nSAyi4NqRrHdcRt3cVy5Y+kAoUE1rbTUWpCcE1MU=\n-----END PRIVATE KEY-----\n",
  "client_email": "fake@adc-project.iam.gserviceaccount.com",
  "client_id": "1234567890",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/fake%40adc-project.iam.gserviceaccount.com"
}`

// clearADCEnv unsets every env var that could let google.FindDefaultCredentials
// find real credentials, isolating tests from the host environment.
func clearADCEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"DATASTORE_EMULATOR_HOST",
		"GOOGLE_CLOUD_PROJECT",
		"DATASTORE_PROJECT_ID",
		"GOOGLE_APPLICATION_CREDENTIALS",
		"CLOUDSDK_CONFIG",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestLoad_EmulatorMode_Defaults(t *testing.T) {
	clearADCEnv(t)
	t.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8081")

	cfg, err := Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Endpoint != "http://localhost:8081" {
		t.Errorf("Endpoint = %q, want http://localhost:8081", cfg.Endpoint)
	}
	if cfg.ProjectID != "test-project" {
		t.Errorf("ProjectID = %q, want test-project", cfg.ProjectID)
	}
	if cfg.TokenSource != nil {
		t.Errorf("TokenSource = %v, want nil in emulator mode", cfg.TokenSource)
	}
}

func TestLoad_ReadOnlyFlag(t *testing.T) {
	clearADCEnv(t)
	t.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8081")

	cfg, err := Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ReadOnly {
		t.Error("ReadOnly = true, want false by default")
	}

	cfg, err = Load(context.Background(), []string{"-read-only"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.ReadOnly {
		t.Error("ReadOnly = false, want true when -read-only is passed")
	}
}

func TestLoad_EmulatorMode_FlagOverridesEnv(t *testing.T) {
	clearADCEnv(t)
	t.Setenv("DATASTORE_EMULATOR_HOST", "localhost:8081")

	cfg, err := Load(context.Background(), []string{"-endpoint", "http://localhost:9999", "-project", "my-proj"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Endpoint != "http://localhost:9999" {
		t.Errorf("Endpoint = %q, want http://localhost:9999", cfg.Endpoint)
	}
	if cfg.ProjectID != "my-proj" {
		t.Errorf("ProjectID = %q, want my-proj", cfg.ProjectID)
	}
	if cfg.TokenSource != nil {
		t.Errorf("TokenSource = %v, want nil in emulator mode", cfg.TokenSource)
	}
}

func TestLoad_RealGCP_UsesADCCredentials(t *testing.T) {
	clearADCEnv(t)

	keyPath := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(keyPath, []byte(fakeServiceAccountKey), 0o600); err != nil {
		t.Fatalf("write fake key: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyPath)

	cfg, err := Load(context.Background(), []string{"-project", "explicit-project"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Endpoint != "https://datastore.googleapis.com" {
		t.Errorf("Endpoint = %q, want https://datastore.googleapis.com", cfg.Endpoint)
	}
	if cfg.ProjectID != "explicit-project" {
		t.Errorf("ProjectID = %q, want explicit-project (flag should override ADC project)", cfg.ProjectID)
	}
	if cfg.TokenSource == nil {
		t.Error("TokenSource = nil, want non-nil in real-GCP mode")
	}
}

func TestLoad_RealGCP_FallsBackToADCProjectID(t *testing.T) {
	clearADCEnv(t)

	keyPath := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(keyPath, []byte(fakeServiceAccountKey), 0o600); err != nil {
		t.Fatalf("write fake key: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyPath)

	cfg, err := Load(context.Background(), nil)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ProjectID != "adc-project" {
		t.Errorf("ProjectID = %q, want adc-project (from the fake key)", cfg.ProjectID)
	}
}

func TestLoad_RealGCP_NoCredentials_ReturnsError(t *testing.T) {
	clearADCEnv(t)
	// Point HOME/CLOUDSDK_CONFIG somewhere with no gcloud ADC file, and leave
	// GOOGLE_APPLICATION_CREDENTIALS unset, so lookup fails deterministically.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CLOUDSDK_CONFIG", t.TempDir())

	_, err := Load(context.Background(), []string{"-project", "explicit-project"})
	if err == nil {
		t.Fatal("Load: want error when no ADC is available, got nil")
	}
}

func TestLoad_RealGCP_NoProjectID_ReturnsError(t *testing.T) {
	clearADCEnv(t)

	keyPath := filepath.Join(t.TempDir(), "key.json")
	// A key with no project_id field, so both flag/env and ADC leave it empty.
	if err := os.WriteFile(keyPath, []byte(`{"type":"service_account","private_key":"","client_email":"fake@example.com","token_uri":"https://oauth2.googleapis.com/token"}`), 0o600); err != nil {
		t.Fatalf("write fake key: %v", err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", keyPath)

	_, err := Load(context.Background(), nil)
	if err == nil {
		t.Fatal("Load: want error when no project ID is resolvable, got nil")
	}
}
