package models

import (
	"time"

	"gorm.io/gorm"
)

// Question represents a question in the system
type Question struct {
	gorm.Model
	Title       string    `json:"title"`       // Question title
	Content     string    `json:"content"`     // Question content
	Published   bool      `json:"published"`   // Publication status
	PublishDate time.Time `json:"publishDate"` // Publication date
	UserID      uint      `json:"userId"`      // Reference to the user who created the question
	User        User      `json:"-" gorm:"foreignKey:UserID"`
	// Submissions defined this way to avoid circular imports
	Submissions []Submission `json:"-" gorm:"foreignKey:QuestionID"`
	// Additional fields
	Difficulty  string `json:"difficulty"`  // Difficulty level
	Tags        string `json:"tags"`        // Question tags
	TimeLimit   int    `json:"timeLimit"`   // Time limit (in milliseconds)
	MemoryLimit int    `json:"memoryLimit"` // Memory limit (in megabytes)
}

func MigrateQuestion(db *gorm.DB) error {
	err := db.AutoMigrate(&Question{})
	if err != nil {
		return err
	}
	return nil
}