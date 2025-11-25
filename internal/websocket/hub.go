package websocket

import (
	"log"
	"sync"

	"github.com/google/uuid"
)

// Client represents a WebSocket client
type Client struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Username  string
	MeetingID uuid.UUID
	Send      chan []byte
	Hub       *Hub
}

// Hub maintains the set of active WebSocket clients
type Hub struct {
	// Clients registered per meeting (approved and in WebRTC)
	clients map[uuid.UUID]map[uuid.UUID]*Client

	// Pending clients waiting for approval (connected but not in WebRTC)
	pendingClients map[uuid.UUID]map[uuid.UUID]*Client

	// Pending join requests per meeting
	pendingJoinRequests map[uuid.UUID]map[uuid.UUID]*JoinRequestInfo

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast messages to specific client
	broadcast chan *Message

	mu sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:             make(map[uuid.UUID]map[uuid.UUID]*Client),
		pendingClients:      make(map[uuid.UUID]map[uuid.UUID]*Client),
		pendingJoinRequests: make(map[uuid.UUID]map[uuid.UUID]*JoinRequestInfo),
		register:            make(chan *Client),
		unregister:          make(chan *Client),
		broadcast:           make(chan *Message, 256),
	}
}

// Run starts the hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// registerClient registers a new client
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.MeetingID] == nil {
		h.clients[client.MeetingID] = make(map[uuid.UUID]*Client)
	}
	h.clients[client.MeetingID][client.UserID] = client

	log.Printf("WebSocket: Client registered - UserID: %s, MeetingID: %s, Total: %d",
		client.UserID, client.MeetingID, len(h.clients[client.MeetingID]))

	// Notify other peers about the new peer
	h.notifyPeerJoined(client)
}

// unregisterClient unregisters a client
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.MeetingID]; ok {
		if _, ok := clients[client.UserID]; ok {
			delete(clients, client.UserID)
			close(client.Send)

			log.Printf("WebSocket: Client unregistered - UserID: %s, MeetingID: %s",
				client.UserID, client.MeetingID)

			// Clean up empty meetings
			if len(clients) == 0 {
				delete(h.clients, client.MeetingID)
			}

			// Notify other peers about the peer leaving
			h.notifyPeerLeft(client)
		}
	}
}

// broadcastMessage sends a message to a specific client or meeting
func (h *Hub) broadcastMessage(message *Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Get both registered and pending clients
	registeredClients, hasRegistered := h.clients[message.MeetingID]
	pendingClientsMap, hasPending := h.pendingClients[message.MeetingID]

	if !hasRegistered && !hasPending {
		log.Printf("WebSocket: No clients in meeting %s", message.MeetingID)
		return
	}

	// If message has a specific recipient, send only to them
	if message.To != uuid.Nil {
		// Check in registered clients
		if hasRegistered {
			if client, ok := registeredClients[message.To]; ok {
				select {
				case client.Send <- mustMarshal(message):
					log.Printf("WebSocket: Sent %s from %s to %s (registered)", message.Type, message.From, message.To)
				default:
					log.Printf("WebSocket: Client %s buffer full", message.To)
				}
				return
			}
		}

		// Check in pending clients
		if hasPending {
			if client, ok := pendingClientsMap[message.To]; ok {
				select {
				case client.Send <- mustMarshal(message):
					log.Printf("WebSocket: Sent %s from %s to %s (pending)", message.Type, message.From, message.To)
				default:
					log.Printf("WebSocket: Client %s buffer full", message.To)
				}
				return
			}
		}

		log.Printf("WebSocket: Recipient %s not found in meeting", message.To)
		return
	}

	// Otherwise, broadcast to all registered clients in the meeting except sender
	sentCount := 0
	if hasRegistered {
		for userID, client := range registeredClients {
			if userID == message.From {
				continue
			}
			select {
			case client.Send <- mustMarshal(message):
				sentCount++
			default:
				log.Printf("WebSocket: Client %s buffer full", userID)
			}
		}
	}

	log.Printf("WebSocket: Broadcast %s from %s to %d clients", message.Type, message.From, sentCount)
}

// notifyPeerJoined notifies all peers in a meeting about a new peer
func (h *Hub) notifyPeerJoined(newClient *Client) {
	clients, ok := h.clients[newClient.MeetingID]
	if !ok {
		return
	}

	peerInfo := PeerInfo{
		UserID:   newClient.UserID,
		Username: newClient.Username,
	}

	message := &Message{
		Type:      MessageTypePeerJoined,
		From:      newClient.UserID,
		MeetingID: newClient.MeetingID,
		Data:      peerInfo,
	}

	// Send to all other clients
	for userID, client := range clients {
		if userID == newClient.UserID {
			continue
		}
		select {
		case client.Send <- mustMarshal(message):
		default:
			log.Printf("WebSocket: Failed to notify %s about new peer", userID)
		}
	}

	// Send list of existing peers to the new client
	existingPeers := make([]PeerInfo, 0)
	for userID, client := range clients {
		if userID != newClient.UserID {
			existingPeers = append(existingPeers, PeerInfo{
				UserID:   client.UserID,
				Username: client.Username,
			})
		}
	}

	if len(existingPeers) > 0 {
		readyMessage := &Message{
			Type:      MessageTypeReady,
			MeetingID: newClient.MeetingID,
			Data:      existingPeers,
		}
		select {
		case newClient.Send <- mustMarshal(readyMessage):
		default:
			log.Printf("WebSocket: Failed to send peer list to new client")
		}
	}
}

