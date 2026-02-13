package chat

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/mikeboe/research-helper/pkg/database"
	"github.com/mikeboe/research-helper/pkg/embeddings"
	"github.com/mikeboe/research-helper/pkg/vectorstore"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type RagToolset struct {
	DB       *database.PostgresDB
	Embedder *embeddings.GoogleEmbedder
}

func NewRagToolset(db *database.PostgresDB, embedder *embeddings.GoogleEmbedder) *RagToolset {
	return &RagToolset{
		DB:       db,
		Embedder: embedder,
	}
}

func (t *RagToolset) Name() string {
	return "rag_tools"
}

func (t *RagToolset) Tools(ctx agent.ReadonlyContext) ([]tool.Tool, error) {
	searchTool, err := functiontool.New[SearchContentArgs, SearchContentResp](
		functiontool.Config{
			Name:        "search_content",
			Description: "Search for content in the research database using semantic search.",
		},
		t.searchContent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create search tool: %w", err)
	}

	findBySourceTool, err := functiontool.New[FindSourceArgs, FindSourceResp](
		functiontool.Config{
			Name:        "find_content_by_source",
			Description: "Find all content associated with a specific source URL.",
		},
		t.findContentBySource,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create find_by_source tool: %w", err)
	}

	return []tool.Tool{searchTool, findBySourceTool}, nil
}

// --- Tool Implementations ---

type SearchContentArgs struct {
	Query  string `json:"query" description:"The search query"`
	TopK   int    `json:"topK,omitempty" description:"Number of results to return (default 5)"`
	Source string `json:"source,omitempty" description:"Optional source filter"`
}

type SearchContentResp struct {
	Results string `json:"results"`
}

func (t *RagToolset) searchContent(ctx tool.Context, args SearchContentArgs) (SearchContentResp, error) {
	if args.TopK == 0 {
		args.TopK = 5
	}
	collection := "thesis" // Or passed in args? Defaulting to common collection.

	slog.Info("Search content", "query", args.Query, "topK", args.TopK, "source", args.Source)

	// Generate embedding for query
	queryEmbedding, err := t.Embedder.EmbedText(ctx, args.Query)
	if err != nil {
		return SearchContentResp{}, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Search vector store
	store, err := vectorstore.NewPGVectorStore(t.DB.Pool, collection)
	if err != nil {
		return SearchContentResp{}, fmt.Errorf("invalid collection name: %w", err)
	}

	results, err := store.SimilaritySearch(ctx, queryEmbedding, args.TopK, args.Source)
	if err != nil {
		return SearchContentResp{}, fmt.Errorf("failed to search: %w", err)
	}

	slog.Info("Search results", "results", results)

	// Format results
	var formattedResults []string
	for _, result := range results {
		resSource := "unknown"
		if s, ok := result.Document.Metadata["source"].(string); ok {
			resSource = s
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("[Source]: %s\n[Content]: %s", resSource, result.Document.Content))

		for k, v := range result.Document.Metadata {
			if k == "source" {
				continue
			}
			// Clean up output if needed
			sb.WriteString(fmt.Sprintf("\n[%s]: %v", k, v))
		}

		formattedResults = append(formattedResults, sb.String())
	}

	serialized := strings.Join(formattedResults, "\n\n")
	return SearchContentResp{Results: serialized}, nil
}

type FindSourceArgs struct {
	Source string `json:"source" description:"The source URL to find content for"`
}

type FindSourceResp struct {
	Content string `json:"content"`
}

func (t *RagToolset) findContentBySource(ctx tool.Context, args FindSourceArgs) (FindSourceResp, error) {
	collection := "thesis_db"

	store, err := vectorstore.NewPGVectorStore(t.DB.Pool, collection)
	if err != nil {
		return FindSourceResp{}, fmt.Errorf("invalid collection name: %w", err)
	}

	results, err := store.GetContentBySource(ctx, args.Source)
	if err != nil {
		return FindSourceResp{}, fmt.Errorf("failed to find content: %w", err)
	}

	// Format results
	var formattedResults []string
	for _, result := range results {
		formattedResults = append(formattedResults, result.Content)
	}

	serialized := strings.Join(formattedResults, "\n\n")
	return FindSourceResp{Content: serialized}, nil
}
