package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/meet-app/backend/internal/api/middleware"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 8192
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// TODO: Restrict in production
		return true
	},
}

// Handler handles WebSocket connections
type Handler struct {
	hub *Hub
}

// NewHandler creates a new WebSocket handler
func NewHandler() *Handler {
	return &Handler{
		hub: GetHub(),
	}
}

// HandleWebSocket handles WebSocket upgrade and communication
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// Get user info from context (set by auth middleware)
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	username, err := middleware.GetUsernameFromContext(c)
	if err != nil {
		username = "Unknown"
	}

	// Get meeting ID from query parameter
	meetingIDStr := c.Query("meeting_id")
	if meetingIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "meeting_id is required"})
		return
	}

	meetingID, err := uuid.Parse(meetingIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid meeting_id"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket: Failed to upgrade connection: %v", err)
		return
	}

	// Create client
	client := &Client{
		ID:        uuid.New(),
		UserID:    userID,
		Username:  username,
		MeetingID: meetingID,
		Send:      make(chan []byte, 256),
		Hub:       h.hub,
	}

	// Register client to hub
	h.hub.register <- client

	// Start goroutines for reading and writing
	go h.writePump(client, conn)
	go h.readPump(client, conn)
}

// readPump pumps messages from the WebSocket connection to the hub
func (h *Handler) readPump(client *Client, conn *websocket.Conn) {
	defer func() {
		h.hub.unregister <- client
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	conn.SetReadLimit(maxMessageSize)

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket: Unexpected close error: %v", err)
			}
			break
		}

		// Parse message
		var msg Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("WebSocket: Failed to parse message: %v", err)
			continue
		}

		// Set sender info
		msg.From = client.UserID
		msg.MeetingID = client.MeetingID

		// Handle message based on type
		h.handleMessage(client, &msg)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (h *Handler) writePump(client *Client, conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current WebSocket message
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles incoming WebSocket messages
func (h *Handler) handleMessage(client *Client, msg *Message) {
	switch msg.Type {
	case MessageTypeOffer, MessageTypeAnswer, MessageTypeICECandidate:
		// Forward signaling messages to the recipient
		if msg.To == uuid.Nil {
			log.Printf("WebSocket: Message %s missing recipient", msg.Type)
			h.sendError(client, "Recipient is required for signaling messages")
			return
		}
		h.hub.SendMessage(msg)

	case MessageTypeMediaStateChanged:
		// Broadcast media state changes to all other participants
		log.Printf("WebSocket: User %s changed media state in meeting %s", client.UserID, client.MeetingID)
		h.hub.SendMessage(msg)

	case MessageTypeJoinRequest:
		// Handle join request from user
		h.handleJoinRequest(client, msg)

	case MessageTypeApproveJoinRequest:
		// Handle approval from host
		h.handleApproveJoinRequest(client, msg)

	case MessageTypeRejectJoinRequest:
		// Handle rejection from host
		h.handleRejectJoinRequest(client, msg)

	case MessageTypeJoin:
		// Already handled by registration
		log.Printf("WebSocket: User %s joined meeting %s", client.UserID, client.MeetingID)

	case MessageTypeLeave:
		// Unregister client
		h.hub.unregister <- client

	default:
		log.Printf("WebSocket: Unknown message type: %s", msg.Type)
		h.sendError(client, "Unknown message type")
	}
}

// handleJoinRequest handles join request from a user
func (h *Handler) handleJoinRequest(client *Client, msg *Message) {
	// Parse join request data
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		log.Printf("WebSocket: Invalid join request data from %s", client.UserID)
		h.sendError(client, "Invalid join request data")
		return
	}

	hostUserIDStr, ok := data["host_user_id"].(string)
	if !ok {
		log.Printf("WebSocket: Missing host_user_id in join request")
		h.sendError(client, "Missing host user ID")
		return
	}

	hostUserID, err := uuid.Parse(hostUserIDStr)
	if err != nil {
		log.Printf("WebSocket: Invalid host_user_id: %s", hostUserIDStr)
		h.sendError(client, "Invalid host user ID")
		return
	}

	email, _ := data["email"].(string)

	// Create join request info
	joinRequest := &JoinRequestInfo{
		UserID:    client.UserID,
		Username:  client.Username,
		Email:     email,
		Timestamp: time.Now().Unix(),
	}

	// Store pending join request
	h.hub.AddPendingJoinRequest(client.MeetingID, joinRequest)

	// Send pending status to requesting user
	pendingMsg := &Message{
		Type:      MessageTypeJoinRequestPending,
		MeetingID: client.MeetingID,
		Data: map[string]interface{}{
			"message": "Waiting for host approval",
		},
	}
	select {
	case client.Send <- mustMarshal(pendingMsg):
		log.Printf("WebSocket: Sent pending status to %s", client.UserID)
	default:
		log.Printf("WebSocket: Failed to send pending status to %s", client.UserID)
	}

	// Notify host about pending join request
	hostClient := h.hub.GetHostClient(client.MeetingID, hostUserID)
	if hostClient != nil {
		notifyMsg := &Message{
			Type:      MessageTypePendingJoinRequest,
			From:      client.UserID,
			MeetingID: client.MeetingID,
			Data:      joinRequest,
		}
		select {
		case hostClient.Send <- mustMarshal(notifyMsg):
			log.Printf("WebSocket: Notified host %s about join request from %s", hostUserID, client.UserID)
		default:
			log.Printf("WebSocket: Failed to notify host about join request")
		}
	} else {
		log.Printf("WebSocket: Host %s not found in meeting %s", hostUserID, client.MeetingID)
		h.sendError(client, "Host is not available")
	}
}

