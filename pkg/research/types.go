package research

import "sync"

// Config holds runtime configuration
type Config struct {
	LLMApiKey   string
	MCPBaseURL  string
	RAGEndpoint string
	Collection  string
}

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// ResearchState tracks the progress of the research
type ResearchState struct {
	Topic            string
	CollectionName   string
	ProcessedURLs    map[string]bool
	AccumulatedFacts []string
	IndexedItems     []SearchResult // Track indexed items for final report
	Iteration        int
	MaxIterations    int
	Mu               sync.Mutex // For thread-safe updates during scraping
}

// RagPayload defines the structure for indexing documents
type RagPayload struct {
	Source       string                 `json:"source"`
	Content      string                 `json:"content"`
	Collection   string                 `json:"collection"`
	ChunkSize    int                    `json:"chunkSize"`
	ChunkOverlap int                    `json:"chunkOverlap"`
	SourceMeta   map[string]interface{} `json:"sourceMeta"`
}
