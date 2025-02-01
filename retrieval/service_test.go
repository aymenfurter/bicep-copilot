package retrieval

import (
	"context"
	"testing"
)

func TestNewService(t *testing.T) {
	config := &RepoConfig{
		Owner:    "test",
		Repo:     "repo",
		Branch:   "main",
		RootPath: "docs",
	}

	service, err := NewService(config)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if service.cache == nil {
		t.Error("NewService() cache is nil")
	}

	if service.repoConfig != config {
		t.Errorf("NewService() repoConfig = %v, want %v", service.repoConfig, config)
	}
}