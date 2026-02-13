# PRD: Agentic Thesis Research Helper (Go-CLI)

## 1. Overview
**Product Name:** ResearchHelper-CLI
**Description:** A terminal-based research agent written in Go. It autonomously researches a thesis topic by iterating through an internal "Plan-Execute-Reflect" loop. It utilizes an existing MCP Server for tool execution (searching/scraping) and an HTTP-based RAG Server for indexing knowledge.
**Primary Goal:** To generate a comprehensive research summary report while simultaneously populating a vector database for future retrieval.

---

## 2. System Architecture

The application acts as the **Orchestrator** (Brain). It connects to an LLM for reasoning and delegates tasks to external servers.

### Components
1.  **ResearchHelper (Go CLI):** The main application containing the agentic loop logic.
2.  **LLM Provider:** (e.g., Gemini/Anthropic API) used by the Go CLI for planning, filtering, and summarization.
3.  **MCP Server (Existing):** A subprocess or local server providing:
    *   `arxiv_search`
    *   `semantic_scholar_search`
    *   `web_search`
    *   `pdf_scrape`
    *   `web_scrape`
4.  **RAG Service (Existing):** An HTTP endpoint (`POST /index`) for storing chunked text.

---

## 3. Functional Requirements

### 3.1 Input & Configuration
*   **CLI Input:** The app accepts a string argument or prompts the user: `Enter research topic:`.
*   **Config:** Must load configuration (env vars or `.env`) for:
    *   LLM API Key.
    *   MCP Server connection details (command to run or WebSocket URL).
    *   RAG Indexing Endpoint (e.g., `http://localhost:3000/index`).
    *   Target Collection Name (e.g., `thesis_v1`).

### 3.2 The Research Loop (Core Logic)
The Agent must implement a state machine with the following phases.

#### Phase A: Planning
*   **Logic:** The LLM analyzes the user input and current knowledge state.
*   **Output:** Generates a set of 2-3 specific search queries.
*   **Constraint:** If it's the first iteration, generate broad academic queries. If later iterations, generate specific queries to fill knowledge gaps.

#### Phase B: Sourcing (Tool Execution via MCP)
*   **Logic:** Call MCP tools based on the plan.
    *   Call `arxiv_search` / `semantic_scholar_search` for academic rigor.
    *   Call `web_search` for general context or recent news.
*   **Aggregation:** Collect top 5-10 results per tool.

#### Phase C: Filtering (The Triage)
*   **Logic:** Send abstracts/snippets to a lightweight LLM (cheaper model).
*   **Task:** Score relevance (0-10). Filter out anything < 7 or duplicates.
*   **User Feedback:** Print to console: *"Found 20 papers, filtered down to 4 relevant sources."*

#### Phase D: Acquisition & Indexing (Parallel Execution)
*   **Concurrency:** Use Go Routines (`sync.WaitGroup`) to process the filtered list in parallel.
*   **Step 1 (Scrape):** Call MCP `pdf_scrape` (for Arxiv/S2 URLs) or `web_scrape`.
*   **Step 2 (Index - RAG):** Send the raw text to the RAG HTTP endpoint.
    *   **Payload Construction:**
        ```json
        {
          "source": "<filename_or_url>",
          "content": "<full_scraped_text>",
          "collection": "<config_collection_name>",
          "chunkSize": 1000,
          "chunkOverlap": 200,
          "sourceMeta": {
            "name": "<author_year_slug>",
            "title": "<paper_title>"
          }
        }
        ```
*   **Step 3 (Summarize):** Generate a brief summary of the specific document for the Agent's "Short term memory."

#### Phase E: Reflection
*   **Logic:** The Agent reviews the summaries gathered in Phase D against the original User Goal.
*   **Decision:**
    *   *Continue:* "I am missing information about X." -> Return to Phase A with new queries.
    *   *Stop:* "I have sufficient information." -> Proceed to Reporting.
*   **Limit:** Hard limit of `N` iterations (default 5) to prevent infinite loops.

### 3.3 Output Generation
*   **Final Report:** The LLM compiles all "Short term memory" summaries into a structured Markdown report (Introduction, Key Findings, Methodology discussions, Conclusion).
*   **Artifacts:**
    *   Save report to `report_[timestamp].md`.
    *   Save a list of all indexed sources to `sources.json`.

---

## 4. Technical Specifications (Go)

### 4.1 Libraries & Stack
*   **Language:** Go 1.22+
*   **CLI UX:** `github.com/charmbracelet/bubbletea` (for nice TUI, spinners, and progress logs) or `github.com/spf13/cobra` (for flags).
*   **MCP Client:** Use your existing Go client implementation. Ensure it handles tool call arguments correctly.
*   **LLM Client:** `github.com/tmc/langchaingo` (or standard `net/http` if using a different provider).

### 4.2 Data Structures (State)
```go
type ResearchState struct {
    Topic             string
    CollectionName    string
    ProcessedURLs     map[string]bool // To avoid re-scraping/re-indexing
    AccumulatedFacts  []string        // Bullet points for the final report
    Iteration         int
    MaxIterations     int
    Mu                sync.Mutex      // For thread-safe updates during scraping
}
```

### 4.3 Error Handling
*   **MCP Failures:** If `pdf_scrape` fails on a specific URL, log the error to console ("Failed to parse PDF X") but **do not crash**. Continue to the next source.
*   **RAG Failures:** If the HTTP POST to the RAG server fails, log a warning. The research report can still be generated, but the knowledge base will be incomplete.

---

## 5. User Experience (Terminal Flow)

**Start:**
```bash
$ ./thesis-helper --topic "Impact of LoRA on LLM Fine-tuning" --collection "thesis_db"
```

**Runtime (Progress Output):**
```text
> [PLANNING] Breaking down topic into sub-queries...
> [SEARCHING] Querying Arxiv for "Low Rank Adaptation"...
> [SEARCHING] Querying Semantic Scholar for "Peft methods"...
> [FILTERING] Reviewed 24 abstracts. Selected 5 high-relevance papers.
> [SCRAPING] (1/5) Downloading "Hu et al. 2021"...
> [INDEXING] (1/5) Sending "Hu et al. 2021" to RAG Database... [OK]
> [SCRAPING] (2/5) Downloading "LoRA vs QLoRA"...
> [INDEXING] (2/5) Sending "LoRA vs QLoRA" to RAG Database... [OK]
> [REFLECTING] Gap identified: Need more info on "QLoRA quantization types".
> [LOOP 2] Starting new search cycle...
```

**Completion:**
```text
> [COMPLETE] Research finished in 3 iterations.
> [OUTPUT] Report saved to ./reports/lora_research.md
> [RAG] 14 Documents indexed to collection "thesis_db".
```

---

## 6. Development Milestones

1.  **Skeleton:** Setup Go project, Cobra CLI entry point, and LLM connection.
2.  **MCP Connection:** Verify the Go app can call `arxiv_search` on the local MCP server.
3.  **RAG Connection:** Implement the `indexDocument(payload)` function and verify with a dummy string.
4.  **The Loop (Single Thread):** Implement Plan -> Search -> Filter -> Index -> Report (linear).
5.  **Concurrency:** Make the "Fetch & Index" step concurrent using Goroutines.
6.  **Refinement:** Improve the "Reflection" prompt to ensure the loop stops intelligently.
