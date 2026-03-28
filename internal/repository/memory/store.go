package memory

import (
	"sync"

	"github.com/google/uuid"
	"github.com/quizforge/quiz-forge/internal/models"
)

type MemoryStore struct {
	quizzes  map[string]*models.Quiz
	sessions map[string]*models.Session
	mu       sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	store := &MemoryStore{
		quizzes:  make(map[string]*models.Quiz),
		sessions: make(map[string]*models.Session),
	}
	store.initSampleQuiz()
	return store
}

func (s *MemoryStore) initSampleQuiz() {
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
	s.quizzes[sample.ID] = sample
}

func (s *MemoryStore) SaveQuiz(q *models.Quiz) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if q.ID == "" {
		q.ID = uuid.New().String()
	}
	s.quizzes[q.ID] = q
	return nil
}

func (s *MemoryStore) GetQuiz(id string) (*models.Quiz, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if quiz, ok := s.quizzes[id]; ok {
		return quiz, nil
	}
	return nil, nil
}

func (s *MemoryStore) ListQuizzes() []*models.Quiz {
	s.mu.RLock()
	defer s.mu.RUnlock()

	quizzes := make([]*models.Quiz, 0, len(s.quizzes))
	for _, q := range s.quizzes {
		quizzes = append(quizzes, q)
	}
	return quizzes
}

func (s *MemoryStore) DeleteQuiz(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.quizzes, id)
	return nil
}

func (s *MemoryStore) CreateSession(session *models.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session.ID == "" {
		session.ID = uuid.New().String()
	}
	s.sessions[session.RoomCode] = session
	return nil
}

func (s *MemoryStore) GetSession(code string) (*models.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if session, ok := s.sessions[code]; ok {
		return session, nil
	}
	return nil, nil
}

func (s *MemoryStore) UpdateSession(session *models.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.RoomCode] = session
	return nil
}

func (s *MemoryStore) ListActiveSessions() []*models.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*models.Session, 0)
	for _, sess := range s.sessions {
		if sess.Status != models.StatusEnded {
			sessions = append(sessions, sess)
		}
	}
	return sessions
}

func (s *MemoryStore) DeleteSession(code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.sessions, code)
	return nil
}
