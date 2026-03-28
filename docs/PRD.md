# Quiz Forge - Product Requirements Document v2.0

**Stack:** Go + templ + HTMX + Tailwind  
**Database:** In-memory (interface-based for future PocketBase extension)  
**Deployment:** Single binary + Docker  

---

## 1. Project Overview

| Item | Description |
|------|-------------|
| **Name** | Quiz Forge |
| **Type** | Real-time multiplayer quiz platform |
| **Core** | Host creates quiz sessions, players join via QR/room code, answer in real-time |
| **Target** | Teachers, trainers, meeting facilitators, event organizers |

---

## 2. Architecture

```
┌────────────────────────────────────────────────────────────┐
│                      Quiz Forge (Go)                        │
├────────────────────────────────────────────────────────────┤
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐   │
│  │   templ      │  │    HTMX      │  │   Tailwind   │   │
│  │  (templates) │  │ (interactive) │  │   (styles)   │   │
│  └──────────────┘  └──────────────┘  └──────────────┘   │
├────────────────────────────────────────────────────────────┤
│                    Chi Router (HTTP)                       │
├────────────────────────────────────────────────────────────┤
│                  Quiz Service (Business Logic)              │
├────────────────────────────────────────────────────────────┤
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Repository Interface                     │  │
│  │   ┌─────────────────┐    ┌─────────────────────┐    │  │
│  │   │ MemoryStore v1  │    │ PocketBaseStore v2  │    │  │
│  │   │  (current)      │    │  (future)           │    │  │
│  │   └─────────────────┘    └─────────────────────┘    │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

---

## 3. Data Models

### 3.1 In-Memory Structures

```go
type Quiz struct {
    ID          string
    Title       string
    Description string
    Questions   []Question
    CreatedAt   time.Time
}

type Question struct {
    ID           string
    Text         string
    Answers      []string      // 2-6 options
    CorrectIndex int            // 0-based
    ImageURL     string         // optional
}

type Session struct {
    ID               string
    QuizID           string
    RoomCode         string    // 6-digit alphanumeric
    HostToken        string    // admin access
    Status           SessionStatus
    TimerSeconds     int
    ShuffleQuestions bool
    QuestionOrder    []int     // shuffled indices
    CurrentIndex     int
    Players          map[string]*Player
    Answers          map[string]*Answer // playerID -> answer
    CreatedAt        time.Time
    StartedAt        *time.Time
    EndedAt          *time.Time
}

type Player struct {
    ID        string
    Nickname  string
    Score     int
    TotalTime int64  // ms
    Answers   map[int]bool  // questionIndex -> correct
    Connected bool
}

type Answer struct {
    PlayerID      string
    QuestionIndex int
    SelectedIndex int
    ResponseTime  int64  // ms from question start
    SubmittedAt   time.Time
}
```

### 3.2 Repository Interface (Future-Proof)

```go
type QuizRepository interface {
    SaveQuiz(q *Quiz) error
    GetQuiz(id string) (*Quiz, error)
    ListQuizzes() []*Quiz
    DeleteQuiz(id string) error
}

