package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/quizforge/quiz-forge/internal/models"
	"github.com/quizforge/quiz-forge/internal/repository"
)

type SessionService struct {
	quizRepo    repository.QuizRepository
	sessionRepo repository.SessionRepository
}

func NewSessionService(quizRepo repository.QuizRepository, sessionRepo repository.SessionRepository) *SessionService {
	return &SessionService{
		quizRepo:    quizRepo,
		sessionRepo: sessionRepo,
	}
}

func (s *SessionService) CreateSession(quizID string, timerSeconds int, shuffleQuestions bool) (*models.Session, error) {
	if quizID == "" {
		return nil, fmt.Errorf("quiz ID cannot be empty")
	}

	quiz, err := s.quizRepo.GetQuiz(quizID)
	if err != nil {
		return nil, fmt.Errorf("failed to get quiz: %w", err)
	}

	if len(quiz.Questions) == 0 {
		return nil, fmt.Errorf("quiz has no questions")
	}

	if timerSeconds < 0 {
		return nil, fmt.Errorf("timer seconds cannot be negative")
	}

	roomCode, err := s.generateRoomCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate room code: %w", err)
	}

	hostToken := uuid.New().String()

	session := models.NewSession(quizID, roomCode, hostToken, timerSeconds, shuffleQuestions)
	session.ID = uuid.New().String()

	// Generate question order
	questionOrder := make([]int, len(quiz.Questions))
	for i := range questionOrder {
		questionOrder[i] = i
	}

	if shuffleQuestions {
		s.shuffleSlice(questionOrder)
	}

	session.QuestionOrder = questionOrder

	if err := s.sessionRepo.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

func (s *SessionService) GetSession(code string) (*models.Session, error) {
	if code == "" {
		return nil, fmt.Errorf("room code cannot be empty")
	}

	session, err := s.sessionRepo.GetSession(code)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return session, nil
}

func (s *SessionService) JoinSession(code, nickname string) (*models.Session, *models.Player, error) {
	if code == "" {
		return nil, nil, fmt.Errorf("room code cannot be empty")
	}
	if nickname == "" {
		return nil, nil, fmt.Errorf("nickname cannot be empty")
	}
	if len(nickname) > 30 {
		return nil, nil, fmt.Errorf("nickname too long (max 30 characters)")
	}

	session, err := s.GetSession(code)
	if err != nil {
		return nil, nil, err
	}

	if session.Status != models.StatusWaiting && session.Status != models.StatusActive {
		return nil, nil, fmt.Errorf("cannot join session in %s status", session.Status)
	}

	// Check if nickname already exists
	for _, player := range session.Players {
		if player.Nickname == nickname {
			return nil, nil, fmt.Errorf("nickname already taken")
		}
	}

	playerID := uuid.New().String()
	player := models.NewPlayer(playerID, nickname)

	session.Players[playerID] = player

	if err := s.sessionRepo.UpdateSession(session); err != nil {
		return nil, nil, fmt.Errorf("failed to update session: %w", err)
	}

	return session, player, nil
}

func (s *SessionService) StartSession(code string) error {
	session, err := s.GetSession(code)
	if err != nil {
		return err
	}

	if session.Status != models.StatusWaiting {
		return fmt.Errorf("session already started")
	}

	if len(session.Players) == 0 {
		return fmt.Errorf("no players in session")
	}

	session.Status = models.StatusActive
	session.CurrentIndex = 0
	now := time.Now()
	session.StartedAt = &now

	return s.sessionRepo.UpdateSession(session)
}

func (s *SessionService) NextQuestion(code string) error {
	session, err := s.GetSession(code)
	if err != nil {
		return err
	}

	if session.Status != models.StatusActive && session.Status != models.StatusReveal {
		return fmt.Errorf("session not active")
	}

	if session.CurrentIndex >= len(session.QuestionOrder)-1 {
		return fmt.Errorf("no more questions")
	}

	session.CurrentIndex++
	session.Status = models.StatusActive

	// Clear answers for new question
	for range session.Players {
		for answerID, answer := range session.Answers {
			if answer.QuestionIndex == session.CurrentIndex {
				delete(session.Answers, answerID)
			}
		}
	}

	return s.sessionRepo.UpdateSession(session)
}

