package llmscope

import (
	"fmt"

	openai "github.com/openai/openai-go/v3"
	"github.com/raghavchitkara36/llmscope/providers"
	"github.com/raghavchitkara36/llmscope/tracer"

	ollama "github.com/ollama/ollama/api"
	"github.com/raghavchitkara36/llmscope/storage"
)

// create a init function return new llmscope init the storage on the basis of path

type Config struct {
	StoragePath string
}

type Scope struct {
	tracer *tracer.Tracer
}

func New(cfg Config) (*Scope, error) {
	if cfg.StoragePath == "" {
		cfg.StoragePath = "./llmscope"
	}

	sqlStorage, err := storage.NewSQLiteStorage(cfg.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("llmscope: initialising storage: %w", err)
	}

	t := tracer.New(sqlStorage)

	return &Scope{tracer: t}, nil
}

func (s *Scope) WrapOpenAI(client *openai.Client, projectID string) *providers.OpenAIClient {
	return providers.WrapOpenAI(client, s.tracer, projectID)
}

func (s *Scope) WrapOllama(client *ollama.Client, projectID string) *providers.OllamaClient {
	return providers.WrapOllama(client, s.tracer, projectID)
}

func (s *Scope) Close() error {
	return s.tracer.Close()
}
