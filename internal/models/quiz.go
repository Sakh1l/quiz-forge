package models

import "time"

type Quiz struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Questions   []Question `json:"questions"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Question struct {
	ID           string   `json:"id"`
	Text         string   `json:"text"`
	Answers      []string `json:"answers"`
	CorrectIndex int      `json:"correct_index"`
	ImageURL     string   `json:"image_url,omitempty"`
}

func NewQuiz(title, description string) *Quiz {
	return &Quiz{
		ID:          "",
		Title:       title,
		Description: description,
		Questions:   []Question{},
		CreatedAt:   time.Now(),
	}
}