func (s *SessionService) SubmitAnswer(code, playerID string, questionIndex, selectedIndex int) error {
	session, err := s.GetSession(code)
	if err != nil {
		return err
	}

	if session.Status != models.StatusActive {
		return fmt.Errorf("not accepting answers")
	}

	if questionIndex != session.CurrentIndex {
		return fmt.Errorf("not current question")
	}

	player, exists := session.Players[playerID]
	if !exists {
		return fmt.Errorf("player not found")
	}

	if !player.Connected {
		return fmt.Errorf("player not connected")
	}

	// Check if already answered
	for _, answer := range session.Answers {
		if answer.PlayerID == playerID && answer.QuestionIndex == questionIndex {
			return fmt.Errorf("answer already submitted")
		}
	}

	// Calculate response time
	var responseTime int64 = 0
	if session.StartedAt != nil {
		responseTime = time.Since(*session.StartedAt).Milliseconds()
	}

	answer := &models.Answer{
		PlayerID:      playerID,
		QuestionIndex: questionIndex,
		SelectedIndex: selectedIndex,
		ResponseTime:  responseTime,
		SubmittedAt:   time.Now(),
	}

	session.Answers[playerID+fmt.Sprintf("-%d", questionIndex)] = answer

	return s.sessionRepo.UpdateSession(session)
}

func (s *SessionService) RevealAnswer(code string) error {
	session, err := s.GetSession(code)
	if err != nil {
		return err
	}

	if session.Status != models.StatusActive {
		return fmt.Errorf("no active question to reveal")
	}

	session.Status = models.StatusReveal

	// Calculate scores
	s.calculateScores(session)

	return s.sessionRepo.UpdateSession(session)
}

func (s *SessionService) EndSession(code string) error {
	session, err := s.GetSession(code)
	if err != nil {
		return err
	}

	if session.Status == models.StatusEnded {
		return fmt.Errorf("session already ended")
	}

	session.Status = models.StatusEnded
	now := time.Now()
	session.EndedAt = &now

	return s.sessionRepo.UpdateSession(session)
}

func (s *SessionService) GetLeaderboard(code string) ([]*models.Player, error) {
	session, err := s.GetSession(code)
	if err != nil {
		return nil, err
	}

	players := make([]*models.Player, 0, len(session.Players))
	for _, player := range session.Players {
		players = append(players, player)
	}

	// Sort by score (descending), then by average response time (ascending)
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			if players[j].Score > players[i].Score {
				players[i], players[j] = players[j], players[i]
			} else if players[j].Score == players[i].Score {
				avgTimeI := float64(players[i].TotalTime) / float64(len(players[i].Answers))
				avgTimeJ := float64(players[j].TotalTime) / float64(len(players[j].Answers))
				if avgTimeJ < avgTimeI {
					players[i], players[j] = players[j], players[i]
				}
			}
		}
	}

	return players, nil
}

func (s *SessionService) calculateScores(session *models.Session) {
	if session.CurrentIndex < 0 || session.CurrentIndex >= len(session.QuestionOrder) {
		return
	}

	questionIndex := session.QuestionOrder[session.CurrentIndex]

	quiz, err := s.quizRepo.GetQuiz(session.QuizID)
	if err != nil {
		return
	}

	if questionIndex >= len(quiz.Questions) {
		return
	}

	question := quiz.Questions[questionIndex]

	for _, answer := range session.Answers {
		if answer.QuestionIndex == session.CurrentIndex {
			if player, exists := session.Players[answer.PlayerID]; exists {
				if answer.SelectedIndex == question.CorrectIndex {
					player.Score++
					player.Answers[session.CurrentIndex] = true
				} else {
					player.Answers[session.CurrentIndex] = false
				}
				player.TotalTime += answer.ResponseTime
			}
		}
	}
}

func (s *SessionService) generateRoomCode() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 6

	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}

	return string(b), nil
}

func (s *SessionService) shuffleSlice(slice []int) {
	for i := range slice {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(len(slice))))
		if err != nil {
			return
		}
		slice[i], slice[j.Int64()] = slice[j.Int64()], slice[i]
	}
}
