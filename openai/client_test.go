package openai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const testAPIKey = "test-key"

func TestNewClient(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", testAPIKey)
	defer os.Unsetenv("OPENAI_API_KEY")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.apiKey != "test-key" {
		t.Errorf("NewClient() apiKey = %v, want test-key", client.apiKey)
	}
}

func TestCreateEmbeddings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("Authorization header not set correctly")
		}
		w.Write([]byte(`{
			"data": [
				{
					"embedding": [0.1, 0.2, 0.3],
					"index": 0
				}
			],
			"usage": {
				"prompt_tokens": 10,
				"total_tokens": 10
			}
		}`))
	}))
	defer server.Close()

	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	client, _ := NewClient()
	client.baseURL = server.URL

	resp, err := client.CreateEmbeddings(context.Background(), []string{"test text"})
	if err != nil {
		t.Fatalf("CreateEmbeddings() error = %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("CreateEmbeddings() got %d embeddings, want 1", len(resp.Data))
	}
}
