# Agentic Development Guidelines

This document outlines the standards, commands, and conventions for AI agents (and human developers) working on the **ResearchHelper-CLI** repository.

## 1. Project Overview

**ResearchHelper-CLI** is a terminal-based research agent written in Go. It autonomously researches a thesis topic by iterating through a "Plan-Execute-Reflect" loop, utilizing internal tools and a direct RAG system for knowledge management.

- **Language:** Go 1.24+
- **Core Libraries:** 
  - `github.com/tmc/langchaingo` (LLM Integration)
  - `google.golang.org/adk` (Agent Development Kit - Tools)
  - `github.com/charmbracelet/bubbletea` (CLI UX - planned)
  - `github.com/pgvector/pgvector-go` (Vector Database)

## 2. Build & Test Commands

Use the provided `Makefile` for standard operations.

| Action | Command | Description |
| :--- | :--- | :--- |
| **Build** | `make build` | Compiles the binary to `bin/research-helper`. |
| **Run** | `make run` | Runs the application from `cmd/server/main.go`. |
| **Test (All)** | `make test` | Runs all tests with race detection and coverage. |
| **Lint** | `make lint` | Runs `golangci-lint` to check code quality. |
| **Format** | `make fmt` | Formats code using `go fmt` and `gofmt`. |
| **Clean** | `make clean` | Removes build artifacts and coverage files. |

### Running Specific Tests

To run a single test or a specific package, use standard `go test` commands:

```bash
# Run tests for a specific package
go test -v ./pkg/research/tools/...

# Run a specific test function
go test -v ./pkg/research/tools/ -run TestArxivSearch

# Run with race detection (RECOMMENDED)
go test -v -race ./pkg/...
```

## 3. Code Style & Conventions

Strictly adhere to these guidelines to maintain codebase consistency.

### 3.1 General Go Standards
- **Formatting:** All code must be formatted with `gofmt` (use `make fmt`).
- **Idioms:** Follow "Effective Go" guidelines.
- **Complexity:** Keep functions small and focused. break large logic into helper functions.

### 3.2 Naming Conventions
- **Files:** Use snake_case for filenames (e.g., `arxiv_api.go`, `pdf_scraper.go`).
- **Interfaces:** Interface names should usually end in `er` (e.g., `Searcher`, `Scraper`).
- **Variables/Constants:**
  - `CamelCase` for exported identifiers.
  - `mixedCase` for unexported identifiers.
  - Acronyms should be consistent (e.g., `ServeHTTP`, not `ServeHttp`; `ID`, not `Id`).

### 3.3 Error Handling
- **Explicit Checks:** Always check errors. Never ignore them using `_` unless absolutely safe.
  ```go
  // BAD
  file.Close()
  
  // GOOD
  if err := file.Close(); err != nil {
      log.Printf("failed to close file: %v", err)
  }
  ```
- **Return Errors:** Prefer returning errors over panicking. Only `panic` on unrecoverable startup errors.
- **Wrapping:** Use `fmt.Errorf("context: %w", err)` to wrap errors with context.

### 3.4 Imports
Group imports in the following order:
1. Standard Library (`"fmt"`, `"os"`)
2. Third-Party Libraries (`"github.com/..."`)
3. Local Packages (`"github.com/mikeboe/research-helper/pkg/..."`)

```go
import (
    "context"
    "fmt"
    
    "github.com/tmc/langchaingo/llms"
    
    "github.com/mikeboe/research-helper/pkg/research/tools"
)
```

### 3.5 Concurrency
- **Synchronization:** Use `sync.Mutex` for shared state protection.
- **Goroutines:** Use `sync.WaitGroup` or `errgroup` to manage concurrent tasks (e.g., scraping multiple URLs).
- **Context:** Always propagate `context.Context` as the first argument to functions performing I/O or long-running tasks.

## 4. Architecture & Patterns

### 4.1 The Research Loop
The core logic follows a state machine pattern defined in the PRD:
1. **Plan:** LLM generates queries.
2. **Source:** Internal tools (Arxiv, Web) fetch raw data.
3. **Filter:** Lightweight LLM scores relevance.
4. **Acquire & Index:** Scrape content and push to Postgres/VectorStore.
5. **Reflect:** Determine if more info is needed or if research is complete.

### 4.2 Tool Integration
- Research tools are implemented in `pkg/research/tools`.
- Chat tools are implemented in `pkg/chat/rag_tools.go` using the ADK `tool.Toolset` interface.
- Ensure tools handle errors gracefully; a tool failure should not crash the agent.

## 5. Agent Instructions (for AI Coders)

1. **Read First:** Before modifying a file, read the PRD (`docs/prd.md`) and related code to understand the context.
2. **Test Safety:** Run `make test` before and after changes to ensure no regressions.
3. **No Assumptions:** Check `go.mod` before importing new packages. If a new dependency is needed, ask the user first.
4. **Docs:** Update documentation if you change function signatures or add new features.
5. **Atomic Changes:** Keep changes focused on the requested task. Do not refactor unrelated code without permission.

---
*Generated based on repository state as of Feb 2026.*
