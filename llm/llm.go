package llm

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"

	"github.com/devalexandre/llmschat/database"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory/sqlite3"
)

// Client represents an LLM client interface
type Client interface {
	Chat(ctx context.Context, prompt string) (string, error)
	StreamChat(ctx context.Context, prompt string) (<-chan string, error)
}

// provider implementations
type openAIClient struct {
	client *openai.LLM
	memory *sqlite3.SqliteChatMessageHistory
}

type anthropicClient struct {
	client *anthropic.LLM
	memory *sqlite3.SqliteChatMessageHistory
}

func (o *openAIClient) Chat(ctx context.Context, prompt string) (string, error) {
	// Add user message to history
	err := o.memory.AddUserMessage(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to save user message: %v", err)
	}

	// Get completion with context from history
	completion, err := o.client.Call(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("openai chat error: %v", err)
	}

	// Save AI response to history
	err = o.memory.AddAIMessage(ctx, completion)
	if err != nil {
		return "", fmt.Errorf("failed to save AI response: %v", err)
	}

	return completion, nil
}

func (o *openAIClient) StreamChat(ctx context.Context, prompt string) (<-chan string, error) {
	stream := make(chan string)

	go func() {
		defer close(stream)

		// Add user message to history
		err := o.memory.AddUserMessage(ctx, prompt)
		if err != nil {
			stream <- fmt.Sprintf("failed to save user message: %v", err)
			return
		}

		// Stream completion with context from history
		_, err = o.client.GenerateContent(ctx, []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, prompt),
		}, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			stream <- string(chunk)
			return nil
		}))
		if err != nil {
			stream <- fmt.Sprintf("openai chat error: %v", err)
			return
		}
	}()

	return stream, nil
}

func (a *anthropicClient) Chat(ctx context.Context, prompt string) (string, error) {
	// Add user message to history
	err := a.memory.AddUserMessage(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("failed to save user message: %v", err)
	}

	// Get completion with context from history
	completion, err := a.client.Call(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("anthropic chat error: %v", err)
	}

	// Save AI response to history
	err = a.memory.AddAIMessage(ctx, completion)
	if err != nil {
		return "", fmt.Errorf("failed to save AI response: %v", err)
	}

	return completion, nil
}

func (a *anthropicClient) StreamChat(ctx context.Context, prompt string) (<-chan string, error) {
	stream := make(chan string)

	go func() {
		defer close(stream)

		// Add user message to history
		err := a.memory.AddUserMessage(ctx, prompt)
		if err != nil {
			stream <- fmt.Sprintf("failed to save user message: %v", err)
			return
		}

		// Stream completion with context from history
		_, err = a.client.GenerateContent(ctx, []llms.MessageContent{
			llms.TextParts(llms.ChatMessageTypeHuman, prompt),
		}, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			stream <- string(chunk)
			return nil
		}))
		if err != nil {
			stream <- fmt.Sprintf("anthropic chat error: %v", err)
			return
		}
	}()

	return stream, nil
}

// NewClient creates a new LLM client based on the selected model in settings
func NewClient(modelName string) (Client, error) {
	settings, err := database.GetSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to get settings: %v", err)
	}
	if settings == nil {
		return nil, fmt.Errorf("no settings found, please configure your settings first")
	}

	// Get company information
	companies, err := database.GetCompanies()
	if err != nil {
		return nil, fmt.Errorf("failed to get companies: %v", err)
	}

	var companyInfo database.Company
	for _, company := range companies {
		if company.ID == settings.CompanyID {
			companyInfo = company
			break
		}
	}

	// Initialize SQLite memory using the database path from InitDB
	dbPath := filepath.Join("data", "chat.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Failed to open database: %v", err)

	}

	mem := sqlite3.NewSqliteChatMessageHistory(sqlite3.WithDB(db))

	// Create appropriate client based on company
	switch companyInfo.Name {
	case "OpenAI":
		client, err := openai.New(
			openai.WithToken(settings.APIKey),
			openai.WithModel(modelName),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenAI client: %v", err)
		}
		return &openAIClient{client: client, memory: mem}, nil

	case "Anthropic":
		client, err := anthropic.New(
			anthropic.WithToken(settings.APIKey),
			anthropic.WithModel(modelName),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Anthropic client: %v", err)
		}
		return &anthropicClient{client: client, memory: mem}, nil

	case "Deepseek":
		client, err := openai.New(
			openai.WithToken(settings.APIKey),
			openai.WithModel(modelName),
			openai.WithBaseURL(companyInfo.BaseURL),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create Deepseek client: %v", err)
		}
		return &openAIClient{client: client, memory: mem}, nil

	default:
		return nil, fmt.Errorf("unsupported company: %s", companyInfo.Name)
	}
}

// GetResponse gets a response from the LLM
func GetResponse(prompt string, modelName string) (string, error) {
	client, err := NewClient(modelName)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return "", err
	}

	ctx := context.Background()
	return client.Chat(ctx, prompt)
}

// GetResponseStream gets a streaming response from the LLM
func GetResponseStream(prompt string, modelName string) (<-chan string, error) {
	client, err := NewClient(modelName)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		stream := make(chan string)
		close(stream)
		return stream, err
	}

	ctx := context.Background()
	return client.StreamChat(ctx, prompt)
}
