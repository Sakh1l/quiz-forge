package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/quizforge/quiz-forge/internal/models"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if dbPath == "" {
		dbPath = "quiz-forge.db"
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS quizzes (
		id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		room_code TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		created_at TEXT DEFAULT CURRENT_TIMESTAMP,
		updated_at TEXT DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_created_at ON sessions(created_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) SaveQuiz(q *models.Quiz) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if q.ID == "" {
		q.ID = uuid.New().String()
	}

	data, err := json.Marshal(q)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO quizzes (id, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET data = ?, updated_at = ?
	`, q.ID, data, time.Now().UTC().Format(time.RFC3339), data, time.Now().UTC().Format(time.RFC3339))

	return err
}

func (s *SQLiteStore) GetQuiz(id string) (*models.Quiz, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	err := s.db.QueryRow("SELECT data FROM quizzes WHERE id = ?", id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var quiz models.Quiz
	if err := json.Unmarshal(data, &quiz); err != nil {
		return nil, err
	}

	return &quiz, nil
}

func (s *SQLiteStore) ListQuizzes() ([]*models.Quiz, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT data FROM quizzes ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var quizzes []*models.Quiz
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var quiz models.Quiz
		if err := json.Unmarshal(data, &quiz); err != nil {
			return nil, err
		}
		quizzes = append(quizzes, &quiz)
	}

	return quizzes, nil
}

func (s *SQLiteStore) DeleteQuiz(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM quizzes WHERE id = ?", id)
	return err
}

func (s *SQLiteStore) CreateSession(session *models.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO sessions (room_code, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(room_code) DO UPDATE SET data = ?, updated_at = ?
	`, session.RoomCode, data, time.Now().UTC().Format(time.RFC3339), data, time.Now().UTC().Format(time.RFC3339))

	return err
}

func (s *SQLiteStore) GetSession(code string) (*models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var data []byte
	err := s.db.QueryRow("SELECT data FROM sessions WHERE room_code = ?", code).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var session models.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *SQLiteStore) UpdateSession(session *models.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(session)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		INSERT INTO sessions (room_code, data, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(room_code) DO UPDATE SET data = ?, updated_at = ?
	`, session.RoomCode, data, time.Now().UTC().Format(time.RFC3339), data, time.Now().UTC().Format(time.RFC3339))

	return err
}

func (s *SQLiteStore) ListActiveSessions() ([]*models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query("SELECT data FROM sessions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*models.Session
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var session models.Session
		if err := json.Unmarshal(data, &session); err != nil {
			return nil, err
		}
		if session.Status != models.StatusEnded {
			sessions = append(sessions, &session)
		}
	}

	return sessions, nil
}

func (s *SQLiteStore) DeleteSession(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM sessions WHERE room_code = ?", code)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) InitSampleQuiz() error {
	store := &SQLiteStore{db: s.db}
	existing, err := store.ListQuizzes()
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		return nil
	}

	sample := &models.Quiz{
		ID:          "sample",
		Title:       "Getting Started with Quiz Forge",
		Description: "A quick tour to learn how Quiz Forge works",
		Questions: []models.Question{
			{
				ID:           uuid.New().String(),
				Text:         "What color is the sky on a clear day?",
				Answers:      []string{"Red", "Blue", "Green", "Yellow"},
				CorrectIndex: 1,
			},
			{
				ID:           uuid.New().String(),
				Text:         "How many days are in a week?",
				Answers:      []string{"5", "6", "7", "8"},
				CorrectIndex: 2,
			},
			{
				ID:           uuid.New().String(),
				Text:         "Which animal says 'Meow'?",
				Answers:      []string{"Dog", "Cat", "Cow", "Duck"},
				CorrectIndex: 1,
			},
		},
	}

	return s.SaveQuiz(sample)
}

func getEnvDatabasePath() string {
	if path := os.Getenv("DATABASE_PATH"); path != "" {
		return path
	}
	if os.Getenv("APP_ENV") == "production" {
		return "/var/lib/quiz-forge/quiz-forge.db"
	}
	return "quiz-forge.db"
}
