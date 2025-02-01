package retrieval

import (
	"sync"
	"time"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultCacheDir  = ".bicep-copilot"
	cacheFile       = "embeddings-cache.json"
)

type Document struct {
	Path      string     `json:"path"`
	Content   string     `json:"content"`
	Embedding []float32  `json:"embedding"`
	Modified  time.Time  `json:"modified"`
}

type Cache struct {
	sync.RWMutex
	documents map[string]*Document
	loaded    bool
}

func NewCache() *Cache {
	return &Cache{
		documents: make(map[string]*Document),
	}
}

func (c *Cache) Store(doc *Document) {
	c.Lock()
	defer c.Unlock()
	c.documents[doc.Path] = doc
}

func (c *Cache) Get(path string) (*Document, bool) {
	c.RLock()
	defer c.RUnlock()
	doc, exists := c.documents[path]
	return doc, exists
}

func (c *Cache) List() []*Document {
	c.RLock()
	defer c.RUnlock()
	docs := make([]*Document, 0, len(c.documents))
	for _, doc := range c.documents {
		docs = append(docs, doc)
	}
	return docs
}

func (c *Cache) IsLoaded() bool {
	c.RLock()
	defer c.RUnlock()
	return c.loaded
}

func (c *Cache) SetLoaded() {
	c.Lock()
	defer c.Unlock()
	c.loaded = true
}

func (c *Cache) Clear() {
	c.Lock()
	defer c.Unlock()
	c.documents = make(map[string]*Document)
	c.loaded = false
}

func (c *Cache) SaveToDisk() error {
	c.RLock()
	defer c.RUnlock()

	cacheDir := filepath.Join(os.Getenv("HOME"), defaultCacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	cacheFile := filepath.Join(cacheDir, cacheFile)
	file, err := os.Create(cacheFile)
	if (err != nil) {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(c.documents)
}

func (c *Cache) LoadFromDisk() error {
	c.Lock()
	defer c.Unlock()

	cacheFile := filepath.Join(os.Getenv("HOME"), defaultCacheDir, cacheFile)
	file, err := os.Open(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil 
		}
		return fmt.Errorf("failed to open cache file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	return decoder.Decode(&c.documents)
}

type RepoConfig struct {
	Owner    string
	Repo     string
	Branch   string
	RootPath string
}