// notifyPeerLeft notifies all peers in a meeting about a peer leaving
func (h *Hub) notifyPeerLeft(leftClient *Client) {
	clients, ok := h.clients[leftClient.MeetingID]
	if !ok {
		return
	}

	message := &Message{
		Type:      MessageTypePeerLeft,
		From:      leftClient.UserID,
		MeetingID: leftClient.MeetingID,
		Data: PeerInfo{
			UserID:   leftClient.UserID,
			Username: leftClient.Username,
		},
	}

	for userID, client := range clients {
		if userID == leftClient.UserID {
			continue
		}
		select {
		case client.Send <- mustMarshal(message):
		default:
			log.Printf("WebSocket: Failed to notify %s about peer leaving", userID)
		}
	}
}

// GetClientsInMeeting returns the number of clients in a meeting
func (h *Hub) GetClientsInMeeting(meetingID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[meetingID]; ok {
		return len(clients)
	}
	return 0
}

// SendMessage sends a message through the hub
func (h *Hub) SendMessage(message *Message) {
	h.broadcast <- message
}

// AddPendingJoinRequest adds a pending join request
func (h *Hub) AddPendingJoinRequest(meetingID uuid.UUID, request *JoinRequestInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pendingJoinRequests[meetingID] == nil {
		h.pendingJoinRequests[meetingID] = make(map[uuid.UUID]*JoinRequestInfo)
	}
	h.pendingJoinRequests[meetingID][request.UserID] = request

	log.Printf("WebSocket: Pending join request added - UserID: %s, MeetingID: %s", request.UserID, meetingID)
}

// RemovePendingJoinRequest removes a pending join request
func (h *Hub) RemovePendingJoinRequest(meetingID uuid.UUID, userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if requests, ok := h.pendingJoinRequests[meetingID]; ok {
		delete(requests, userID)
		if len(requests) == 0 {
			delete(h.pendingJoinRequests, meetingID)
		}
		log.Printf("WebSocket: Pending join request removed - UserID: %s, MeetingID: %s", userID, meetingID)
	}
}

// GetPendingJoinRequest gets a pending join request
func (h *Hub) GetPendingJoinRequest(meetingID uuid.UUID, userID uuid.UUID) (*JoinRequestInfo, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if requests, ok := h.pendingJoinRequests[meetingID]; ok {
		if request, ok := requests[userID]; ok {
			return request, true
		}
	}
	return nil, false
}

// GetHostClient returns the host client for a meeting
func (h *Hub) GetHostClient(meetingID uuid.UUID, hostUserID uuid.UUID) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Check in registered clients first
	if clients, ok := h.clients[meetingID]; ok {
		if hostClient, ok := clients[hostUserID]; ok {
			return hostClient
		}
	}

	// Check in pending clients
	if clients, ok := h.pendingClients[meetingID]; ok {
		if hostClient, ok := clients[hostUserID]; ok {
			return hostClient
		}
	}

	return nil
}

// AddPendingClient adds a client to pending clients (not yet approved for WebRTC)
func (h *Hub) AddPendingClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pendingClients[client.MeetingID] == nil {
		h.pendingClients[client.MeetingID] = make(map[uuid.UUID]*Client)
	}
	h.pendingClients[client.MeetingID][client.UserID] = client

	log.Printf("WebSocket: Pending client added - UserID: %s, MeetingID: %s", client.UserID, client.MeetingID)
}

// RemovePendingClient removes a client from pending clients
func (h *Hub) RemovePendingClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.pendingClients[client.MeetingID]; ok {
		if _, ok := clients[client.UserID]; ok {
			delete(clients, client.UserID)

			// Clean up empty meetings
			if len(clients) == 0 {
				delete(h.pendingClients, client.MeetingID)
			}

			log.Printf("WebSocket: Pending client removed - UserID: %s, MeetingID: %s", client.UserID, client.MeetingID)
		}
	}
}

// ApproveClient moves a client from pending to registered (approved for WebRTC)
func (h *Hub) ApproveClient(meetingID uuid.UUID, userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Get client from pending
	if clients, ok := h.pendingClients[meetingID]; ok {
		if client, ok := clients[userID]; ok {
			// Remove from pending
			delete(clients, userID)
			if len(clients) == 0 {
				delete(h.pendingClients, meetingID)
			}

			// Add to registered clients
			if h.clients[meetingID] == nil {
				h.clients[meetingID] = make(map[uuid.UUID]*Client)
			}
			h.clients[meetingID][userID] = client

			log.Printf("WebSocket: Client approved and registered - UserID: %s, MeetingID: %s", userID, meetingID)

			// Notify other peers about the new peer
			h.notifyPeerJoined(client)
		}
	}
}

// GetPendingClient gets a pending client
func (h *Hub) GetPendingClient(meetingID uuid.UUID, userID uuid.UUID) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.pendingClients[meetingID]; ok {
		if client, ok := clients[userID]; ok {
			return client
		}
	}
	return nil
}

// GetClient gets a registered client
func (h *Hub) GetClient(meetingID uuid.UUID, userID uuid.UUID) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[meetingID]; ok {
		if client, ok := clients[userID]; ok {
			return client
		}
	}
	return nil
}

// Global hub instance
var globalHub *Hub
var once sync.Once

// GetHub returns the global hub instance
func GetHub() *Hub {
	once.Do(func() {
		globalHub = NewHub()
		go globalHub.Run()
	})
	return globalHub
}
