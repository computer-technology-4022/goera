package models

import (
	"time"

	"gorm.io/gorm"
)

// JudgeStatus represents the status of a submission
type JudgeStatus string

const (
	Pending             JudgeStatus = "pending"               // Waiting for judgment
	Judging             JudgeStatus = "judging"               // Currently being judged
	Accepted            JudgeStatus = "accepted"              // Accepted
	Rejected            JudgeStatus = "rejected"              // Rejected
	TimeLimitExceeded   JudgeStatus = "time_limit_exceeded"   // Time limit exceeded
	MemoryLimitExceeded JudgeStatus = "memory_limit_exceeded" // Memory limit exceeded
	RuntimeError        JudgeStatus = "runtime_error"         // Runtime error
	CompilationError    JudgeStatus = "compilation_error"     // Compilation error
)

// Submission represents a code submission in the system
type Submission struct {
	gorm.Model
	Code           string      `json:"code"`           // Submitted code
	Language       string      `json:"language"`       // Programming language
	JudgeStatus    JudgeStatus `json:"judgeStatus"`    // Judgment status
	Output         string      `json:"output"`         // Code execution output
	Error          string      `json:"error"`          // Error message if any
	ExecutionTime  int         `json:"executionTime"`  // Execution time (milliseconds)
	MemoryUsage    int         `json:"memoryUsage"`    // Memory usage (megabytes)
	SubmissionTime time.Time   `json:"submissionTime"` // Submission time
	QuestionID     uint        `json:"questionId"`     // Reference to the question
	QuestionName   string      `json:"questionName"`   // Name of the question
	Question       Question    `json:"-" gorm:"foreignKey:QuestionID"`
	UserID         uint        `json:"userId"` // Reference to the user
	User           User        `json:"-" gorm:"foreignKey:UserID"`
}

func MigrateSubmission(db *gorm.DB) error {
	err := db.AutoMigrate(&Submission{})
	if err != nil {
		return err
	}
	return nil
}
