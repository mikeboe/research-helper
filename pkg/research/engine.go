package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tmc/langchaingo/llms"

	"github.com/mikeboe/research-helper/pkg/clients"
	"github.com/mikeboe/research-helper/pkg/database"
	"github.com/mikeboe/research-helper/pkg/embeddings"
	"github.com/mikeboe/research-helper/pkg/research/tools"
	"github.com/mikeboe/research-helper/pkg/splitter"
	"github.com/mikeboe/research-helper/pkg/vectorstore"
)

type ResearchEngine struct {
	Config        Config
	State         *ResearchState
	LLM           llms.Model
	DB            *database.PostgresDB
	Embedder      *embeddings.GoogleEmbedder
	Logger        *slog.Logger
	OnStateUpdate func(state ResearchState)
}

func NewEngine(cfg Config, db *database.PostgresDB) (*ResearchEngine, error) {
	// Initialize LLM
	llm, err := clients.GoogleAi(clients.DefaultModel)
	if err != nil {
		return nil, fmt.Errorf("failed to init LLM: %w", err)
	}

	// Initialize Embedder
	// Assuming LLMApiKey is same for embeddings or we use env var inside NewGoogleEmbedder if empty
	// Note: pkg/embeddings/google.go NewGoogleEmbedder takes apiKey as argument.
	// We might need to ensure cfg.LLMApiKey is set or get from env.
	apiKey := cfg.LLMApiKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}

	embedder, err := embeddings.NewGoogleEmbedder(context.Background(), "gemini-embedding-001", apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to init embedder: %w", err)
	}

	return &ResearchEngine{
		Config: cfg,
		State: &ResearchState{
			Topic:            "", // will be set in Run
			CollectionName:   cfg.Collection,
			ProcessedURLs:    make(map[string]bool),
			AccumulatedFacts: []string{},
			IndexedItems:     []SearchResult{},
			Iteration:        0,
			MaxIterations:    5,
		},
		LLM:      llm,
		DB:       db,
		Embedder: embedder,
		Logger:   slog.Default(),
	}, nil
}

// generateWithRetry attempts to generate content and validates it using the provided function.
// It retries up to 3 times if the LLM fails or the validator returns an error.
func (e *ResearchEngine) generateWithRetry(ctx context.Context, prompts []llms.MessageContent, validator func(string) error) (string, error) {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			e.Logger.Warn("Retrying LLM generation", "attempt", i+1, "last_error", lastErr)
			time.Sleep(time.Second * time.Duration(i)) // Linear backoff
		}

		resp, err := e.LLM.GenerateContent(ctx, prompts, llms.WithJSONMode())
		if err != nil {
			lastErr = fmt.Errorf("llm generation failed: %w", err)
			continue
		}

		if len(resp.Choices) == 0 {
			lastErr = fmt.Errorf("llm returned no choices")
			continue
		}

		content := resp.Choices[0].Content
		if err := validator(content); err != nil {
			lastErr = fmt.Errorf("validation failed: %w", err)
			continue
		}

		return content, nil
	}

	return "", fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

func (e *ResearchEngine) Run(ctx context.Context, topic string) (string, error) {
	e.State.Topic = topic
	e.Logger.Info("Starting research loop", "topic", topic)

	if e.OnStateUpdate != nil {
		e.OnStateUpdate(*e.State)
	}

	for e.State.Iteration < e.State.MaxIterations {
		e.State.Iteration++
		e.Logger.Info("Starting iteration", "iteration", e.State.Iteration, "max", e.State.MaxIterations)

		if e.OnStateUpdate != nil {
			e.OnStateUpdate(*e.State)
		}

		// 1. Plan
		queries, err := e.planPhase(ctx)
		if err != nil {
			return "", fmt.Errorf("planning failed: %w", err)
		}
		if len(queries) == 0 {
			e.Logger.Warn("No queries generated. Research might be stuck.")
			break
		}

		// 2. Source
		searchResults, err := e.sourcePhase(ctx, queries)
		if err != nil {
			return "", fmt.Errorf("sourcing failed: %w", err)
		}

		// 3. Filter
		relevantItems, err := e.filterPhase(ctx, searchResults)
		if err != nil {
			return "", fmt.Errorf("filtering failed: %w", err)
		}

		if len(relevantItems) == 0 {
			e.Logger.Info("No relevant items found in this iteration.")
			// Don't break, maybe reflection will change direction
		}

		// 4. Acquire & Index
		summaries, err := e.acquireAndIndexPhase(ctx, relevantItems)
		if err != nil {
			return "", fmt.Errorf("acquire/index failed: %w", err)
		}

		if e.OnStateUpdate != nil {
			e.OnStateUpdate(*e.State)
		}

		// 5. Reflect
		shouldContinue, newFocus, err := e.reflectPhase(ctx, summaries)
		if err != nil {
			return "", fmt.Errorf("reflection failed: %w", err)
		}

		if !shouldContinue {
			e.Logger.Info("Research complete!")
			break
		}

		if newFocus != "" {
			e.Logger.Info("Adjusting focus", "focus", newFocus)
			// Ideally we'd update context or state here
		}
	}

	// Generate Final Report
	return e.generateReport(ctx)
}

