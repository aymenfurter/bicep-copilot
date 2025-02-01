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

func TestFindRelevantDocuments(t *testing.T) {
	service, _ := NewService(&RepoConfig{})
	
	embedding := make([]float32, 1536)
	for i := range embedding {
		embedding[i] = 0.1
	}
	
	doc := &Document{
		Path:      "test.md",
		Content:   "test content",
		Embedding: embedding,
	}
	service.cache.Store(doc)
	service.cache.SetLoaded()

	ctx := context.Background()
	_, err := service.FindRelevantDocuments(ctx, "test query")
	if err != nil {
		t.Errorf("FindRelevantDocuments() error = %v", err)
	}
}
