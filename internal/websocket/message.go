package websocket

import "github.com/google/uuid"

// MessageType represents the type of WebSocket message
type MessageType string

const (
	// Signaling messages
	MessageTypeOffer       MessageType = "offer"
	MessageTypeAnswer      MessageType = "answer"
	MessageTypeICECandidate MessageType = "ice-candidate"

	// Peer management
	MessageTypeJoin        MessageType = "join"
	MessageTypeLeave       MessageType = "leave"
	MessageTypePeerJoined  MessageType = "peer-joined"
	MessageTypePeerLeft    MessageType = "peer-left"

	// Media state
	MessageTypeMediaStateChanged MessageType = "media-state-changed"

	// Connection status
	MessageTypeReady       MessageType = "ready"
	MessageTypeError       MessageType = "error"
)

// Message represents a WebSocket message
type Message struct {
	Type      MessageType `json:"type"`
	From      uuid.UUID   `json:"from,omitempty"`
	To        uuid.UUID   `json:"to,omitempty"`
	MeetingID uuid.UUID   `json:"meeting_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

// SDPMessage represents SDP offer/answer
type SDPMessage struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"` // "offer" or "answer"
}

// ICECandidateMessage represents an ICE candidate
type ICECandidateMessage struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

// PeerInfo represents information about a peer
type PeerInfo struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}

// ErrorMessage represents an error message
type ErrorMessage struct {
	Message string `json:"message"`
}
