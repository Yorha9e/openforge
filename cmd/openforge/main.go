package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"openforge/internal/agent/domain"
	"openforge/internal/agent/port"
	"openforge/internal/shared/profile"
	"openforge/internal/tool"
)

func main() {
	// Subcommand dispatch (T8): `openforge migrate profile <from> <to>`
	// runs the profile backend migration and exits without entering the
	// interactive REPL. Handled before the bootstrap so the operator
	// can run the migration on a host that has a profile config but
	// is otherwise not ready to start the daemon.
	if len(os.Args) >= 4 && os.Args[1] == "migrate" && os.Args[2] == "profile" {
		from, to := os.Args[3], os.Args[4]
		configPath := os.Getenv("OF_PROFILE")
		if configPath == "" {
			configPath = "config/profiles/minimal.yaml"
		}
		// Allow `--config` to override the env-derived path.
		for i := 5; i < len(os.Args); i++ {
			if os.Args[i] == "--config" && i+1 < len(os.Args) {
				configPath = os.Args[i+1]
			}
		}

		cfg, err := profile.Load(configPath, false)
		if err != nil {
			log.Fatalf("migrate profile: load profile %s: %v", configPath, err)
		}
		db, err := sql.Open("postgres", cfg.Database.DSN())
		if err != nil {
			log.Fatalf("migrate profile: open db: %v", err)
		}
		defer db.Close()

		res, err := profile.MigrateProfileBackend(context.Background(), db, from, to)
		if err != nil {
			log.Fatalf("migrate profile %s → %s: %v", from, to, err)
		}
		for _, s := range res.Steps {
			log.Printf("step: %s", s)
		}
		for _, w := range res.Warnings {
			log.Printf("WARN: %s", w)
		}
		fmt.Printf("profile migration %s → %s complete (%d steps, %d warnings)\n",
			res.From, res.To, len(res.Steps), len(res.Warnings))
		return
	}

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

	llmClient := of.LLMRouter
	toolReg := tool.NewRegistry()
	
	// Register core tools
	toolReg.RegisterTool(&tool.ReadFileTool{})
	toolReg.RegisterTool(&tool.WriteFileTool{})
	toolReg.RegisterTool(&tool.EditFileTool{})
	toolReg.RegisterTool(&tool.ReplaceInFileTool{})
	toolReg.RegisterTool(&tool.DeleteFileTool{})
	toolReg.RegisterTool(&tool.ListDirTool{})
	toolReg.RegisterTool(&tool.SearchFileTool{})
	toolReg.RegisterTool(&tool.GrepTool{})
	toolReg.RegisterTool(&tool.GlobTool{})
	toolReg.RegisterTool(&tool.GitStatusTool{})
	toolReg.RegisterTool(&tool.GitDiffTool{})
	toolReg.RegisterTool(&tool.GitLogTool{})
	toolReg.RegisterTool(&tool.BashToolAdapter{Executor: of.CommandExec})
	
	coordinator := domain.NewCoordinator(llmClient, toolReg)

	// Phase 7: wire the in-memory embedding index into the KnowledgeQuerier
	// and the LearningService so newly learned knowledge surfaces in prompts.
	embeddingIndex := domain.NewInMemoryEmbeddingIndex()
	of.EmbeddingIndex = embeddingIndex
	if of.KnowledgeQuerier != nil {
		of.KnowledgeQuerier.SetEmbeddingIndex(embeddingIndex.AsEmbeddingIndex())
	}
	if of.LearningSvc != nil {
		of.LearningSvc.SetEmbeddingIndex(embeddingIndex)
	}

	// T10: wire the trajectory store into the priority engine so the
	// LearningFactor can pull per-project success rates.
	if of.PriorityEngine != nil && of.TrajectoryStore != nil {
		of.PriorityEngine.SetTrajectoryStore(of.TrajectoryStore)
	}

	llmConfig := port.LLMConfig{
		Provider:    cfg.LLM.DefaultProvider,
		Model:       cfg.LLM.DefaultModel,
		MaxTokens:   4096,
		Temperature: 0.7,
	}

	fmt.Println("OpenForge CLI — Phase 1 MVP")
	fmt.Printf("Profile: %s | Model: %s/%s\n", cfg.Profile, llmConfig.Provider, llmConfig.Model)
	if cfg.Ownership != nil {
		fmt.Printf("Module ownership: %d entries loaded from %s\n", len(cfg.Ownership.Modules), cfg.ModuleOwnershipPath)
	} else {
		fmt.Println("Module ownership: using PG-seeded defaults")
	}
	if of.OwnershipRepo != nil {
		fmt.Println("Ownership repository: PG-backed (module_ownership table)")
	}
	fmt.Println("Type /help for commands, /quit to exit.")
	fmt.Println()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// T12: periodic Ed25519 profile revalidation (every 24h).
	// Failures are logged; the process is intentionally not terminated because
	// operators are expected to respond to alerts and the running config is
	// still trusted until a restart.
	cfg.StartPeriodicRevalidation(ctx, 24*time.Hour, nil)

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
		for chunk := range ch {
			fmt.Print(chunk.Delta)
			fullResponse.WriteString(chunk.Delta)
		}
		fmt.Println()
		fmt.Println()

		history = append(history, port.Message{Role: "agent", Content: fullResponse.String()})
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Input error: %v\n", err)
	}
}
