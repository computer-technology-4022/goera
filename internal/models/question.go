package models

import (
	"time"

	"gorm.io/gorm"
)

type Question struct {
	gorm.Model
	Title       string       `json:"title"`       // Question title
	Content     string       `json:"content"`     // Question content/description
	Published   bool         `json:"published"`   // Whether the question is published
	PublishedBy *uint        `json:"publishedBy"` // ID of the admin who published the question (null if not published)
	PublishedAt *time.Time   `json:"publishedAt"` // Date when the question was published
	UserID      uint         `json:"userId"`      // ID of the user who created the question
	User        User         `json:"-" gorm:"foreignKey:UserID"`
	Submissions []Submission `json:"-" gorm:"foreignKey:QuestionID"`
	Difficulty  string       `json:"difficulty"`  // Difficulty level
	Tags        string       `json:"tags"`        // Question tags
	TimeLimit   int          `json:"timeLimit"`   // Time limit (in milliseconds)
	MemoryLimit int          `json:"memoryLimit"` // Memory limit (in megabytes)
}

func MigrateQuestion(db *gorm.DB) error {
	err := db.AutoMigrate(&Question{})
	if err != nil {
		return err
	}
	return nil
}
