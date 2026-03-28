package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/quizforge/quiz-forge/internal/models"
	"github.com/quizforge/quiz-forge/internal/repository"
)

type QuizService struct {
	repo repository.QuizRepository
}

func NewQuizService(repo repository.QuizRepository) *QuizService {
	return &QuizService{
		repo: repo,
	}
}

func (s *QuizService) CreateQuiz(title, description string) (*models.Quiz, error) {
	if title == "" {
		return nil, fmt.Errorf("quiz title cannot be empty")
	}
	if len(title) > 200 {
		return nil, fmt.Errorf("quiz title too long (max 200 characters)")
	}
	if len(description) > 1000 {
		return nil, fmt.Errorf("quiz description too long (max 1000 characters)")
	}

	quiz := models.NewQuiz(title, description)
	quiz.ID = uuid.New().String()

	if err := s.repo.SaveQuiz(quiz); err != nil {
		return nil, fmt.Errorf("failed to save quiz: %w", err)
	}

	return quiz, nil
}

func (s *QuizService) GetQuiz(id string) (*models.Quiz, error) {
	if id == "" {
		return nil, fmt.Errorf("quiz ID cannot be empty")
	}

	quiz, err := s.repo.GetQuiz(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get quiz: %w", err)
	}

	return quiz, nil
}

func (s *QuizService) ListQuizzes() ([]*models.Quiz, error) {
	quizzes := s.repo.ListQuizzes()
	return quizzes, nil
}

func (s *QuizService) UpdateQuiz(quiz *models.Quiz) error {
	if quiz == nil {
		return fmt.Errorf("quiz cannot be nil")
	}
	if quiz.ID == "" {
		return fmt.Errorf("quiz ID cannot be empty")
	}
	if quiz.Title == "" {
		return fmt.Errorf("quiz title cannot be empty")
	}
	if len(quiz.Title) > 200 {
		return fmt.Errorf("quiz title too long (max 200 characters)")
	}
	if len(quiz.Description) > 1000 {
		return fmt.Errorf("quiz description too long (max 1000 characters)")
	}

	// Validate questions
	if len(quiz.Questions) > 50 {
		return fmt.Errorf("too many questions (max 50)")
	}

	for i, question := range quiz.Questions {
		if err := s.validateQuestion(&question, i); err != nil {
			return err
		}
	}

	if err := s.repo.SaveQuiz(quiz); err != nil {
		return fmt.Errorf("failed to update quiz: %w", err)
	}

	return nil
}

func (s *QuizService) DeleteQuiz(id string) error {
	if id == "" {
		return fmt.Errorf("quiz ID cannot be empty")
	}

	if err := s.repo.DeleteQuiz(id); err != nil {
		return fmt.Errorf("failed to delete quiz: %w", err)
	}

	return nil
}

func (s *QuizService) AddQuestion(quizID string, question *models.Question) error {
	if quizID == "" {
		return fmt.Errorf("quiz ID cannot be empty")
	}
	if question == nil {
		return fmt.Errorf("question cannot be nil")
	}

	quiz, err := s.GetQuiz(quizID)
	if err != nil {
		return err
	}

	if len(quiz.Questions) >= 50 {
		return fmt.Errorf("quiz already has maximum number of questions (50)")
	}

	if err := s.validateQuestion(question, len(quiz.Questions)); err != nil {
		return err
	}

	question.ID = uuid.New().String()
	quiz.Questions = append(quiz.Questions, *question)

	return s.UpdateQuiz(quiz)
}

func (s *QuizService) UpdateQuestion(quizID string, questionIndex int, question *models.Question) error {
	if quizID == "" {
		return fmt.Errorf("quiz ID cannot be empty")
	}
	if question == nil {
		return fmt.Errorf("question cannot be nil")
	}

	quiz, err := s.GetQuiz(quizID)
	if err != nil {
		return err
	}

	if questionIndex < 0 || questionIndex >= len(quiz.Questions) {
		return fmt.Errorf("invalid question index")
	}

	if err := s.validateQuestion(question, questionIndex); err != nil {
		return err
	}

	question.ID = quiz.Questions[questionIndex].ID
	quiz.Questions[questionIndex] = *question

	return s.UpdateQuiz(quiz)
}

func (s *QuizService) DeleteQuestion(quizID string, questionIndex int) error {
	if quizID == "" {
		return fmt.Errorf("quiz ID cannot be empty")
	}

	quiz, err := s.GetQuiz(quizID)
	if err != nil {
		return err
	}

	if questionIndex < 0 || questionIndex >= len(quiz.Questions) {
		return fmt.Errorf("invalid question index")
	}

	if len(quiz.Questions) <= 1 {
		return fmt.Errorf("quiz must have at least one question")
	}

	quiz.Questions = append(quiz.Questions[:questionIndex], quiz.Questions[questionIndex+1:]...)

	return s.UpdateQuiz(quiz)
}

func (s *QuizService) validateQuestion(question *models.Question, index int) error {
	if question.Text == "" {
		return fmt.Errorf("question %d text cannot be empty", index+1)
	}
	if len(question.Text) > 500 {
		return fmt.Errorf("question %d text too long (max 500 characters)", index+1)
	}

	if len(question.Answers) < 2 || len(question.Answers) > 6 {
		return fmt.Errorf("question %d must have 2-6 answer options", index+1)
	}

	for i, answer := range question.Answers {
		if answer == "" {
			return fmt.Errorf("question %d answer %d cannot be empty", index+1, i+1)
		}
		if len(answer) > 200 {
			return fmt.Errorf("question %d answer %d too long (max 200 characters)", index+1, i+1)
		}
	}

	if question.CorrectIndex < 0 || question.CorrectIndex >= len(question.Answers) {
		return fmt.Errorf("question %d invalid correct answer index", index+1)
	}

	return nil
}
