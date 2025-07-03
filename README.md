# AITuber OnAir Bushitsu

![AITuber OnAir Bushitsu](./images/logo.png)

A real-time WebSocket chat server implementation in Go with room support, @mentions, and join/leave notifications, designed for AITuber streaming environments.

[日本語版](./README.ja.md)

## What is “Bushitsu”?
**Bushitsu** (部室, *boo-she-tsu*) literally means “clubroom” in Japanese — a relaxed space where members gather to chat, create, and collaborate. This project brings that always-open digital clubroom experience to AITubers.

## Features

- 🚀 **High Performance**: Efficient concurrent processing with Go
- 🏠 **Room Support**: Multiple chat rooms (predefined/dynamic modes)
- 💬 **@Mentions**: Direct messaging to specific users
- 📢 **Join/Leave Notifications**: Automatic user join/leave announcements
- 🔄 **Real-time Communication**: Bidirectional WebSocket communication
- 🌐 **Structured Messages**: Extensible JSON message format
- 🆔 **Session IDs**: Unique ID per connection for reliable message identification
- 🛡️ **Enhanced Security**: Race condition prevention, message validation, graceful shutdown, room access control
- 📊 **High Reliability**: Timeout-based message sending, detailed error logging, message drop prevention
- 🏃 **Single Binary**: Easy deployment with single executable file

## Architecture

```
Client ──HTTP Upgrade──▶ /ws?room=ROOM&name=USER
             │
             ▼
      Hub (singleton)
        ├─ rooms: map[string]map[*client]struct{}
        └─ route() – broadcast / mention
```

- **Hub**: Singleton managing all connections
- **Client**: One goroutine per WebSocket connection
- **Message Routing**: Supports broadcast and @mention delivery

## Message Specification

See [MESSAGE_SPEC.md](./MESSAGE_SPEC.md) for detailed specification.

### Message Types

1. **chat**: Regular chat messages
2. **user_event**: User join/leave events
3. **system**: System messages

### Sample Message

```json
{
  "type": "chat",
  "room": "lobby",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "from": "alice",
    "fromId": "session-a1b2c3d4e5f67890abcdef1234567890",
    "text": "Hello @bob!",
    "mention": ["bob"]
  }
}
```

## Quick Start

### Requirements

- Go 1.21 or later

### Build and Run

```bash
# Install dependencies
go mod download

# Build
go build -o bushitsu .

# Run with default settings (port 8080)
./bushitsu

# Run with custom port
./bushitsu -addr :3000

# Run with dynamic room creation enabled
./bushitsu -allow-dynamic-rooms

# Run with Basic auth for web UI
./bushitsu -auth-user admin -auth-password secret

# Run with allowed origins restriction
./bushitsu -allowed-origins "https://example.com,https://app.example.com"

# Run with multiple options
./bushitsu -addr :3000 -allow-dynamic-rooms -auth-user admin -auth-password secret -allowed-origins "https://example.com"
```

### Development Testing

Open http://localhost:8080 in your browser to access the test UI.
- Only served at root path (`/`)
- Only accepts GET requests

## API Specification

### REST API Endpoints

#### Get Room List
```
GET /api/rooms
```

**Response Example**:
```json
{
  "rooms": [
    {
      "name": "lobby",
      "userCount": 5
    },
    {
      "name": "development",
      "userCount": 2
    }
  ]
}
```

#### Create Room
```
POST /api/rooms
Content-Type: application/json

{
  "name": "new_room"
}
```

**Response Example**:
- Success (201 Created):
```json
{
  "status": "created",
  "name": "new_room"
}
```
- Error (409 Conflict):
```json
{
  "error": "room already exists: new_room"
}
```

### WebSocket Endpoint

```
GET /ws?room=<room_name>&name=<user_name>
```

| Parameter | Description | Required | Default |
|-----------|-------------|----------|---------|
| room | Room name to join | No | lobby |
| name | User name | Yes | - |

### Client Message Format

```json
{
  "type": "chat",
  "text": "message content"
}
```

## Implementation Details

### File Structure

- `main.go` - Entry point and HTTP server
- `hub.go` - Connection management and message routing
- `client.go` - WebSocket client handling
- `index.html` - Development test UI

### Security and Operation Specifications

- **CORS**: Configurable origin restrictions with `-allowed-origins` flag (defaults to allow all origins)
- **Basic Auth**: Optional HTTP Basic auth for Web UI (index.html) only (WebSocket/API excluded)
- **Session ID Generation**: Uses crypto/rand, falls back to timestamp-based ID on failure
- **Unresponsive Client Handling**: Disconnects after 5-second timeout when send channel is full
- **Empty Room Handling**: In dynamic room mode, deletes room when last user leaves
- **Room Access Control**: 
  - Default (predefined mode): Only predefined rooms are accessible
  - Dynamic mode (`-allow-dynamic-rooms`): Any room name is accessible

### Room Management Modes

1. **Predefined Mode (Default)**
   - Only rooms created via REST API are accessible
   - Connections to non-existent rooms return 403 error
   - "lobby" room is auto-created on startup
   - Empty rooms are retained

2. **Dynamic Mode (`-allow-dynamic-rooms` flag)**
   - Any room name is accessible
   - Rooms are auto-created on connection
   - Rooms are deleted when last user leaves
   - Compatible with legacy behavior

### Key Constants

- **writeWait**: 10 seconds - Write timeout
- **pongWait**: 60 seconds - Pong wait time
- **pingPeriod**: 30 seconds - Ping interval (set to half of pongWait)
- **maxMessageSize**: 512KB - Max message size
- **maxMessageLength**: 4096 chars - Max message text length
- **sendChannelBuffer**: 1024 - Client send channel buffer size
- **broadcastBuffer**: 1024 - Broadcast channel buffer size
- **sendTimeout**: 5 seconds - Message send timeout

## Graceful Shutdown

When the server receives `SIGINT` (Ctrl+C) or `SIGTERM` signal, it shuts down safely following these steps:

1. Stop accepting new connections
2. Close all existing client connections properly
3. Shutdown HTTP server with 10-second timeout
4. Clean up Hub resources

The shutdown process has a 10-second timeout.

## Production Deployment

### systemd Service Example

```ini
[Unit]
Description=AITuber OnAir Bushitsu Server
After=network.target

[Service]
Type=simple
User=bushitsu
ExecStart=/opt/bushitsu/bushitsu
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Nginx Reverse Proxy Configuration Example

```nginx
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

## Test UI Features

The browser-based test UI includes the following features:

- **Room Management Panel**: Create, list, and select rooms
- **Auto Room Updates**: Updates connected user count every 5 seconds
- **Click-to-Join Rooms**: Direct connection from room list
- **Session ID Display**: Identifies message senders
- **Message Highlighting**: Color-coded own messages, mentions, and sent mentions

## Limitations

- No duplicate username checking (distinguishable by session ID)
- No message history persistence
- Basic auth for Web UI only (optional, WebSocket/API not authenticated)
- Message length limited to 4096 characters
- Control characters not allowed (except tab, newline, carriage return)
- @mentions only recognized at message start (first one only)
- CORS allows all origins by default (use `-allowed-origins` flag for production)
- Room deletion API not implemented (planned for future)

## License

MIT License

## Contributing

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.