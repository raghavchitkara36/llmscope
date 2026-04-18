package main

import (
	"context"
	"fmt"
	"log"

	ollama "github.com/ollama/ollama/api"
	"github.com/raghavchitkara36/llmscope"
)

func main() {
	ctx := context.Background()

	// 1 — init llmscope — creates SQLite at ./llmscope-traces
	scope, err := llmscope.New(llmscope.Config{
		StoragePath: "./llmscope-traces",
	})
	if err != nil {
		log.Fatalf("failed to init llmscope: %v", err)
	}
	defer scope.Close() // flushes pending traces and closes storage

	// 2 — create your Ollama client as usual
	ollamaClient, err := ollama.ClientFromEnvironment()
	if err != nil {
		log.Fatalf("failed to create ollama client: %v", err)
	}

	// 3 — wrap it with llmscope — this is the only change to your existing code
	client := scope.WrapOllama(ollamaClient, "example-project")

	// 4 — use the wrapped client exactly as before — everything is traced automatically
	fmt.Println("Sending request to Ollama...")

	var fullResponse string

	err = client.Chat(ctx, &ollama.ChatRequest{
		Model: "llama3",
		Messages: []ollama.Message{
			{
				Role:    "system",
				Content: "You are a helpful assistant. Keep responses concise.",
			},
			{
				Role:    "user",
				Content: "What is LLM observability and why does it matter?",
			},
		},
	}, func(resp ollama.ChatResponse) error {
		fmt.Print(resp.Message.Content) // stream to terminal
		fullResponse += resp.Message.Content
		return nil
	})
	if err != nil {
		log.Fatalf("chat request failed: %v", err)
	}

	fmt.Println("\n\n--- llmscope captured this trace ---")
	fmt.Printf("Response length : %d characters\n", len(fullResponse))
	fmt.Println("Trace saved to  : ./llmscope-traces/llmscope.db")
	fmt.Println("------------------------------------")
}
