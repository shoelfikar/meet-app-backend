package sse

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/google/uuid"
)

// EventType represents the type of SSE event
type EventType string

const (
	EventParticipantJoined  EventType = "participant_joined"
	EventParticipantLeft    EventType = "participant_left"
	EventParticipantUpdated EventType = "participant_updated"
	EventChatMessage        EventType = "chat_message"
	EventMeetingEnded       EventType = "meeting_ended"
	EventRecordingStarted   EventType = "recording_started"
	EventRecordingStopped   EventType = "recording_stopped"
	EventScreenShareStarted EventType = "screen_share_started"
	EventScreenShareStopped EventType = "screen_share_stopped"
)

// Event represents an SSE event
type Event struct {
	Type EventType   `json:"type"`
	Data interface{} `json:"data"`
}

// Client represents a connected SSE client
type Client struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	MeetingID uuid.UUID
	Send      chan []byte
}

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Clients registered per meeting
	clients map[uuid.UUID]map[uuid.UUID]*Client
	mu      sync.RWMutex
}

// NewHub creates a new SSE hub
func NewHub() *Hub {
	return &Hub{
		clients: make(map[uuid.UUID]map[uuid.UUID]*Client),
	}
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.MeetingID] == nil {
		h.clients[client.MeetingID] = make(map[uuid.UUID]*Client)
	}
	h.clients[client.MeetingID][client.ID] = client
	log.Printf("SSE: Client registered - UserID: %s, MeetingID: %s, Total clients in meeting: %d",
		client.UserID, client.MeetingID, len(h.clients[client.MeetingID]))
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.MeetingID]; ok {
		if _, ok := clients[client.ID]; ok {
			delete(clients, client.ID)
			close(client.Send)
		}
		// Clean up empty meeting rooms
		if len(clients) == 0 {
			delete(h.clients, client.MeetingID)
		}
	}
}

// BroadcastToMeeting sends an event to all clients in a meeting
func (h *Hub) BroadcastToMeeting(meetingID uuid.UUID, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[meetingID]
	if !ok {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	for _, client := range clients {
		select {
		case client.Send <- data:
		default:
			// Client buffer full, skip
		}
	}
}

// BroadcastToMeetingExcept sends an event to all clients in a meeting except one
func (h *Hub) BroadcastToMeetingExcept(meetingID uuid.UUID, excludeUserID uuid.UUID, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[meetingID]
	if !ok {
		log.Printf("SSE: No clients found for meeting %s", meetingID)
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("SSE: Failed to marshal event: %v", err)
		return
	}

	sentCount := 0
	for _, client := range clients {
		if client.UserID == excludeUserID {
			continue
		}
		select {
		case client.Send <- data:
			sentCount++
		default:
			log.Printf("SSE: Client buffer full, skipping UserID: %s", client.UserID)
		}
	}
	log.Printf("SSE: Broadcast %s to %d clients in meeting %s (excluded: %s)",
		event.Type, sentCount, meetingID, excludeUserID)
}

// GetClientCount returns the number of connected clients for a meeting
func (h *Hub) GetClientCount(meetingID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[meetingID]; ok {
		return len(clients)
	}
	return 0
}

// Global hub instance
var globalHub *Hub
var once sync.Once

// GetHub returns the global hub instance
func GetHub() *Hub {
	once.Do(func() {
		globalHub = NewHub()
	})
	return globalHub
}
