# Eventing WAL Toy

This is a toy application that demonstrates some event ingestion patterns I've been thinking about. It's almost entirely written with Cursor and is not designed for production use.

The main idea is that after doing the business logic, we want to persist the events to a "sink" of some kind. In this case it's postgres, but it could be a message broker too. We want to prioritize throughput without sacrificing durability, hence the WAL implementation.

## 🚀 Quick Start

### Prerequisites

- Go 1.24.1 or later
- Docker and Docker Compose
- Node.js and pnpm (for load testing)

### 1. Start the Database

```bash
docker-compose up -d postgres
```

### 3. Start the Server

```bash
go run cmd/server/main.go
```

The server will start on `http://localhost:8080` by default.

## 📋 Configuration

The application uses environment variables for configuration:

| Variable              | Default                                                                    | Description                                |
| --------------------- | -------------------------------------------------------------------------- | ------------------------------------------ |
| `PORT`                | `8080`                                                                     | HTTP server port                           |
| `DB_CONN_STRING`      | `postgres://postgres:postgres@localhost:5444/eventing_wal?sslmode=disable` | PostgreSQL connection string               |
| `EVENT_CONSUMER_TYPE` | `gochannel`                                                                | Event consumer type (`gochannel` or `wal`) |

### Event Consumer Types

The application supports two different event consumer implementations:

- **`gochannel`** (default): Uses in-memory Go channels for event processing. This is faster and simpler but events are lost if the application crashes.
- **`wal`**: Uses Write-Ahead Log (WAL) for durable event processing. Events are persisted to disk and can be recovered after crashes, but with higher latency.

## 🔧 Development

### Project Structure

```
eventing-wal-example/
├── cmd/                    # Application entry points
│   ├── migrate/           # Database migration tool
│   └── server/            # HTTP server
├── internal/              # Internal application code
│   ├── domain/            # Business logic and domain models
│   ├── events/            # Event handling and consumer
│   ├── migrations/        # Database migration utilities
│   ├── repository/        # Data access layer
│   └── server/            # HTTP server implementation
├── migrations/            # SQL migration files
├── testing/               # Load testing with k6
└── docker-compose.yml     # Local development environment
```

## 📡 API Reference

### Health Check

```http
GET /healthz
```

Returns server health status and uptime information.

### User Management

#### Create User

```http
POST /users
Content-Type: application/json

{
  "email": "user@example.com",
  "name": "John Doe"
}
```

**Response:**

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "John Doe",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Load Testing

For detailed information about load testing this API, see the [Load Testing README](./testing/README.md).

```

```
