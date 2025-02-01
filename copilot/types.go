package copilot

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
}

type Model string

const (
	ModelGPT35      Model = "gpt-3.5-turbo"
	ModelGPT4       Model = "gpt-4"
)

type ChatCompletionsRequest struct {
	Messages []ChatMessage `json:"messages"`
	Model    Model        `json:"model"`
	Stream   bool         `json:"stream"`
}