// --- Phase Implementations ---

func (e *ResearchEngine) planPhase(ctx context.Context) ([]string, error) {
	e.Logger.Info("Starting planning phase")

	systemPrompt := `You are a research planner.
Generate 3 specific search queries to gather information about the topic.`

	schema := CreateSearchQueriesSchema()

	input := fmt.Sprintf(`Topic: %s
Current Iteration: %d
Accumulated Facts: %d`, e.State.Topic, e.State.Iteration, len(e.State.AccumulatedFacts))

	// Define a struct to match the JSON schema
	type QueryResponse struct {
		Queries []string `json:"queries"`
	}
	var queryResp QueryResponse

	// Use retry mechanism
	_, err := e.generateWithRetry(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt+"\n\n# Response Format: \n\n"+schema),
		llms.TextParts(llms.ChatMessageTypeHuman, input),
	}, func(content string) error {
		// Reset for retry
		queryResp = QueryResponse{}

		if err := json.Unmarshal([]byte(content), &queryResp); err != nil {
			return fmt.Errorf("json parse error: %w (content: %s)", err, content)
		}
		if len(queryResp.Queries) == 0 {
			return fmt.Errorf("empty queries list")
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	e.Logger.Info("Generated queries", "queries", queryResp.Queries)
	return queryResp.Queries, nil
}

func CreateSearchQueriesSchema() string {
	return `Return the JSON object directly without any formatting or additional text. The JSON object should have the following structure as defined in the schema. Make sure to answer in valid json and include all necessary properties:{
  "type": "object",
  "properties": {
    "queries": {
      "type": "array",
      "items": {
        "type": "string"
      },
      "description": "List of 3 specific search queries"
    }
  },
  "required": ["queries"]
}`
}

func (e *ResearchEngine) sourcePhase(ctx context.Context, queries []string) ([]SearchResult, error) {
	e.Logger.Info("Starting sourcing phase")
	var allResults []SearchResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, q := range queries {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()

			// Call Arxiv directly
			response, err := tools.SearchArxiv(query, 2)
			if err == nil {
				parsedResults := parseArxivOutput(response)
				e.Logger.Info("Arxiv search successful", "query", query, "count", len(parsedResults))

				mu.Lock()
				allResults = append(allResults, parsedResults...)
				mu.Unlock()
			} else {
				e.Logger.Error("Arxiv search failed", "query", query, "error", err)
			}

		}(q)
	}
	wg.Wait()

	// remove duplicates based on Title
	uniqueResults := make([]SearchResult, 0, len(allResults))
	seen := make(map[string]bool)
	for _, r := range allResults {
		if !seen[r.Title] {
			seen[r.Title] = true
			uniqueResults = append(uniqueResults, r)
		}
	}

	return uniqueResults, nil
}

func parseArxivOutput(content string) []SearchResult {
	var results []SearchResult

	// Regex to split by "# Title:" but keep the delimiter
	parts := strings.Split(content, "# Title: ")

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		lines := strings.Split(part, "\n")
		if len(lines) == 0 {
			continue
		}

		title := strings.TrimSpace(lines[0])
		summary := ""
		pdfLink := ""

		// Use regex for more robust parsing of the summary block
		summaryRegex := regexp.MustCompile(`## Summary: ([\s\S]*?)(?:\n##|$)`)
		linkRegex := regexp.MustCompile(`## PDF Link: (.*)`)

		sumMatch := summaryRegex.FindStringSubmatch(part)
		if len(sumMatch) > 1 {
			summary = strings.TrimSpace(sumMatch[1])
		}

		linkMatch := linkRegex.FindStringSubmatch(part)
		if len(linkMatch) > 1 {
			pdfLink = strings.TrimSpace(linkMatch[1])
		}

		if title != "" {
			results = append(results, SearchResult{
				Title:   title,
				URL:     pdfLink,
				Snippet: summary,
			})
		}
	}

	return results
}