// handleApproveJoinRequest handles approval from host
func (h *Handler) handleApproveJoinRequest(client *Client, msg *Message) {
	// Parse approval data
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		log.Printf("WebSocket: Invalid approval data")
		h.sendError(client, "Invalid approval data")
		return
	}

	requestUserIDStr, ok := data["user_id"].(string)
	if !ok {
		log.Printf("WebSocket: Missing user_id in approval")
		h.sendError(client, "Missing user ID")
		return
	}

	requestUserID, err := uuid.Parse(requestUserIDStr)
	if err != nil {
		log.Printf("WebSocket: Invalid user_id: %s", requestUserIDStr)
		h.sendError(client, "Invalid user ID")
		return
	}

	// Get pending join request
	joinRequest, exists := h.hub.GetPendingJoinRequest(client.MeetingID, requestUserID)
	if !exists {
		log.Printf("WebSocket: No pending join request for user %s", requestUserID)
		h.sendError(client, "Join request not found")
		return
	}

	// Remove from pending requests
	h.hub.RemovePendingJoinRequest(client.MeetingID, requestUserID)

	// Send approval to requesting user
	approvalMsg := &Message{
		Type:      MessageTypeJoinApproved,
		To:        requestUserID,
		MeetingID: client.MeetingID,
		Data: map[string]interface{}{
			"message": "Your join request has been approved",
		},
	}
	h.hub.SendMessage(approvalMsg)

	log.Printf("WebSocket: Host %s approved join request from %s (%s)", client.UserID, joinRequest.Username, requestUserID)
}

// handleRejectJoinRequest handles rejection from host
func (h *Handler) handleRejectJoinRequest(client *Client, msg *Message) {
	// Parse rejection data
	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		log.Printf("WebSocket: Invalid rejection data")
		h.sendError(client, "Invalid rejection data")
		return
	}

	requestUserIDStr, ok := data["user_id"].(string)
	if !ok {
		log.Printf("WebSocket: Missing user_id in rejection")
		h.sendError(client, "Missing user ID")
		return
	}

	requestUserID, err := uuid.Parse(requestUserIDStr)
	if err != nil {
		log.Printf("WebSocket: Invalid user_id: %s", requestUserIDStr)
		h.sendError(client, "Invalid user ID")
		return
	}

	// Get pending join request
	joinRequest, exists := h.hub.GetPendingJoinRequest(client.MeetingID, requestUserID)
	if !exists {
		log.Printf("WebSocket: No pending join request for user %s", requestUserID)
		h.sendError(client, "Join request not found")
		return
	}

	// Remove from pending requests
	h.hub.RemovePendingJoinRequest(client.MeetingID, requestUserID)

	// Send rejection to requesting user
	rejectionMsg := &Message{
		Type:      MessageTypeJoinRejected,
		To:        requestUserID,
		MeetingID: client.MeetingID,
		Data: map[string]interface{}{
			"message": "Your join request has been rejected",
		},
	}
	h.hub.SendMessage(rejectionMsg)

	log.Printf("WebSocket: Host %s rejected join request from %s (%s)", client.UserID, joinRequest.Username, requestUserID)
}

// sendError sends an error message to a client
func (h *Handler) sendError(client *Client, message string) {
	errorMsg := &Message{
		Type: MessageTypeError,
		Data: ErrorMessage{Message: message},
	}
	select {
	case client.Send <- mustMarshal(errorMsg):
	default:
		log.Printf("WebSocket: Failed to send error to client %s", client.UserID)
	}
}
