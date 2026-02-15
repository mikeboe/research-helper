package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/mikeboe/research-helper/pkg/config"
	"github.com/mikeboe/research-helper/pkg/database"
	"github.com/mikeboe/research-helper/pkg/embeddings"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/genai"
)

type Service struct {
	config *config.Config
	DB     *database.PostgresDB
	Client *genai.Client
	Agent  agent.Agent
}

type Conversation struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             uuid.UUID `json:"id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// StreamEvent represents a single event in the chat stream
type StreamEvent struct {
	Type    string      `json:"type"` // "content", "tool_call", "tool_result", "error", "done"
	Payload interface{} `json:"payload"`
}

func NewService(ctx context.Context, db *database.PostgresDB, config *config.Config) (*Service, error) {

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: config.GoogleApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	// Initialize ADK Agent
	modelClient, err := gemini.NewModel(ctx, config.ReasoningModel, &genai.ClientConfig{
		APIKey: config.GoogleApiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	// Initialize Embedder
	embedder, err := embeddings.NewGoogleEmbedder(ctx, config.EmbeddingModel, config.GoogleApiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	// Initialize RAG Toolset
	ragTools := NewRagToolset(db, embedder, config)

	researchAgent, err := llmagent.New(llmagent.Config{
		Name:        "research_helper",
		Model:       modelClient,
		Description: "A research assistant with access to RAG tools.",
		Instruction: "You are a helpful research assistant. Use the available tools to search for information and answer the user's questions based on the retrieved content. ALWAYS use search_content tool first. The answer format should be grouped by source, with a unordered list of content pieces supporting the question. the format would be: # Source: <source>, \n\n - <content>\n - <content>\n - <content>....",
		Toolsets: []tool.Toolset{
			ragTools,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return &Service{
		DB:     db,
		Client: client,
		Agent:  researchAgent,
	}, nil
}

func (s *Service) CreateConversation(ctx context.Context) (*Conversation, error) {
	id := uuid.New()
	query := `INSERT INTO conversations (id) VALUES ($1) RETURNING id, title, created_at, updated_at`

	conv := &Conversation{}
	err := s.DB.Pool.QueryRow(ctx, query, id).Scan(&conv.ID, &conv.Title, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func (s *Service) ListConversations(ctx context.Context) ([]Conversation, error) {
	query := `SELECT id, title, created_at, updated_at FROM conversations ORDER BY updated_at DESC`
	rows, err := s.DB.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		convs = append(convs, c)
	}
	return convs, nil
}

func (s *Service) GetHistory(ctx context.Context, conversationID uuid.UUID) ([]Message, error) {
	query := `SELECT id, conversation_id, role, content, created_at FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC`
	rows, err := s.DB.Pool.Query(ctx, query, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (s *Service) SendMessage(ctx context.Context, conversationID uuid.UUID, content string) (iter.Seq2[StreamEvent, error], error) {
	// 1. Save User Message
	userMsgID := uuid.New()
	_, err := s.DB.Pool.Exec(ctx,
		`INSERT INTO messages (id, conversation_id, role, content) VALUES ($1, $2, 'user', $3)`,
		userMsgID, conversationID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// 2. Setup Session and History
	sessionSvc := session.InMemoryService()
	appName := "research-helper"
	userID := "user" // Single user for now
	sessionID := conversationID.String()

	// Initialize session
	createRes, err := sessionSvc.Create(ctx, &session.CreateRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		// If session already exists, we ignore? But with InMemoryService() new instance it won't exist.
	}

	storedSession := createRes.Session

	// Hydrate history from DB
	history, err := s.GetHistory(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch history: %w", err)
	}

	// Add history to session
	for _, msg := range history {
		if msg.ID == userMsgID {
			continue // Skip the current message we just saved
		}

		role := "user"
		author := "user"
		if msg.Role == "model" {
			role = "model"
			author = "research_helper"
		}

		evt := session.NewEvent(uuid.NewString())
		evt.Author = author
		evt.LLMResponse = model.LLMResponse{
			Content: &genai.Content{
				Role: role,
				Parts: []*genai.Part{
					{Text: msg.Content},
				},
			},
		}

		sessionSvc.AppendEvent(ctx, storedSession, evt)
	}

	// 4. Run Agent
	runner, err := runner.New(runner.Config{
		AppName:        appName,
		Agent:          s.Agent,
		SessionService: sessionSvc,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create runner: %w", err)
	}

	userContent := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: content},
		},
	}

	// Return iterator
	return func(yield func(StreamEvent, error) bool) {
		slog.Info("Starting agent run", "conversation_id", conversationID)
		runCfg := agent.RunConfig{
			StreamingMode: agent.StreamingModeSSE,
		}

		// runner.Run returns iter.Seq2[*session.Event, error]
		next := runner.Run(ctx, userID, sessionID, userContent, runCfg)

		var finalResponse string

		for event, err := range next {
			if err != nil {
				slog.Error("Agent runner error", "error", err)
				yield(StreamEvent{Type: "error", Payload: err.Error()}, err)
				return
			}

			// Process event
			if event.LLMResponse.Content != nil {
				for _, part := range event.LLMResponse.Content.Parts {
					if part.Text != "" {
						slog.Debug("Agent output (text)", "text_len", len(part.Text))
						finalResponse += part.Text
						if !yield(StreamEvent{Type: "content", Payload: part.Text}, nil) {
							return
						}
					}
					if part.FunctionCall != nil {
						slog.Info("Agent tool call", "tool", part.FunctionCall.Name)
						if !yield(StreamEvent{Type: "tool_call", Payload: part.FunctionCall}, nil) {
							return
						}
					}
					if part.FunctionResponse != nil {
						slog.Info("Agent tool result", "tool", part.FunctionResponse.Name)
						if !yield(StreamEvent{Type: "tool_result", Payload: part.FunctionResponse}, nil) {
							return
						}
					}
				}
			}
		}

		slog.Info("Agent run completed")

		// 5. Save Model Message to DB after stream completion
		modelMsgID := uuid.New()
		_, err := s.DB.Pool.Exec(ctx,
			`INSERT INTO messages (id, conversation_id, role, content) VALUES ($1, $2, 'model', $3)`,
			modelMsgID, conversationID, finalResponse)

		if err != nil {
			slog.Error("Failed to save model message", "error", err)
		} else {
			_, _ = s.DB.Pool.Exec(ctx, `UPDATE conversations SET updated_at = NOW() WHERE id = $1`, conversationID)
		}

		yield(StreamEvent{Type: "done", Payload: "done"}, nil)

		// Generate title async (fire and forget)
		if len(history) <= 2 {
			go s.generateTitle(conversationID, content, finalResponse)
		}

	}, nil
}

func (s *Service) generateTitle(convID uuid.UUID, userMsg, modelMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf("Generate a short, concise title (max 5 words) for this chat conversation:\nUser: %s\nModel: %s", userMsg, modelMsg)

	returnSchema := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"title": {
				Type: genai.TypeString,
			},
		},
		Required: []string{"title"},
	}

	resp, err := s.Client.Models.GenerateContent(ctx, "gemini-2.0-flash", []*genai.Content{
		{Parts: []*genai.Part{{Text: prompt}}},
	}, &genai.GenerateContentConfig{
		ResponseMIMEType: "application/json",
		ResponseSchema:   returnSchema,
	})

	if err == nil && len(resp.Candidates) > 0 {
		var respData struct {
			Title string `json:"title"`
		}

		rawJSON := ""
		for _, p := range resp.Candidates[0].Content.Parts {
			rawJSON += p.Text
		}

		if err := json.Unmarshal([]byte(rawJSON), &respData); err != nil {
			slog.Error("Failed to unmarshal title generation response", "error", err, "raw_json", rawJSON)
			return
		}

		if respData.Title != "" {
			_, err := s.DB.Pool.Exec(ctx, `UPDATE conversations SET title = $2 WHERE id = $1`, convID, respData.Title)
			if err != nil {
				slog.Error("Failed to update conversation title", "error", err)
			}
		}
	}
}