func (e *ResearchEngine) filterPhase(ctx context.Context, results []SearchResult) ([]SearchResult, error) {
	e.Logger.Info("Starting filtering phase")

	if len(results) == 0 {
		return nil, nil
	}

	// Prepare batch prompt
	var papersList strings.Builder
	for i, r := range results {
		papersList.WriteString(fmt.Sprintf("ID: %d\nTitle: %s\nSummary: %s\n\n", i, r.Title, r.Snippet))
	}

	systemPrompt := `You are a research filter.
Evaluate the relevance of the following papers to the research topic.
Score each paper from 0-10 (10 being most relevant).
Return a JSON object mapping ID to score.`

	input := fmt.Sprintf("Topic: %s\n\nPapers:\n%s", e.State.Topic, papersList.String())

	schema := `{"type": "object", "properties": {"scores": {"type": "array", "items": {"type": "object", "properties": {"id": {"type": "integer"}, "score": {"type": "integer"}}, "required": ["id", "score"]}}}, "required": ["scores"]}`

	type ScoreItem struct {
		ID    int `json:"id"`
		Score int `json:"score"`
	}
	type FilterResponse struct {
		Scores []ScoreItem `json:"scores"`
	}
	var filterResp FilterResponse

	_, err := e.generateWithRetry(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt+"\n\n# Response Format:\n"+schema),
		llms.TextParts(llms.ChatMessageTypeHuman, input),
	}, func(content string) error {
		filterResp = FilterResponse{}
		if err := json.Unmarshal([]byte(content), &filterResp); err != nil {
			return fmt.Errorf("json parse error: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("llm filtering failed: %w", err)
	}

	var relevant []SearchResult
	for _, item := range filterResp.Scores {
		if item.Score >= 7 && item.ID < len(results) {
			relevant = append(relevant, results[item.ID])
			e.Logger.Info("Keeping paper", "title", results[item.ID].Title, "score", item.Score)
		}
	}

	e.Logger.Info("Filtering complete", "total", len(results), "relevant", len(relevant))
	return relevant, nil
}

func (e *ResearchEngine) acquireAndIndexPhase(ctx context.Context, items []SearchResult) ([]string, error) {
	e.Logger.Info("Starting acquire and index phase")
	var summaries []string
	var wg sync.WaitGroup
	var mu sync.Mutex // Local mutex for summaries slice

	semaphore := make(chan struct{}, 3) // Limit concurrency to 3

	// Ensure DB table exists (can happen once per engine/phase or once globally)
	// Ideally globally but here is safe too
	if err := e.DB.EnsureVectorExtension(ctx); err != nil {
		e.Logger.Error("Failed to ensure vector extension", "error", err)
		return nil, err
	}
	if err := e.DB.CreateEmbeddingsTable(ctx, e.State.CollectionName, 1536); err != nil { // 1536 for Google Embeddings
		e.Logger.Error("Failed to create embeddings table", "error", err)
		return nil, err
	}

	for _, item := range items {
		wg.Add(1)
		go func(item SearchResult) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			// Check if already processed
			e.State.Mu.Lock()
			if e.State.ProcessedURLs[item.URL] {
				e.State.Mu.Unlock()
				return
			}
			e.State.ProcessedURLs[item.URL] = true
			e.State.Mu.Unlock()

			e.Logger.Info("Scraping source", "title", item.Title, "url", item.URL)

			fullText := ""
			if item.URL != "" {
				// 1. Scrape PDF directly
				text, err := tools.ScrapePDF(item.URL)
				if err != nil {
					e.Logger.Warn("Failed to scrape, using summary", "url", item.URL, "error", err)
					fullText = item.Snippet // Fallback
				} else {
					fullText = text
				}
			}

			if fullText == "" {
				fullText = item.Snippet // Fallback
			}

			// 2. Index to RAG directly
			// Chunking
			chunkSize := 1000
			chunkOverlap := 200
			textSplitter := splitter.NewRecursiveCharacterTextSplitter(chunkSize, chunkOverlap)
			chunks, err := textSplitter.SplitText(fullText)
			if err != nil {
				e.Logger.Error("Failed to split text", "title", item.Title, "error", err)
			} else {
				// Embed and Store
				embeddings, err := e.Embedder.EmbedTexts(ctx, chunks)
				if err != nil {
					e.Logger.Error("Failed to generate embeddings", "title", item.Title, "error", err)
				} else {
					documents := make([]vectorstore.Document, len(chunks))
					for i, chunk := range chunks {
						documents[i] = vectorstore.Document{
							Content: chunk,
							Metadata: map[string]interface{}{
								"source": item.URL,
								"title":  item.Title,
							},
							Embedding: embeddings[i],
						}
					}

					store, err := vectorstore.NewPGVectorStore(e.DB.Pool, e.State.CollectionName)
					if err != nil {
						e.Logger.Error("Invalid collection name", "error", err)
					} else {
						if err := store.AddDocuments(ctx, documents); err != nil {
							e.Logger.Error("Failed to add documents to vector store", "title", item.Title, "error", err)
						}
					}
				}
			}

			// 3. Summarize (Short term memory)
			// Generate a concise summary of the full text for the agent's context
			// Safe truncation using runes to avoid invalid UTF-8
			excerpt := fullText
			runes := []rune(fullText)
			if len(runes) > 500 {
				excerpt = string(runes[:500])
			}

			summary := fmt.Sprintf("Source: %s\nSummary: %s\nExcerpts: %s...",
				item.Title,
				item.Snippet,
				excerpt)

			// Update state
			e.State.Mu.Lock()
			e.State.AccumulatedFacts = append(e.State.AccumulatedFacts, summary)
			e.State.IndexedItems = append(e.State.IndexedItems, item)
			e.State.Mu.Unlock()

			// Update local summaries (for reflection phase return)
			mu.Lock()
			summaries = append(summaries, summary)
			mu.Unlock()

		}(item)
	}

	wg.Wait()
	return summaries, nil
}

