package agent

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"

	"github.com/aymenfurter/bicep-copilot/copilot"
	"github.com/aymenfurter/bicep-copilot/retrieval"
)

type Service struct {
	pubKey           *ecdsa.PublicKey
	retrievalService *retrieval.Service
}

func NewService(pubKey *ecdsa.PublicKey, retrievalService *retrieval.Service) *Service {
	return &Service{
		pubKey:           pubKey,
		retrievalService: retrievalService,
	}
}

func (s *Service) ChatCompletion(w http.ResponseWriter, r *http.Request) {
	sig := r.Header.Get("Github-Public-Key-Signature")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("failed to read request body: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	isValid, err := s.validPayload(body, sig)
	if err != nil {
		fmt.Printf("failed to validate payload signature: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !isValid {
		http.Error(w, "invalid payload signature", http.StatusUnauthorized)
		return
	}

	var req *copilot.ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		fmt.Printf("failed to unmarshal request: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	apiToken := r.Header.Get("X-GitHub-Token")
	integrationID := r.Header.Get("Copilot-Integration-Id")

	if err := s.generateCompletion(r.Context(), integrationID, apiToken, req, w); err != nil {
		fmt.Printf("failed to execute agent: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Service) findLastUserMessage(messages []copilot.ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" && messages[i].Content != "" {
			return messages[i].Content
		}
	}
	return ""
}

func (s *Service) buildContextMessage(docs []*retrieval.Document) string {
	var contextBuilder strings.Builder
	contextBuilder.WriteString("Here is some relevant documentation to help answer the question:\n\n")
	
	const maxContextLength = 100000
	currentLength := contextBuilder.Len()

	for _, doc := range docs {
		additionalLen := len(doc.Path) + len(doc.Content) + 8 
		if currentLength + additionalLen > maxContextLength {
			continue
		}

		contextBuilder.WriteString("From ")
		contextBuilder.WriteString(doc.Path)
		contextBuilder.WriteString(":\n")
		contextBuilder.WriteString(doc.Content)
		contextBuilder.WriteString("\n\n")

		currentLength += additionalLen
	}

	return contextBuilder.String()
}

func (s *Service) processStream(stream io.ReadCloser, w io.Writer) error {
	scanner := bufio.NewScanner(stream)
	for scanner.Scan() {
		if _, err := w.Write(scanner.Bytes()); err != nil {
			return fmt.Errorf("failed to write to stream: %w", err)
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return fmt.Errorf("failed to write delimiter to stream: %w", err)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("failed to read from stream: %w", err)
	}
	return nil
}

func (s *Service) generateCompletion(ctx context.Context, integrationID, apiToken string, req *copilot.ChatRequest, w io.Writer) error {
	var messages []copilot.ChatMessage

	lastUserMessage := s.findLastUserMessage(req.Messages)
	if lastUserMessage != "" {
		docs, err := s.retrievalService.FindRelevantDocuments(ctx, lastUserMessage)
		if err != nil {
			return fmt.Errorf("error finding relevant documents: %w", err)
		}

		if len(docs) > 0 {
			contextMessage := s.buildContextMessage(docs)
			messages = append(messages, copilot.ChatMessage{
				Role:    "system",
				Content: contextMessage,
			})
		}
	}

	messages = append(messages, req.Messages...)
	messages = append(messages, copilot.ChatMessage{
		Role:    "system",
		Content: "Based on the provided documentation, answer the user's question about Bicep. If you're unsure about something, acknowledge that and suggest looking at the official documentation. At the end give citations of the used resource names and versions. You may also link to docs. For instance for Microsoft.Storage/storageAccounts/queueServices/queues@2021-06-01 you may link to https://learn.microsoft.com/en-us/azure/templates/microsoft.storage/2021-06-01/storageaccounts/queueservices/queues?pivots=deployment-language-bicep - In citations use emojis.",
	})

	chatReq := &copilot.ChatCompletionsRequest{
		Model:    copilot.ModelGPT4,
		Messages: messages,
		Stream:   true,
	}

	stream, err := copilot.ChatCompletions(ctx, integrationID, apiToken, chatReq)
	if err != nil {
		return fmt.Errorf("failed to get chat completion stream: %w", err)
	}
	defer stream.Close()

	return s.processStream(stream, w)
}

type asn1Signature struct {
	R *big.Int
	S *big.Int
}

func (s *Service) validPayload(data []byte, sig string) (bool, error) {
	asnSig, err := base64.StdEncoding.DecodeString(sig)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	parsedSig := asn1Signature{}
	rest, err := asn1.Unmarshal(asnSig, &parsedSig)
	if err != nil || len(rest) != 0 {
		return false, fmt.Errorf("failed to parse signature: %w", err)
	}

	digest := sha256.Sum256(data)
	return ecdsa.Verify(s.pubKey, digest[:], parsedSig.R, parsedSig.S), nil
}