# Meet App Backend

Backend service for the Meet App - a Google Meet clone built with Go and Gin framework.

## Architecture

This backend follows a clean architecture pattern with clear separation of concerns:

```
backend/
├── cmd/server/          # Application entry point
├── internal/
│   ├── api/
│   │   ├── handlers/    # HTTP request handlers
│   │   └── middleware/  # HTTP middleware (auth, logging, CORS, etc.)
│   ├── config/          # Configuration management
│   ├── models/          # Database models (GORM)
│   ├── repository/      # Data access layer
│   ├── service/         # Business logic layer
│   ├── websocket/       # WebSocket signaling (to be implemented)
│   └── sse/             # Server-Sent Events (to be implemented)
├── pkg/
│   ├── auth/            # JWT authentication & password hashing
│   ├── database/        # Database connection management
│   └── utils/           # Utility functions
└── migrations/          # SQL migration files
```

## Tech Stack

- **Framework**: Gin (HTTP router)
- **Database**: PostgreSQL (with GORM ORM)
- **Cache/PubSub**: Redis
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **Password Hashing**: bcrypt
- **Real-time**: WebSocket (gorilla/websocket), SSE

## Features Implemented

### Authentication
- ✅ User registration with email, username, and password
- ✅ Login with JWT token generation
- ✅ Token refresh mechanism
- ✅ Password hashing with bcrypt
- ✅ Protected routes with JWT middleware

### Meeting Management
- ✅ Create meetings with custom settings
- ✅ Join meetings by code
- ✅ Leave meetings
- ✅ End meetings (host only)
- ✅ Get meeting participants
- ✅ Auto-start meeting when first participant joins

### Chat
- ✅ Send messages in meetings
- ✅ Get meeting message history
- ✅ Support for text, system, and file message types

### Database
- ✅ PostgreSQL connection with connection pooling
- ✅ Redis connection for pub/sub
- ✅ GORM auto-migration
- ✅ SQL migration files
- ✅ Soft delete support

### Middleware
- ✅ CORS handling
- ✅ JWT authentication
- ✅ Request logging
- ✅ Error handling
- ✅ Request ID tracking

## API Endpoints

### Authentication
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login user
- `POST /api/auth/refresh` - Refresh access token
- `GET /api/auth/me` - Get current user (protected)
- `POST /api/auth/logout` - Logout user (protected)

### Meetings
All meeting endpoints require authentication.

- `POST /api/meetings` - Create new meeting
- `POST /api/meetings/join` - Join meeting by code
- `GET /api/meetings/code/:code` - Get meeting by code
- `POST /api/meetings/:id/leave` - Leave meeting
- `POST /api/meetings/:id/end` - End meeting (host only)
- `GET /api/meetings/:id/participants` - Get meeting participants
- `POST /api/meetings/:id/messages` - Send chat message
- `GET /api/meetings/:id/messages` - Get chat messages

### Health
- `GET /health` - Health check
- `GET /ready` - Readiness check

### WebSocket (To be implemented)
- `GET /ws` - WebSocket endpoint for signaling

## Environment Variables

Create a `.env` file in the backend directory:

```env
# Server
PORT=8080
ENVIRONMENT=development
GIN_MODE=debug

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=meetapp
DB_PASSWORD=meetapp123
DB_NAME=meetapp
DB_SSLMODE=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# JWT
JWT_SECRET=your-super-secret-key-change-this
JWT_EXPIRY_HOURS=24
JWT_REFRESH_HOURS=168

# MinIO
MINIO_ENDPOINT=localhost:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin123
MINIO_USE_SSL=false
MINIO_BUCKET_NAME=meeting-recordings

# WebRTC
STUN_SERVER=stun:stun.l.google.com:19302
TURN_SERVER=
TURN_USERNAME=
TURN_PASSWORD=
```

## Getting Started

### Prerequisites
- Go 1.21 or higher
- PostgreSQL 14+
- Redis 7+
- Docker (optional, for running services)

### Installation

1. Install dependencies:
```bash
cd backend
go mod download
```

2. Set up PostgreSQL database:
```bash
# Using Docker
docker run -d \
  --name meetapp-postgres \
  -e POSTGRES_USER=meetapp \
  -e POSTGRES_PASSWORD=meetapp123 \
  -e POSTGRES_DB=meetapp \
  -p 5432:5432 \
  postgres:14
```

3. Set up Redis:
```bash
# Using Docker
docker run -d \
  --name meetapp-redis \
  -p 6379:6379 \
  redis:7-alpine
```

4. Copy environment file:
```bash
cp .env.example .env
# Edit .env with your configuration
```

5. Run database migrations:
The application uses GORM auto-migration, so migrations will run automatically on startup. Alternatively, you can run SQL migrations manually:
```bash
psql -U meetapp -d meetapp -f migrations/001_initial_schema.up.sql
```

6. Run the server:
```bash
go run cmd/server/main.go
```

The server will start on `http://localhost:8080`

### Development

Run with auto-reload (using air):
```bash
# Install air
go install github.com/air-verse/air@latest

# Run with air
air
```

Build for production:
```bash
go build -o bin/server cmd/server/main.go
./bin/server
```

## Database Schema

### Users
- id (UUID, PK)
- email (unique)
- username (unique)
- password (hashed)
- name
- avatar_url
- timestamps

### Meetings
- id (UUID, PK)
- code (unique, 10 chars)
- title
- description
- host_id (FK to users)
- status (scheduled, active, ended)
- scheduled_at, started_at, ended_at
- max_users (default: 50)
- is_recording
- recording_url
- settings (JSONB)
- timestamps

### Participants
- id (UUID, PK)
- meeting_id (FK to meetings)
- user_id (FK to users)
- role (host, moderator, guest)
- joined_at, left_at
- is_muted, is_video_on, is_sharing
- timestamps

### Messages
- id (UUID, PK)
- meeting_id (FK to meetings)
- user_id (FK to users)
- type (text, system, file)
- content
- file_url
- timestamps

## Testing

Run tests:
```bash
go test ./... -v
```

Run with coverage:
```bash
go test ./... -cover
```

## Next Steps (To be implemented)

1. **WebSocket Signaling**
   - Implement WebSocket hub for managing connections
   - Handle WebRTC signaling messages (offer, answer, ICE candidates)
   - Room management for meetings

2. **Server-Sent Events (SSE)**
   - Implement SSE for chat and notifications
   - Participant join/leave events
   - Meeting status updates

3. **File Upload**
   - MinIO integration for file storage
   - Avatar uploads
   - Chat file attachments

4. **Recording**
   - Meeting recording functionality
   - Recording storage in MinIO
   - Recording playback

5. **Testing**
   - Unit tests for all layers
   - Integration tests for API endpoints
   - E2E tests

6. **Monitoring**
   - Prometheus metrics
   - Structured logging
   - Distributed tracing

## License

MIT
