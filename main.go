package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aymenfurter/bicep-copilot/agent"
	"github.com/aymenfurter/bicep-copilot/config"
	"github.com/aymenfurter/bicep-copilot/oauth"
	"github.com/aymenfurter/bicep-copilot/retrieval"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	if os.Getenv("OPENAI_API_KEY") == "" {
		return fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}

	pubKey, err := fetchPublicKey()
	if err != nil {
		return fmt.Errorf("failed to fetch public key: %w", err)
	}

	callbackURL, err := url.Parse(cfg.FQDN)
	if err != nil {
		return fmt.Errorf("invalid FQDN: %w", err)
	}
	callbackURL.Path = "auth/callback"

	oauthService := oauth.NewService(cfg.ClientID, cfg.ClientSecret, callbackURL.String())
	http.HandleFunc("/auth/authorization", oauthService.PreAuth)
	http.HandleFunc("/auth/callback", oauthService.PostAuth)

	repoConfig := &retrieval.RepoConfig{
		Owner:    cfg.RepoOwner,
		Repo:     cfg.RepoName,
		Branch:   cfg.RepoBranch,
		RootPath: cfg.RepoPath,
	}

	retrievalService, err := retrieval.NewService(repoConfig)
	if err != nil {
		return fmt.Errorf("failed to create retrieval service: %w", err)
	}

	log.Println("Initializing document embeddings...")
	startTime := time.Now()
	if err := retrievalService.Initialize(context.Background()); err != nil {
		return fmt.Errorf("failed to initialize documents: %w", err)
	}
	log.Printf("Document embeddings initialized in %v", time.Since(startTime))

	agentService := agent.NewService(pubKey, retrievalService)

	http.HandleFunc("/agent", agentService.ChatCompletion)

	addr := ":" + cfg.Port
	server := &http.Server{
		Addr:         addr,
		Handler:      nil,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Server starting on port %s", cfg.Port)
	return server.ListenAndServe()
}

func fetchPublicKey() (*ecdsa.PublicKey, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get("https://api.github.com/meta/public_keys/copilot_api")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var respBody struct {
		PublicKeys []struct {
			Key       string `json:"key"`
			IsCurrent bool   `json:"is_current"`
		} `json:"public_keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var currentKey string
	for _, pk := range respBody.PublicKeys {
		if pk.IsCurrent {
			currentKey = pk.Key
			break
		}
	}

	if currentKey == "" {
		return nil, fmt.Errorf("no current public key found")
	}

	pemStr := strings.ReplaceAll(currentKey, "\\n", "\n")
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not ECDSA")
	}

	return ecdsaKey, nil
}