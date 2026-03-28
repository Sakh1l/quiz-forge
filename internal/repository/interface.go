package repository

import (
	"github.com/quizforge/quiz-forge/internal/models"
)

type QuizRepository interface {
	SaveQuiz(q *models.Quiz) error
	GetQuiz(id string) (*models.Quiz, error)
	ListQuizzes() []*models.Quiz
	DeleteQuiz(id string) error
}

type SessionRepository interface {
	CreateSession(s *models.Session) error
	GetSession(code string) (*models.Session, error)
	UpdateSession(s *models.Session) error
	ListActiveSessions() []*models.Session
	DeleteSession(code string) error
}
