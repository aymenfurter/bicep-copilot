package retrieval

import (
	"os"
	"testing"
	"time"
)

func TestCache(t *testing.T) {
	cache := NewCache()
	doc := &Document{
		Path:     "test.md",
		Content:  "test content",
		Modified: time.Now(),
	}

	cache.Store(doc)

	if got, exists := cache.Get("test.md"); !exists || got != doc {
		t.Errorf("Cache.Get() = %v, %v, want %v, true", got, exists, doc)
	}

	docs := cache.List()
	if len(docs) != 1 || docs[0] != doc {
		t.Errorf("Cache.List() = %v, want [%v]", docs, doc)
	}

	if cache.IsLoaded() {
		t.Error("Cache.IsLoaded() = true, want false")
	}

	cache.SetLoaded()
	if !cache.IsLoaded() {
		t.Error("Cache.IsLoaded() = false, want true")
	}

	cache.Clear()
	if cache.IsLoaded() || len(cache.List()) != 0 {
		t.Error("Cache.Clear() did not clear the cache")
	}
}

func TestCachePersistence(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	cache := NewCache()
	doc := &Document{
		Path:     "test.md",
		Content:  "test content",
		Modified: time.Now(),
		Embedding: []float32{0.1, 0.2, 0.3},
	}

	cache.Store(doc)

	if err := cache.SaveToDisk(); err != nil {
		t.Fatalf("SaveToDisk() error = %v", err)
	}

	newCache := NewCache()
	if err := newCache.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk() error = %v", err)
	}

	if got, exists := newCache.Get("test.md"); !exists {
		t.Error("LoadFromDisk() did not load stored document")
	} else if got.Content != doc.Content {
		t.Errorf("LoadFromDisk() content = %v, want %v", got.Content, doc.Content)
	}
}
