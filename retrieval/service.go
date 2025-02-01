package retrieval

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"crypto/sha256"

	"github.com/aymenfurter/bicep-copilot/openai"
)

type Service struct {
	cache         *Cache
	repoConfig    *RepoConfig
	httpClient    *http.Client
	openAI        *openai.Client
	initOnce      sync.Once
	initErr       error
	embeddingsMap sync.Map
}

func NewService(repoConfig *RepoConfig) (*Service, error) {
	openAIClient, err := openai.NewClient()
	if (err != nil) {
		return nil, fmt.Errorf("failed to create OpenAI client: %w", err)
	}

	return &Service{
		cache:      NewCache(),
		repoConfig: repoConfig,
		openAI:     openAIClient,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (s *Service) Initialize(ctx context.Context) error {
	s.initOnce.Do(func() {
		s.initErr = s.initialize(ctx)
		if s.initErr != nil {
			log.Printf("Failed to initialize service: %v", s.initErr)
		} else {
			log.Printf("Successfully initialized service with embeddings")
		}
	})
	return s.initErr
}

func (s *Service) initialize(ctx context.Context) error {
	if err := s.cache.LoadFromDisk(); err != nil {
		log.Printf("Failed to load cache from disk: %v", err)
	} else if len(s.cache.List()) > 0 {
		log.Printf("Loaded %d documents from cache", len(s.cache.List()))
		s.cache.SetLoaded()
		return nil
	}

	zipData, err := s.downloadRepo()
	if err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}

	tmpDir, err := ioutil.TempDir("", "bicep-docs")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := s.extractZip(zipData, tmpDir); err != nil {
		return fmt.Errorf("failed to extract files: %w", err)
	}

	repoDir := filepath.Join(tmpDir, fmt.Sprintf("%s-%s", s.repoConfig.Repo, s.repoConfig.Branch))
	rootDir := filepath.Join(repoDir, s.repoConfig.RootPath)

	docs, err := s.processDirectory(rootDir)
	if err != nil {
		return fmt.Errorf("failed to process directory: %w", err)
	}

	if err := s.generateEmbeddings(ctx, docs); err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	for _, doc := range docs {
		s.cache.Store(doc)
	}
	s.cache.SetLoaded()

	log.Printf("Successfully initialized %d documents with embeddings", len(docs))

	if err := s.cache.SaveToDisk(); err != nil {
		log.Printf("Failed to save cache to disk: %v", err)
	}

	return nil
}

func (s *Service) generateEmbeddings(ctx context.Context, docs []*Document) error {
	batchSize := 5
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}

		if (i/batchSize)%6 == 0 {
			log.Printf("Generating embeddings for documents %d-%d out of %d", i, end, len(docs))
		}

		batch := docs[i:end]
		inputs := make([]string, len(batch))
		for j, doc := range batch {
			inputs[j] = doc.Content
		}

		resp, err := s.openAI.CreateEmbeddings(ctx, inputs)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for batch: %w", err)
		}

		for j, data := range resp.Data {
			batch[j].Embedding = data.Embedding
		}

		if end < len(docs) {
			time.Sleep(10 * time.Millisecond)
		}
	}

	return nil
}

func (s *Service) FindRelevantDocuments(ctx context.Context, query string) ([]*Document, error) {
	if !s.cache.IsLoaded() {
		return nil, fmt.Errorf("service not initialized")
	}

	queryHash := fmt.Sprintf("%x", sha256.Sum256([]byte(query)))
	if cachedEmbedding, ok := s.embeddingsMap.Load(queryHash); ok {
		return s.findSimilarDocuments(cachedEmbedding.([]float32))
	}

	resp, err := s.openAI.CreateEmbeddings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no embedding generated for query")
	}

	queryEmbedding := resp.Data[0].Embedding
	s.embeddingsMap.Store(queryHash, queryEmbedding)

	return s.findSimilarDocuments(queryEmbedding)
}

func (s *Service) findSimilarDocuments(queryEmbedding []float32) ([]*Document, error) {
	docs := s.cache.List()
	scored := make([]struct {
		doc   *Document
		score float32
	}, len(docs))

	for i, doc := range docs {
		score := cosineSimilarity(queryEmbedding, doc.Embedding)
		scored[i] = struct {
			doc   *Document
			score float32
		}{doc, score}
	}

	quicksortBySimilarity(scored)

	resultCount := 3
	if len(scored) < resultCount {
		resultCount = len(scored)
	}

	results := make([]*Document, resultCount)
	for i := 0; i < resultCount; i++ {
		results[i] = scored[i].doc
	}

	return results, nil
}

func (s *Service) downloadRepo() ([]byte, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.zip",
		s.repoConfig.Owner, s.repoConfig.Repo, s.repoConfig.Branch)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download zip: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}

func (s *Service) extractZip(zipData []byte, dest string) error {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return err
	}

	for _, f := range reader.File {
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processDirectory(root string) ([]*Document, error) {
	var docs []*Document

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		docs = append(docs, &Document{
			Path:     relPath,
			Content:  string(content),
			Modified: info.ModTime(),
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	return docs, nil
}

func cosineSimilarity(a, b []float32) float32 {
	var dot, magA, magB float32
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}
	
	if magA == 0 || magB == 0 {
		return 0
	}
	
	return dot / (sqrt32(magA) * sqrt32(magB))
}

func sqrt32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}

func quicksortBySimilarity(items []struct {
	doc   *Document
	score float32
}) {
	if len(items) < 2 {
		return
	}

	left, right := 0, len(items)-1
	pivot := len(items) / 2

	items[pivot], items[right] = items[right], items[pivot]

	for i := range items {
		if items[i].score > items[right].score {
			items[i], items[left] = items[left], items[i]
			left++
		}
	}

	items[left], items[right] = items[right], items[left]

	quicksortBySimilarity(items[:left])
	quicksortBySimilarity(items[left+1:])
}