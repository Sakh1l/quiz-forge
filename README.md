# Quiz Forge - Real-time Quiz Platform

A lightweight, self-hosted quiz platform built with Go. Host real-time multiplayer quizzes with room codes and instant player joining.

## Features

- **Real-time Quiz Sessions** - Players join via QR code or room code
- **Live Updates** - Server-Sent Events (SSE) for instant synchronization
- **Built-in Sample Quiz** - Test immediately after starting
- **Quiz Editor** - Create and manage custom quizzes
- **Leaderboards** - Real-time rankings with tiebreaker by response time
- **HTMX Powered** - Minimal JavaScript, server-rendered interactivity
- **Single Binary** - No external dependencies required

## Quick Start

```bash
# Download the latest release for your platform
# Or build from source:

go build -o quiz-forge ./cmd/server
./quiz-forge
```

Visit http://localhost:8080

## Development

```bash
# Install dependencies
go mod tidy

# Run in development
go run ./cmd/server
```

## Configuration

| Environment | Default | Description |
|------------|---------|-------------|
| `PORT` | `8080` | HTTP server port |
| `SESSION_SECRET` | auto-generated | Host token signing |
| `MAX_PLAYERS` | `100` | Max players per session |
| `MAX_QUESTIONS` | `50` | Max questions per quiz |

## Architecture

```
┌─────────────────────────────────────┐
│          Go + Chi Router            │
├─────────────────────────────────────┤
│  templ (templates) + HTMX          │
├─────────────────────────────────────┤
│  In-Memory Store (interface-based)  │
└─────────────────────────────────────┘
```

## Project Structure

```
quiz-forge/
├── cmd/server/          # Entry point
├── internal/
│   ├── config/         # Configuration
│   ├── models/         # Data models
│   ├── repository/     # Data access layer
│   ├── handler/        # HTTP handlers
│   ├── sse/           # Server-Sent Events
│   └── router/         # Route definitions
├── static/             # Static files
└── templates/          # HTML templates
```

## License

MIT License - See LICENSE file