func (e *ResearchEngine) reflectPhase(ctx context.Context, summaries []string) (bool, string, error) {
	e.Logger.Info("Starting reflection phase")

	// Hard limit check
	if e.State.Iteration >= e.State.MaxIterations {
		return false, "", nil
	}

	systemPrompt := `You are a research manager.
Review the gathered facts and decide if sufficient information has been gathered to answer the original research topic comprehensively.
If yes, output "STOP".
If no, output "CONTINUE" and a brief focus area for the next iteration.`

	input := fmt.Sprintf("Topic: %s\n\nRecent Findings:\n%s\n\nTotal Iterations: %d/%d",
		e.State.Topic, strings.Join(summaries, "\n\n"), e.State.Iteration, e.State.MaxIterations)

	resp, err := e.LLM.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, input),
	})
	if err != nil {
		return false, "", err
	}

	content := resp.Choices[0].Content
	if strings.Contains(strings.ToUpper(content), "STOP") {
		return false, "", nil
	}

	return true, content, nil
}

func (e *ResearchEngine) generateReport(ctx context.Context) (string, error) {
	e.Logger.Info("Compiling final report")

	prompt := fmt.Sprintf(`Write a comprehensive research report on "%s".
Use the following gathered facts and summaries:

%s

Format as Markdown with Introduction, Key Findings, Methodology/Discussion, and Conclusion.`,
		e.State.Topic, strings.Join(e.State.AccumulatedFacts, "\n\n"))

	resp, err := e.LLM.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})
	if err != nil {
		return "", err
	}

	report := resp.Choices[0].Content

	// We no longer write to file by default in the library code, the caller (CLI/Server) should handle persistence
	// But to maintain CLI backward compatibility without major refactor, we can still write if desired,
	// OR better: we just return the string and let the CLI write it.

	// Let's keep the side effect for now for the CLI but primarily return the string
	timestamp := time.Now().Unix()
	reportFilename := fmt.Sprintf("report_%d.md", timestamp)

	if err := os.WriteFile(reportFilename, []byte(report), 0644); err != nil {
		e.Logger.Warn("failed to save report locally", "error", err)
	}

	// Save sources to JSON
	sourcesFilename := "sources.json"
	sourcesData, err := json.MarshalIndent(e.State.IndexedItems, "", "  ")
	if err == nil {
		if err := os.WriteFile(sourcesFilename, sourcesData, 0644); err != nil {
			e.Logger.Error("Failed to save sources.json", "error", err)
		} else {
			e.Logger.Info("Saved sources", "filename", sourcesFilename)
		}
	}

	e.Logger.Info("Final report generated", "length", len(report))
	return report, nil
}
