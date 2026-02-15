package chat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mikeboe/research-helper/pkg/config"
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
	config   *config.Config
}

func NewRagToolset(db *database.PostgresDB, embedder *embeddings.GoogleEmbedder, config *config.Config) *RagToolset {
	return &RagToolset{
		DB:       db,
		Embedder: embedder,
		config:   config,
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
		t.searchContentTool,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create search tool: %w", err)
	}

	findBySourceTool, err := functiontool.New[FindSourceArgs, FindSourceResp](
		functiontool.Config{
			Name:        "find_content_by_source",
			Description: "Find all content associated with a specific source URL.",
		},
		t.findContentBySourceTool,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create find_by_source tool: %w", err)
	}

	findByMetadataTool, err := functiontool.New[FindMetadataArgs, FindMetadataResp](
		functiontool.Config{
			Name:        "find_content_by_metadata",
			Description: "Find content using complex logical filters on metadata.",
		},
		t.findContentByMetadataTool,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create find_by_metadata tool: %w", err)
	}

	return []tool.Tool{searchTool, findBySourceTool, findByMetadataTool}, nil
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

// Wrapper for ADK tool interface
func (t *RagToolset) searchContentTool(ctx tool.Context, args SearchContentArgs) (SearchContentResp, error) {
	return t.SearchContent(ctx, args)
}

// Public method using standard context
func (t *RagToolset) SearchContent(ctx context.Context, args SearchContentArgs) (SearchContentResp, error) {
	if args.TopK == 0 {
		args.TopK = 5
	}
	collection := t.config.CollectionName

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

// Wrapper for ADK tool interface
func (t *RagToolset) findContentBySourceTool(ctx tool.Context, args FindSourceArgs) (FindSourceResp, error) {
	return t.FindContentBySource(ctx, args)
}

// Public method using standard context
func (t *RagToolset) FindContentBySource(ctx context.Context, args FindSourceArgs) (FindSourceResp, error) {
	collection := t.config.CollectionName

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

type FindMetadataArgs struct {
	Filter map[string]interface{} `json:"filter" description:"JSON filter object with logical operators ($and, $or, $not)"`
}

type FindMetadataResp struct {
	Content string `json:"content"`
}

// Wrapper for ADK tool interface
func (t *RagToolset) findContentByMetadataTool(ctx tool.Context, args FindMetadataArgs) (FindMetadataResp, error) {
	return t.FindContentByMetadata(ctx, args)
}

// Public method using standard context
func (t *RagToolset) FindContentByMetadata(ctx context.Context, args FindMetadataArgs) (FindMetadataResp, error) {
	collection := t.config.CollectionName

	store, err := vectorstore.NewPGVectorStore(t.DB.Pool, collection)
	if err != nil {
		return FindMetadataResp{}, fmt.Errorf("invalid collection name: %w", err)
	}

	results, err := store.GetContentByMetadata(ctx, args.Filter)
	if err != nil {
		return FindMetadataResp{}, fmt.Errorf("failed to find content: %w", err)
	}

	var formattedResults []string
	for _, result := range results {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("[Content]: %s", result.Content))
		for k, v := range result.Metadata {
			sb.WriteString(fmt.Sprintf("\n[%s]: %v", k, v))
		}
		formattedResults = append(formattedResults, sb.String())
	}

	serialized := strings.Join(formattedResults, "\n\n")
	return FindMetadataResp{Content: serialized}, nil
}
