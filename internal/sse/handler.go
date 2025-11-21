package sse

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/meet-app/backend/internal/api/middleware"
)

// Handler handles SSE connections
type Handler struct {
	hub *Hub
}

// NewHandler creates a new SSE handler
func NewHandler() *Handler {
	return &Handler{
		hub: GetHub(),
	}
}

// Stream handles SSE connection for a meeting
func (h *Handler) Stream(c *gin.Context) {
	// Get user ID from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Get meeting ID from URL
	meetingIDStr := c.Param("id")
	meetingID, err := uuid.Parse(meetingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid meeting ID"})
		return
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	// Create client
	client := &Client{
		ID:        uuid.New(),
		UserID:    userID,
		MeetingID: meetingID,
		Send:      make(chan []byte, 256),
	}

	// Register client
	h.hub.Register(client)

	// Ensure headers are sent
	c.Writer.WriteHeader(http.StatusOK)

	// Send initial connection event
	fmt.Fprintf(c.Writer, "event: connected\ndata: {\"message\": \"Connected to meeting events\"}\n\n")
	c.Writer.Flush()

	// Handle client disconnect
	notify := c.Request.Context().Done()

	// Cleanup on disconnect
	defer h.hub.Unregister(client)

	// Stream events to client
	for {
		select {
		case message, ok := <-client.Send:
			if !ok {
				return
			}
			// Parse the event to get the type for SSE event name
			var event Event
			if err := json.Unmarshal(message, &event); err == nil {
				// Send with event type as SSE event name
				fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, string(message))
			} else {
				// Fallback to just data
				fmt.Fprintf(c.Writer, "data: %s\n\n", message)
			}
			c.Writer.Flush()
		case <-notify:
			return
		}
	}
}
