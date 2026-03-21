package indexer

import (
	"context"
	"fmt"
	"sync"
)

// IndexCommand is a registered backend that can execute indexing requests for
// one or more languages.
type IndexCommand interface {
	Name() string
	Supports(language string, req IndexRequest) bool
	Execute(context.Context, IndexRequest) (Result, error)
}

var (
	commandRegistryMu sync.RWMutex
	commandRegistry   []IndexCommand
)

// RegisterCommand adds an indexing backend to the global dispatch registry.
// Backends register themselves from init functions so the active build selects
// the appropriate implementations automatically.
func RegisterCommand(command IndexCommand) {
	commandRegistryMu.Lock()
	defer commandRegistryMu.Unlock()
	commandRegistry = append(commandRegistry, command)
}

// Dispatch selects the appropriate backend for the request and executes it.
func Dispatch(ctx context.Context, req IndexRequest) (Result, error) {
	req, err := NormalizeRequest(req)
	if err != nil {
		return Result{}, err
	}

	command, err := commandForRequest(req)
	if err != nil {
		return Result{}, err
	}

	return command.Execute(ctx, req)
}

// pickLanguage chooses the effective language when the request does not
// explicitly supply one.
func pickLanguage(req IndexRequest) string {
	if req.Language != "" {
		return req.Language
	}
	if len(req.Paths) == 1 {
		return DetectLanguage(req.Paths[0])
	}
	return ""
}

func commandForRequest(req IndexRequest) (IndexCommand, error) {
	language := pickLanguage(req)

	commandRegistryMu.RLock()
	defer commandRegistryMu.RUnlock()

	findByName := func(name string) IndexCommand {
		for _, command := range commandRegistry {
			if command.Name() == name {
				return command
			}
		}
		return nil
	}

	if req.Indexer != "" {
		command := findByName(req.Indexer)
		if command != nil {
			return command, nil
		}
		return nil, fmt.Errorf("no registered index command matches indexer %q", req.Indexer)
	}

	// Auto-routing priority:
	// 1) dedicated Go backend
	// 2) configured LSP backend
	// 3) any other non-fallback backend
	// 4) tree-sitter/symbolic fallback as a last resort
	if language == "go" {
		command := findByName("go")
		if command != nil && command.Supports(language, req) {
			return command, nil
		}
	}

	if req.LSPCommand != "" {
		command := findByName("scip-lsp")
		if command != nil && command.Supports(language, req) {
			return command, nil
		}
	}

	for _, command := range commandRegistry {
		if command.Name() == "tree-sitter" || command.Name() == "symbolic-fallback" {
			continue
		}
		if command.Supports(language, req) {
			return command, nil
		}
	}

	for _, command := range commandRegistry {
		if command.Name() != "tree-sitter" && command.Name() != "symbolic-fallback" {
			continue
		}
		if command.Supports(language, req) {
			return command, nil
		}
	}

	if len(commandRegistry) == 0 {
		return nil, fmt.Errorf("no index commands have been registered")
	}

	if language == "" {
		return nil, fmt.Errorf("no registered index command supports the request")
	}
	return nil, fmt.Errorf("no registered index command supports language %q", language)
}
