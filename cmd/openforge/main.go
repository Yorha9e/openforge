package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"openforge/internal/agent/adapter"
	"openforge/internal/agent/domain"
	"openforge/internal/agent/port"
	"openforge/internal/shared/profile"
)

func main() {
	configPath := "config/profiles/minimal.yaml"
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "--config" && i+1 < len(os.Args) {
			configPath = os.Args[i+1]
		}
	}

	cfg, err := profile.Load(configPath, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load profile: %v\n", err)
		os.Exit(1)
	}

	of, err := profile.Bootstrap(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	apiKey, err := of.Secrets.Get(context.Background(), "ANTHROPIC_API_KEY")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ANTHROPIC_API_KEY not set: %v\n", err)
		os.Exit(1)
	}

	llmClient, err := adapter.NewLLMClient(cfg.GRPC.NodejsIOAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to LLM router: %v\n", err)
		os.Exit(1)
	}
	defer llmClient.Close()

	coordinator := domain.NewCoordinator(llmClient)

	llmConfig := port.LLMConfig{
		Provider:    cfg.LLM.DefaultProvider,
		Model:       cfg.LLM.DefaultModel,
		APIKey:      string(apiKey),
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	fmt.Println("OpenForge CLI — Phase 1 MVP")
	fmt.Printf("Profile: %s | Model: %s/%s\n", cfg.Profile, llmConfig.Provider, llmConfig.Model)
	fmt.Println("Type /help for commands, /quit to exit.")
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	scanner := bufio.NewScanner(os.Stdin)
	var history []port.Message
	history = append(history, port.Message{Role: "system", Content: "You are an AI engineering assistant. Respond concisely in Chinese."})

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch input {
		case "/quit", "/q":
			fmt.Println("Goodbye.")
			return
		case "/help":
			fmt.Println("Commands: /quit, /help, /clear")
			fmt.Println("Type a natural language request to chat with the AI agent.")
			continue
		case "/clear":
			history = history[:1]
			fmt.Println("Context cleared.")
			continue
		}

		history = append(history, port.Message{Role: "user", Content: input})

		fmt.Print("\nAgent: ")
		ch, err := coordinator.ChatStream(ctx, history, llmConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			continue
		}

		var fullResponse strings.Builder
		for text := range ch {
			fmt.Print(text)
			fullResponse.WriteString(text)
		}
		fmt.Println()
		fmt.Println()

		history = append(history, port.Message{Role: "agent", Content: fullResponse.String()})
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
	}
}
