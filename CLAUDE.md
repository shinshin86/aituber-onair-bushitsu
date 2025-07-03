# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Build and Run
```bash
# Install dependencies
go mod download

# Build the server
go build -o bushitsu .

# Run with default settings (port 8080)
./bushitsu

# Run with custom port
./bushitsu -addr :3000

# Run with dynamic room creation enabled
./bushitsu -allow-dynamic-rooms

# Run with Basic authentication for web UI
./bushitsu -auth-user admin -auth-password secret

# Run with allowed origins restriction
./bushitsu -allowed-origins "https://example.com,https://app.example.com"

# Run with multiple options
./bushitsu -addr :3000 -allow-dynamic-rooms -auth-user admin -auth-password secret -allowed-origins "https://example.com"

# Clean build artifacts
rm bushitsu chat-server
```

### Testing
- Open browser to http://localhost:8080 for manual testing via web UI
- Multiple browser tabs/windows can simulate multiple users
- No automated tests currently exist

## Architecture Overview

This is AITuber OnAir Bushitsu, a real-time WebSocket chat server with a **hub-and-spoke architecture** designed for AITuber streaming environments:

### Core Components
- **Hub**: Central singleton managing all connections, message routing, and room management
- **Client**: Individual WebSocket connection handler with read/write goroutines  
- **HTTP Server**: Handles WebSocket upgrades, REST API endpoints, and serves test UI

### Key Interaction Flow
1. Client connects via HTTP upgrade to `/ws?room=ROOM&name=USER`
2. Hub validates room existence (unless dynamic rooms are enabled)
3. Client registers with Hub and joins specified room
4. Hub routes messages between clients based on room/mention targeting
5. Each client runs concurrent read/write loops for real-time communication

### Concurrency Model
- **One goroutine per client connection** (read + write loops)
- **Hub runs in single goroutine** processing registration/broadcast channels
- **Mutex protection** for shared Hub state (rooms, clients maps)
- **Channel-based communication** between Hub and clients

## Key Implementation Details

### Message Format
All messages use structured JSON with `type`, `room`, `timestamp`, and `data` fields. See MESSAGE_SPEC.md for complete specification.

### Session Management
- Each client gets unique session ID (crypto/rand generated)
- Session IDs enable client-side identification of own messages
- Fallback to timestamp-based IDs if random generation fails

### Connection Management  
- Automatic cleanup of unresponsive clients after 5-second timeout
- Graceful shutdown on SIGINT/SIGTERM with 10-second timeout
- Empty rooms automatically deleted when last user leaves (dynamic rooms only)
- Improved message delivery reliability with timeout-based sending
- Room validation on connection (predefined rooms only)

### Message Validation
- 4096 character limit on message text
- Control characters blocked except tab/newline/carriage return
- @mentions only recognized at message start (first mention only)

### Configuration Constants
```go
// client.go
writeWait        = 10 * time.Second   // Write timeout
pongWait         = 60 * time.Second   // Pong wait time  
pingPeriod       = 30 * time.Second   // Ping interval
maxMessageSize   = 512 * 1024         // Max WebSocket message
maxMessageLength = 4096               // Max text length

// Buffer sizes (main.go, hub.go)
sendChannelBuffer   = 1024            // Client send channel buffer
broadcastBuffer     = 1024            // Broadcast channel buffer

// Message sending (hub.go)
sendTimeout      = 5 * time.Second    // Timeout for message delivery
```

## Security Considerations

- **CORS configurable with `-allowed-origins` flag** (defaults to allow all origins)
- **Basic Authentication** - Optional HTTP Basic auth for web UI access only (`-auth-user` and `-auth-password` flags)
- **WebSocket/API access** - No authentication required for WebSocket connections or REST API
- **Input validation** prevents control character injection
- **Rate limiting** not implemented
- **Race condition prevention** in unregister process
- **Detailed error logging** for troubleshooting
- **Room access control** - connections only allowed to predefined rooms (configurable)

## File Organization

- `main.go` - HTTP server, WebSocket upgrade, REST API endpoints, graceful shutdown
- `hub.go` - Central message routing, connection management, and room management
- `client.go` - WebSocket client handling and message validation
- `index.html` - Browser-based test interface with Japanese UI and room management features
- `README.md` - Comprehensive documentation (English)
- `README.ja.md` - Comprehensive documentation (Japanese)
- `MESSAGE_SPEC.md` - Detailed message format specification

## Room Management

### Configuration
- **Default Mode**: Only predefined rooms allowed (secure)
- **Dynamic Mode**: Any room name accepted (`-allow-dynamic-rooms` flag)
- **Default Room**: "lobby" is created automatically in predefined mode

### REST API Endpoints
```bash
# Get list of rooms
curl http://localhost:8080/api/rooms

# Create a new room
curl -X POST http://localhost:8080/api/rooms \
  -H "Content-Type: application/json" \
  -d '{"name": "development"}'
```

### Room Management Features
- Predefined room creation via API
- Room list with active user counts
- Automatic room validation on connection
- Dynamic room creation toggle
- Visual room selection in web UI