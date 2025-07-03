package main

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait        = 10 * time.Second
	pongWait         = 60 * time.Second
	pingPeriod       = 30 * time.Second
	maxMessageSize   = 512 * 1024
	maxMessageLength = 4096 // Maximum text message length
)

// WebSocketMessage represents all messages sent between server and client
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Room      string      `json:"room"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// ChatData represents chat message data
type ChatData struct {
	From    string   `json:"from"`
	FromId  string   `json:"fromId"`
	Text    string   `json:"text"`
	Mention []string `json:"mention,omitempty"`
}

// UserEventData represents user join/leave events
type UserEventData struct {
	Event string `json:"event"` // "join" or "leave"
	User  string `json:"user"`
}

// SystemEventData represents system events
type SystemEventData struct {
	Event   string                 `json:"event"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// ClientMessage represents messages sent from client
type ClientMessage struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type Client struct {
	id        string
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	room      string
	name      string
	closeOnce sync.Once
}

func (c *Client) readLoop() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[ERROR] websocket error: %v", err)
			}
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("[ERROR] json unmarshal error: %v", err)
			continue
		}
		
		// Validate message
		if err := validateMessage(clientMsg); err != nil {
			log.Printf("[ERROR] invalid message from %s: %v", c.name, err)
			continue
		}

		// Convert client message to server message format
		if clientMsg.Type == "chat" {
			// Parse mentions
			var mentions []string
			if strings.HasPrefix(clientMsg.Text, "@") {
				parts := strings.SplitN(clientMsg.Text, " ", 2)
				if len(parts) > 0 {
					mentions = append(mentions, strings.TrimPrefix(parts[0], "@"))
				}
			}

			wsMsg := WebSocketMessage{
				Type:      "chat",
				Room:      c.room,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Data: ChatData{
					From:    c.name,
					FromId:  c.id,
					Text:    clientMsg.Text,
					Mention: mentions,
				},
			}
			
			c.hub.broadcast <- wsMsg
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Printf("[INFO] Send channel closed for client %s", c.name)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[ERROR] Failed to write message to client %s: %v", c.name, err)
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[ERROR] Failed to write ping to client %s: %v", c.name, err)
				return
			}
		}
	}
}

// close safely closes the client connection and channels
func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.send)
		c.conn.Close()
	})
}

// validateMessage validates incoming client messages
func validateMessage(msg ClientMessage) error {
	// Check message type
	if msg.Type != "chat" {
		return errors.New("invalid message type")
	}
	
	// Check text length
	if len(msg.Text) == 0 {
		return errors.New("empty message")
	}
	
	if len(msg.Text) > maxMessageLength {
		return errors.New("message too long")
	}
	
	// Check for control characters
	for _, r := range msg.Text {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return errors.New("invalid characters in message")
		}
	}
	
	return nil
}