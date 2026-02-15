package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mikeboe/research-helper/pkg/chat"
)

// MCPSession represents an MCP session
type MCPSession struct {
	ID      string
	Created int64
}

var (
	mcpSessions = make(map[string]*MCPSession)
	sessionMu   sync.RWMutex
)

// MCPRequest represents an MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Handler struct {
	Service *Service
	Chat    *chat.Service
	Tools   *chat.RagToolset
}

func NewHandler(s *Service, c *chat.Service, tools *chat.RagToolset) *Handler {
	return &Handler{Service: s, Chat: c, Tools: tools}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
	r.POST("/mcp", h.MCPHandler)
	api := r.Group("/api")
	{
		api.POST("/research", h.createJob)
		api.GET("/research", h.listJobs)
		api.GET("/research/:id", h.getJob)
		api.GET("/research/:id/logs", h.getJobLogs)

		// Chat Routes
		api.POST("/chat/conversations", h.createConversation)
		api.GET("/chat/conversations", h.listConversations)
		api.GET("/chat/conversations/:id/messages", h.getMessages)
		api.POST("/chat/conversations/:id/messages", h.sendMessage)
	}
}

// MCPHandler handles MCP protocol requests
func (h *Handler) MCPHandler(c *gin.Context) {
	sessionID := c.GetHeader("Mcp-Session-Id")

	var req MCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, MCPResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error: &MCPError{
				Code:    -32700,
				Message: "Parse error",
			},
		})
		return
	}

	// Handle initialize request
	if req.Method == "initialize" {
		if sessionID == "" {
			sessionID = uuid.New().String()
			c.Header("Mcp-Session-Id", sessionID)

			sessionMu.Lock()
			mcpSessions[sessionID] = &MCPSession{
				ID:      sessionID,
				Created: time.Now().Unix(),
			}
			sessionMu.Unlock()
		}

		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"serverInfo": map[string]interface{}{
					"name":    "research-helper-mcp",
					"version": "1.0.0",
				},
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
			},
		})
		return
	}

	// Validate session for other requests
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: "Bad Request: No valid session ID provided",
			},
		})
		return
	}

	sessionMu.RLock()
	_, exists := mcpSessions[sessionID]
	sessionMu.RUnlock()

	if !exists {
		c.JSON(http.StatusBadRequest, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32000,
				Message: "Invalid session ID",
			},
		})
		return
	}

	switch req.Method {
	case "tools/list":
		h.handleToolsList(c, req)
	case "tools/call":
		h.handleToolsCall(c, req)
	case "ping":
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]interface{}{},
		})
	default:
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		})
	}
}

func (h *Handler) handleToolsList(c *gin.Context, req MCPRequest) {
	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "search_content",
					"description": "Search content in the research database using semantic search.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type":        "string",
								"description": "The search query.",
							},
							"topK": map[string]interface{}{
								"type":        "number",
								"description": "The number of top results to return.",
								"default":     5,
							},
							"source": map[string]interface{}{
								"type":        "string",
								"description": "The source to filter results by.",
							},
						},
						"required": []string{"query"},
					},
				},
				{
					"name":        "find_content_by_source",
					"description": "Find all content for a specific source.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"source": map[string]interface{}{
								"type":        "string",
								"description": "The source to find content for.",
							},
						},
						"required": []string{"source"},
					},
				},
				{
					"name":        "find_content_by_metadata",
					"description": "Find content using complex logical filters on metadata.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"filter": map[string]interface{}{
								"type":        "object",
								"description": "JSON filter object with logical operators ($and, $or, $not)",
							},
						},
						"required": []string{"filter"},
					},
				},
			},
		},
	})
}

func (h *Handler) handleToolsCall(c *gin.Context, req MCPRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		c.JSON(http.StatusOK, MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		})
		return
	}

	switch params.Name {
	case "search_content":
		var args chat.SearchContentArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			h.sendError(c, req.ID, -32602, "Invalid arguments")
			return
		}
		resp, err := h.Tools.SearchContent(c.Request.Context(), args)
		if err != nil {
			h.sendError(c, req.ID, -32603, err.Error())
			return
		}
		h.sendResult(c, req.ID, resp)

	case "find_content_by_source":
		var args chat.FindSourceArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			h.sendError(c, req.ID, -32602, "Invalid arguments")
			return
		}
		resp, err := h.Tools.FindContentBySource(c.Request.Context(), args)
		if err != nil {
			h.sendError(c, req.ID, -32603, err.Error())
			return
		}
		h.sendResult(c, req.ID, resp)

	case "find_content_by_metadata":
		var args chat.FindMetadataArgs
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			h.sendError(c, req.ID, -32602, "Invalid arguments")
			return
		}
		resp, err := h.Tools.FindContentByMetadata(c.Request.Context(), args)
		if err != nil {
			h.sendError(c, req.ID, -32603, err.Error())
			return
		}
		h.sendResult(c, req.ID, resp)

	default:
		h.sendError(c, req.ID, -32601, fmt.Sprintf("Tool not found: %s", params.Name))
	}
}

func (h *Handler) sendError(c *gin.Context, id interface{}, code int, msg string) {
	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: msg,
		},
	})
}

func (h *Handler) sendResult(c *gin.Context, id interface{}, result interface{}) {
	var textContent string
	switch v := result.(type) {
	case chat.SearchContentResp:
		textContent = v.Results
	case chat.FindSourceResp:
		textContent = v.Content
	case chat.FindMetadataResp:
		textContent = v.Content
	default:
		textContent = fmt.Sprintf("%v", result)
	}

	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": textContent,
				},
			},
		},
	})
}

func (h *Handler) createConversation(c *gin.Context) {
	conv, err := h.Chat.CreateConversation(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conv)
}

func (h *Handler) listConversations(c *gin.Context) {
	convs, err := h.Chat.ListConversations(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if convs == nil {
		convs = []chat.Conversation{}
	}
	c.JSON(http.StatusOK, convs)
}

func (h *Handler) getMessages(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}

	msgs, err := h.Chat.GetHistory(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if msgs == nil {
		msgs = []chat.Message{}
	}
	c.JSON(http.StatusOK, msgs)
}

func (h *Handler) sendMessage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	next, err := h.Chat.SendMessage(c.Request.Context(), id, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	for event, err := range next {
		if err != nil {
			// If we encounter an error during the stream, we try to send it as an event
			errEvent := chat.StreamEvent{
				Type:    "error",
				Payload: err.Error(),
			}
			if data, err := json.Marshal(errEvent); err == nil {
				_, _ = c.Writer.Write([]byte("data: "))
				_, _ = c.Writer.Write(data)
				_, _ = c.Writer.Write([]byte("\n\n"))
				c.Writer.Flush()
			}
			return
		}

		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(data)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	}
}

func (h *Handler) createJob(c *gin.Context) {
	var req CreateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := h.Service.CreateJob(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

func (h *Handler) listJobs(c *gin.Context) {
	jobs, err := h.Service.ListJobs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Return empty list instead of null
	if jobs == nil {
		jobs = []Job{}
	}
	c.JSON(http.StatusOK, jobs)
}

func (h *Handler) getJob(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}

	job, err := h.Service.GetJob(c.Request.Context(), id)
	if err != nil {
		// Differentiate 404 vs 500 later if needed
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, job)
}

func (h *Handler) getJobLogs(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid uuid"})
		return
	}

	logs, err := h.Service.GetJobLogs(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if logs == nil {
		logs = []LogEntry{}
	}
	c.JSON(http.StatusOK, logs)
}