type SessionRepository interface {
    CreateSession(s *Session) error
    GetSession(code string) (*Session, error)
    UpdateSession(s *Session) error
    ListActiveSessions() []*Session
    DeleteSession(code string) error
}
```

---

## 4. API Design

### 4.1 HTTP Routes

| Method | Path | Description |
|--------|------|-------------|
| **Host (Admin)** |
| GET | `/` | Landing page |
| GET | `/host` | Host dashboard |
| POST | `/host/quiz/create` | Create new quiz |
| GET | `/host/quiz/:id` | Edit quiz |
| POST | `/host/quiz/:id` | Save quiz |
| POST | `/host/quiz/:id/start` | Start session |
| POST | `/host/session/:code/next` | Next question |
| POST | `/host/session/:code/end-round` | Close voting |
| POST | `/host/session/:code/reveal` | Reveal answer |
| POST | `/host/session/:code/end` | End session |
| **Player (Public)** |
| GET | `/join/:code` | Join page |
| POST | `/join/:code` | Submit nickname |
| **API (JSON)** |
| GET | `/api/session/:code` | Session info |
| GET | `/api/session/:code/leaderboard` | Rankings |
| GET | `/api/session/:code/stats` | Answer distribution |

### 4.2 HTMX Partials

| Path | Purpose |
|------|---------|
| `/partials/session/:code/players` | Player count |
| `/partials/session/:code/question` | Question display |
| `/partials/session/:code/answer-form` | Answer options |
| `/partials/session/:code/result` | Answer result |
| `/partials/session/:code/leaderboard` | Rankings |

### 4.3 SSE Events

| Event | Payload | Recipients |
|-------|---------|------------|
| `player_joined` | nickname, count | Host |
| `player_left` | nickname, count | Host |
| `question_start` | index, text, timer | All |
| `question_end` | correctIndex, stats | All |
| `session_ended` | rankings | All |
| `timer_tick` | remaining | All |

---

## 5. Project Structure

```
quiz-forge/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── models/
│   │   ├── quiz.go
│   │   ├── session.go
│   │   └── player.go
│   ├── repository/
│   │   ├── interface.go
│   │   └── memory/
│   │       └── store.go
│   ├── service/
│   │   ├── quiz.go
│   │   └── session.go
│   ├── handler/
│   │   ├── host.go
│   │   ├── player.go
│   │   └── api.go
│   ├── sse/
│   │   └── broker.go
│   └── router/
│       └── router.go
├── templates/
│   ├── base.html
│   ├── index.html
│   ├── host/
│   ├── play/
│   └── partials/
├── static/
│   ├── htmx.min.js
│   └── app.js
├── go.mod
├── go.sum
└── Dockerfile
```

---

## 6. Features

### Quiz Management
- Create/edit/delete quiz
- 2-20 questions per quiz
- 2-6 answer options per question
- Mark correct answer
- Reorder questions
- Image upload per question (base64)
- **Built-in sample quiz**

### Hosting
- Select quiz from list
- Configure timer (0=unlimited)
- Shuffle toggle
- 6-digit room code
- QR code generation
- Real-time player list
- Start/next/end round controls
- Reveal answer + stats
- Pause/resume
- End session + final results

### Player
- Scan QR or enter code
- Nickname entry
- Waiting room
- Question display
- Click to answer
- Timer countdown
- Correct/incorrect feedback
- Final score display

### Scoring
- 1 point per correct
- Tiebreaker: avg response time (lower = better)

---

## 7. Built-in Sample Quiz

```go
var SampleQuiz = &Quiz{
    ID:          "sample",
    Title:       "Getting Started",
    Description: "Learn how Quiz Forge works",
    Questions: []Question{
        {Text: "What color is the sky?", Answers: []string{"Red","Blue","Green","Yellow"}, CorrectIndex: 1},
        {Text: "How many days in a week?", Answers: []string{"5","6","7","8"}, CorrectIndex: 2},
        {Text: "Which animal says Meow?", Answers: []string{"Dog","Cat","Cow","Duck"}, CorrectIndex: 1},
    },
}
```

---

## 8. Deployment

### Binary
```bash
go build -o quiz-forge ./cmd/server
./quiz-forge
```

### Docker
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o quiz-forge ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/quiz-forge .
EXPOSE 8080
CMD ["./quiz-forge"]
```

---

## 9. Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP port |
| `SESSION_SECRET` | auto | Host token signing |
| `MAX_PLAYERS` | `100` | Per session |
| `MAX_QUESTIONS` | `50` | Per quiz |

---

## 10. Implementation Phases

| Phase | Tasks |
|-------|-------|
| 1 | Project setup, router, memory store, base templates |
| 2 | Quiz CRUD, sample quiz, editor |
| 3 | Session flow, room codes, QR, player join |
| 4 | Gameplay, questions, timer, scoring |
| 5 | SSE realtime, HTMX polling, leaderboard |
| 6 | Stats, polish, error handling |
| 7 | Docker, deployment docs |

---

## 11. Open Source

- **License:** MIT
- **Telemetry:** None
- **Analytics:** None
- **Dependencies:** MIT/Apache-2/BSD only
