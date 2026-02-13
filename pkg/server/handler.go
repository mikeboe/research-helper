package server

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mikeboe/research-helper/pkg/chat"
)

type Handler struct {
	Service *Service
	Chat    *chat.Service
}

func NewHandler(s *Service, c *chat.Service) *Handler {
	return &Handler{Service: s, Chat: c}
}

func (h *Handler) RegisterRoutes(r *gin.Engine) {
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

	c.Header("Content-Type", "application/x-ndjson")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	enc := json.NewEncoder(c.Writer)
	for event, err := range next {
		if err != nil {
			// If we encounter an error during the stream, we try to send it as an event
			_ = enc.Encode(chat.StreamEvent{
				Type:    "error",
				Payload: err.Error(),
			})
			return
		}
		if err := enc.Encode(event); err != nil {
			return
		}
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
