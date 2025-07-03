# AITuber OnAir Bushitsu Message Specification

## Overview
This document defines the message format for communication between the AITuber OnAir Bushitsu server and clients, as well as the REST API specifications for room management.

## Message Structure

### Server to Client Messages

All messages from server to client follow this structure:

```json
{
  "type": "message_type",
  "room": "room_name",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    // Type-specific data
  }
}
```

### Message Types

#### 1. Chat Message (`type: "chat"`)
Regular chat messages and mentions.

```json
{
  "type": "chat",
  "room": "lobby",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "from": "alice",
    "fromId": "session-a1b2c3d4e5f67890abcdef1234567890",  // Unique session identifier
    "text": "Hello @bob!",
    "mention": ["bob"]  // Array of mentioned users
  }
}
```

**Fields:**
- `from`: Username of the sender
- `fromId`: Unique session ID for the sender (format: `session-` + 32 hex characters)
- `text`: The message content
- `mention`: Array of usernames mentioned in the message

#### 2. User Event (`type: "user_event"`)
User join/leave notifications.

```json
{
  "type": "user_event",
  "room": "lobby",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "event": "join",  // "join" or "leave"
    "user": "alice"
  }
}
```

#### 3. System Event (`type: "system"`)
System-level notifications.

```json
{
  "type": "system",
  "room": "lobby",
  "timestamp": "2024-01-15T10:30:00Z",
  "data": {
    "event": "room_created",
    "details": {}  // Optional additional information
  }
}
```

### Client to Server Messages

Clients send simplified messages:

```json
{
  "type": "chat",
  "text": "Hello @bob!"
}
```

The server automatically:
- Adds room information from connection context
- Adds sender information from connection context
- Adds unique session ID for the sender
- Adds timestamp
- Parses mentions from text

## Implementation Notes

### Message Validation

1. **Message Length**: Maximum 4096 characters
2. **Empty Messages**: Not allowed
3. **Control Characters**: Not allowed except for tab (\t), newline (\n), and carriage return (\r)

### Connection Management

1. **Send Channel Buffer**: Each client has a send channel with buffer size of 256
2. **Unresponsive Clients**: Automatically disconnected if send channel is full
3. **Empty Rooms**: When the last user leaves a room, no leave notification is sent and the room is deleted

## Implementation Notes

1. **Timestamps**: All timestamps use RFC3339 format in UTC
2. **Mentions**: Detected by `@username` pattern at the beginning of the message (only the first mention is recognized)
3. **Room Context**: Room is determined by WebSocket connection parameters
4. **User Context**: Username is determined by WebSocket connection parameters
5. **Session ID**: Each WebSocket connection receives a unique session ID
   - Format: `session-` followed by 32 hexadecimal characters
   - Generated using crypto/rand with timestamp-based fallback if random generation fails
   - Persists for the duration of the connection
   - Allows clients to identify their own messages even with duplicate usernames

## Benefits

1. **Localization**: UI text is generated client-side, enabling easy multi-language support
2. **Flexibility**: Clients can customize how messages are displayed
3. **Extensibility**: New message types can be added without breaking existing clients
4. **Clear Structure**: Type-based handling makes client implementation straightforward
5. **Message Ownership**: Session IDs enable reliable identification of own messages
6. **Multi-device Support**: Same username can be used across multiple devices/tabs

## REST API Specification

### Room Management API

#### 1. Get Room List
**Endpoint**: `GET /api/rooms`

**Response**: `200 OK`
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

**Description**: Returns a list of all available rooms with the current number of connected users.

#### 2. Create Room
**Endpoint**: `POST /api/rooms`

**Request Body**:
```json
{
  "name": "room_name"
}
```

**Response**: `201 Created`
```json
{
  "status": "created",
  "name": "room_name"
}
```

**Error Response**: `409 Conflict`
```json
{
  "error": "room already exists: room_name"
}
```

**Error Response**: `400 Bad Request`
```json
{
  "error": "Room name is required"
}
```

**Description**: Creates a new predefined room. Room names must be unique.

### Connection Error Handling

When connecting to a non-existent room (in predefined rooms mode):
- **HTTP Status**: `403 Forbidden`
- **Response**: "Room does not exist"
- **Behavior**: WebSocket upgrade is rejected before establishing connection

### Room Management Configuration

1. **Predefined Rooms Mode** (default)
   - Only rooms created via API can be joined
   - Connection attempts to non-existent rooms are rejected
   - Empty rooms persist until explicitly deleted
   - Default "lobby" room is auto-created on startup

2. **Dynamic Rooms Mode** (`-allow-dynamic-rooms` flag)
   - Any room name is accepted on connection
   - Rooms are created automatically when first user joins
   - Empty rooms are automatically deleted
   - Backward compatible with existing behavior