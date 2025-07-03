package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var addr = flag.String("addr", ":8080", "http service address")
var allowDynamicRooms = flag.Bool("allow-dynamic-rooms", false, "allow dynamic room creation")
var authUser = flag.String("auth-user", "", "basic auth username for web UI (requires auth-password)")
var authPassword = flag.String("auth-password", "", "basic auth password for web UI (requires auth-user)")
var allowedOrigins = flag.String("allowed-origins", "", "comma-separated list of allowed origins for CORS (empty allows all)")

var upgrader websocket.Upgrader

func serveWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	name := r.URL.Query().Get("name")

	if room == "" {
		room = "lobby"
	}
	if name == "" {
		http.Error(w, "name parameter is required", http.StatusBadRequest)
		return
	}

	// Check if room is allowed
	if !hub.IsRoomAllowed(room) {
		http.Error(w, "Room does not exist", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ERROR] Failed to upgrade connection: %v", err)
		return
	}

	// Generate unique session ID
	sessionID := generateSessionID()

	client := &Client{
		id:   sessionID,
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 1024),
		room: room,
		name: name,
	}

	client.hub.register <- client

	go client.writeLoop()
	go client.readLoop()
}

// basicAuth performs HTTP Basic Authentication
func basicAuth(username, password string, w http.ResponseWriter, r *http.Request) bool {
	// Check if authentication is enabled
	if *authUser == "" || *authPassword == "" {
		return true // No authentication required
	}
	
	auth := r.Header.Get("Authorization")
	if auth == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	
	if !strings.HasPrefix(auth, "Basic ") {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	
	payload, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	
	// Use constant time comparison to prevent timing attacks
	userMatch := subtle.ConstantTimeCompare([]byte(pair[0]), []byte(username))
	passMatch := subtle.ConstantTimeCompare([]byte(pair[1]), []byte(password))
	
	if userMatch != 1 || passMatch != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	
	return true
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Check Basic Authentication for web UI
	if !basicAuth(*authUser, *authPassword, w, r) {
		return
	}
	
	http.ServeFile(w, r, "index.html")
}

// API response types
type RoomsResponse struct {
	Rooms []RoomInfo `json:"rooms"`
}

type CreateRoomRequest struct {
	Name string `json:"name"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// handleCreateRoom handles POST /api/rooms
func handleCreateRoom(hub *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	if req.Name == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Room name is required"})
		return
	}

	if err := hub.CreateRoom(req.Name); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "name": req.Name})
}

// handleGetRooms handles GET /api/rooms
func handleGetRooms(hub *Hub, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rooms := hub.GetRooms()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RoomsResponse{Rooms: rooms})
}

func main() {
	flag.Parse()
	hub := NewHub()
	
	// Configure allowed origins
	var allowedOriginsList []string
	if *allowedOrigins != "" {
		allowedOriginsList = strings.Split(*allowedOrigins, ",")
		for i, origin := range allowedOriginsList {
			allowedOriginsList[i] = strings.TrimSpace(origin)
		}
		log.Printf("[INFO] Allowed origins configured: %v", allowedOriginsList)
	} else {
		log.Printf("[INFO] All origins allowed (no restrictions)")
	}
	
	// Configure WebSocket upgrader
	upgrader.CheckOrigin = func(r *http.Request) bool {
		if len(allowedOriginsList) == 0 {
			// If no origins specified, allow all (backward compatible)
			return true
		}
		
		// Check if origin is in allowed list
		origin := r.Header.Get("Origin")
		for _, allowed := range allowedOriginsList {
			if origin == allowed {
				return true
			}
		}
		
		log.Printf("[WARN] Rejected WebSocket connection from origin: %s", origin)
		return false
	}
	
	// Set dynamic room creation policy
	hub.SetAllowDynamicRooms(*allowDynamicRooms)
	
	// If dynamic rooms are not allowed, create a default "lobby" room
	if !*allowDynamicRooms {
		hub.CreateRoom("lobby")
	}
	
	go hub.run()

	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWS(hub, w, r)
	})
	http.HandleFunc("/api/rooms", func(w http.ResponseWriter, r *http.Request) {
		// Configure CORS headers based on allowed origins
		origin := r.Header.Get("Origin")
		if len(allowedOriginsList) == 0 {
			// Allow all origins if none specified
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			// Check if origin is allowed
			originAllowed := false
			for _, allowed := range allowedOriginsList {
				if origin == allowed {
					originAllowed = true
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
			if !originAllowed && origin != "" {
				log.Printf("[WARN] Rejected API request from origin: %s", origin)
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
		}
		
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		switch r.Method {
		case http.MethodGet:
			handleGetRooms(hub, w, r)
		case http.MethodPost:
			handleCreateRoom(hub, w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	server := &http.Server{
		Addr: *addr,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("[INFO] Server starting on %s", *addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("[ERROR] ListenAndServe: ", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("[INFO] Shutting down server...")

	// Create a deadline for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	}

	// Shutdown the hub
	hub.shutdown()

	log.Println("[INFO] Server gracefully stopped")
}

// generateSessionID generates a unique session ID
func generateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		log.Printf("[WARN] Failed to generate random session ID: %v, using timestamp", err)
		timestamp := time.Now().UnixNano()
		return fmt.Sprintf("session-%d", timestamp)
	}
	return "session-" + hex.EncodeToString(bytes)
}