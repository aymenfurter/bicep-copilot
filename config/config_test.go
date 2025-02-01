package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	envVars := map[string]string{
		"PORT":           "8080",
		"FQDN":          "https://example.com",
		"CLIENT_ID":      "test-client",
		"CLIENT_SECRET": "test-secret",
		"REPO_OWNER":    "owner",
		"REPO_NAME":     "repo",
		"REPO_BRANCH":   "main",
		"REPO_PATH":     "docs",
		"ENVIRONMENT":   "production",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	cfg, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("New() Port = %v, want 8080", cfg.Port)
	}

	if cfg.FQDN != "https://example.com" {
		t.Errorf("New() FQDN = %v, want https://example.com", cfg.FQDN)
	}

	if !cfg.IsProduction() {
		t.Error("New() IsProduction = false, want true")
	}
}

func TestLoadEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envContent := `
PORT=9090
FQDN=http://localhost
CLIENT_ID=local-client
CLIENT_SECRET=local-secret
`
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	if err := loadEnv(); err != nil {
		t.Fatalf("loadEnv() error = %v", err)
	}

	if os.Getenv("PORT") != "9090" {
		t.Errorf("loadEnv() PORT = %v, want 9090", os.Getenv("PORT"))
	}
}
