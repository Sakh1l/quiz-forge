package models

import "time"

type SessionStatus string

const (
	StatusWaiting SessionStatus = "waiting"
	StatusActive  SessionStatus = "active"
	StatusPaused  SessionStatus = "paused"
	StatusReveal  SessionStatus = "reveal"
	StatusEnded   SessionStatus = "ended"
)

type Session struct {
	ID               string             `json:"id"`
	QuizID           string             `json:"quiz_id"`
	RoomCode         string             `json:"room_code"`
	HostToken        string             `json:"-"`
	Status           SessionStatus      `json:"status"`
	TimerSeconds     int                `json:"timer_seconds"`
	ShuffleQuestions bool               `json:"shuffle_questions"`
	QuestionOrder    []int              `json:"question_order"`
	CurrentIndex     int                `json:"current_index"`
	Players          map[string]*Player `json:"players"`
	Answers          map[string]*Answer `json:"answers"`
	CreatedAt        time.Time          `json:"created_at"`
	StartedAt        *time.Time         `json:"started_at,omitempty"`
	EndedAt          *time.Time         `json:"ended_at,omitempty"`
}

type Player struct {
	ID        string       `json:"id"`
	Nickname  string       `json:"nickname"`
	Score     int          `json:"score"`
	TotalTime int64        `json:"total_time"`
	Answers   map[int]bool `json:"answers"`
	Connected bool         `json:"connected"`
}

type Answer struct {
	PlayerID      string    `json:"player_id"`
	QuestionIndex int       `json:"question_index"`
	SelectedIndex int       `json:"selected_index"`
	ResponseTime  int64     `json:"response_time_ms"`
	SubmittedAt   time.Time `json:"submitted_at"`
}

func NewSession(quizID, roomCode, hostToken string, timerSeconds int, shuffle bool) *Session {
	return &Session{
		ID:               "",
		QuizID:           quizID,
		RoomCode:         roomCode,
		HostToken:        hostToken,
		Status:           StatusWaiting,
		TimerSeconds:     timerSeconds,
		ShuffleQuestions: shuffle,
		QuestionOrder:    []int{},
		CurrentIndex:     -1,
		Players:          make(map[string]*Player),
		Answers:          make(map[string]*Answer),
		CreatedAt:        time.Now(),
	}
}

func NewPlayer(id, nickname string) *Player {
	return &Player{
		ID:        id,
		Nickname:  nickname,
		Score:     0,
		TotalTime: 0,
		Answers:   make(map[int]bool),
		Connected: true,
	}
}
