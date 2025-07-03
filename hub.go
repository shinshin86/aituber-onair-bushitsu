package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// RoomInfo represents information about a chat room
type RoomInfo struct {
	Name      string `json:"name"`
	UserCount int    `json:"userCount"`
}

type Hub struct {
	mu               sync.RWMutex
	clients          map[*Client]bool
	rooms            map[string]map[*Client]bool
	predefinedRooms  map[string]bool
	allowDynamicRooms bool
	broadcast        chan WebSocketMessage
	register         chan *Client
	unregister       chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:          make(map[*Client]bool),
		rooms:            make(map[string]map[*Client]bool),
		predefinedRooms:  make(map[string]bool),
		allowDynamicRooms: false,
		broadcast:        make(chan WebSocketMessage, 1024),
		register:         make(chan *Client),
		unregister:       make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			
			if _, ok := h.rooms[client.room]; !ok {
				h.rooms[client.room] = make(map[*Client]bool)
			}
			h.rooms[client.room][client] = true
			h.mu.Unlock()
			
			log.Printf("[INFO] Client connected: name=%s, room=%s", client.name, client.room)
			
			// Send join notification to the room
			joinMsg := WebSocketMessage{
				Type:      "user_event",
				Room:      client.room,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Data: UserEventData{
					Event: "join",
					User:  client.name,
				},
			}
			h.broadcast <- joinMsg

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.close()
				
				var shouldSendLeaveMsg bool
				var leaveMsg WebSocketMessage
				
				if room, ok := h.rooms[client.room]; ok {
					delete(room, client)
					if len(room) == 0 {
						delete(h.rooms, client.room)
					} else {
						// Prepare leave notification
						shouldSendLeaveMsg = true
						leaveMsg = WebSocketMessage{
							Type:      "user_event",
							Room:      client.room,
							Timestamp: time.Now().UTC().Format(time.RFC3339),
							Data: UserEventData{
								Event: "leave",
								User:  client.name,
							},
						}
					}
				}
				
				log.Printf("[INFO] Client disconnected: name=%s, room=%s", client.name, client.room)
				h.mu.Unlock()
				
				// Send leave message after releasing the lock
				if shouldSendLeaveMsg {
					h.broadcast <- leaveMsg
				}
			} else {
				h.mu.Unlock()
			}

		case message := <-h.broadcast:
			h.route(message)
		}
	}
}

func (h *Hub) route(msg WebSocketMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ERROR] Failed to marshal message: %v", err)
		return
	}

	// Handle chat messages with mentions
	if msg.Type == "chat" {
		if chatData, ok := msg.Data.(ChatData); ok {
			if strings.HasPrefix(chatData.Text, "@") {
				parts := strings.SplitN(chatData.Text, " ", 2)
				if len(parts) > 0 {
					targetName := strings.TrimPrefix(parts[0], "@")
					h.sendToUser(targetName, data)
					// Also send to the sender
					h.sendToUser(chatData.From, data)
					return
				}
			}
		}
	}

	h.sendToRoom(msg.Room, data)
}

func (h *Hub) sendToRoom(room string, data []byte) {
	h.mu.RLock()
	clients, ok := h.rooms[room]
	if !ok {
		h.mu.RUnlock()
		return
	}
	
	// Copy client list to avoid holding lock during send
	clientList := make([]*Client, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	h.mu.RUnlock()
	
	// Track send statistics
	sentCount := 0
	timeoutCount := 0

	for _, client := range clientList {
		select {
		case client.send <- data:
			// Message sent successfully
			sentCount++
		case <-time.After(5 * time.Second):
			// Timeout - client is not responsive
			timeoutCount++
			log.Printf("[WARN] Message send timeout for client %s in room %s, removing client", client.name, client.room)
			h.removeClient(client)
		}
	}
	
	if timeoutCount > 0 {
		log.Printf("[INFO] Room %s: sent to %d/%d clients (%d timeouts)", room, sentCount, len(clientList), timeoutCount)
	}
}

func (h *Hub) sendToUser(name string, data []byte) {
	h.mu.RLock()
	// Find all clients with this name
	targetClients := make([]*Client, 0)
	for client := range h.clients {
		if client.name == name {
			targetClients = append(targetClients, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range targetClients {
		select {
		case client.send <- data:
			// Message sent successfully
		case <-time.After(5 * time.Second):
			// Timeout - client is not responsive
			log.Printf("[WARN] Message send timeout for client %s (direct message), removing client", client.name)
			h.removeClient(client)
		}
	}
}

// removeClient safely removes a client from all maps
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		client.close()
		
		if room, ok := h.rooms[client.room]; ok {
			delete(room, client)
			if len(room) == 0 {
				delete(h.rooms, client.room)
			}
		}
	}
}

// CreateRoom creates a new predefined room
func (h *Hub) CreateRoom(name string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if _, exists := h.predefinedRooms[name]; exists {
		return fmt.Errorf("room already exists: %s", name)
	}
	
	h.predefinedRooms[name] = true
	log.Printf("[INFO] Room created: %s", name)
	return nil
}

// GetRooms returns a list of all rooms with their user counts
func (h *Hub) GetRooms() []RoomInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	var rooms []RoomInfo
	
	// Add predefined rooms
	for roomName := range h.predefinedRooms {
		info := RoomInfo{
			Name:      roomName,
			UserCount: 0,
		}
		if activeRoom, exists := h.rooms[roomName]; exists {
			info.UserCount = len(activeRoom)
		}
		rooms = append(rooms, info)
	}
	
	// If dynamic rooms are allowed, add active rooms not in predefined list
	if h.allowDynamicRooms {
		for roomName, clients := range h.rooms {
			if _, isPredefined := h.predefinedRooms[roomName]; !isPredefined {
				rooms = append(rooms, RoomInfo{
					Name:      roomName,
					UserCount: len(clients),
				})
			}
		}
	}
	
	return rooms
}

// IsRoomAllowed checks if a room can be joined
func (h *Hub) IsRoomAllowed(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// If dynamic rooms are allowed, any room name is valid
	if h.allowDynamicRooms {
		return true
	}
	
	// Otherwise, check if it's a predefined room
	_, exists := h.predefinedRooms[name]
	return exists
}

// SetAllowDynamicRooms updates the dynamic room creation setting
func (h *Hub) SetAllowDynamicRooms(allow bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowDynamicRooms = allow
}

// shutdown gracefully shuts down the hub
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	log.Println("[INFO] Shutting down hub...")
	
	// Close all client connections
	for client := range h.clients {
		client.close()
	}
	
	// Clear maps
	h.clients = make(map[*Client]bool)
	h.rooms = make(map[string]map[*Client]bool)
	
	log.Println("[INFO] Hub shutdown complete")
}