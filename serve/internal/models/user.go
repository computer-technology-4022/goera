package models

import "gorm.io/gorm"

// UserRole represents the role type of a user
type UserRole string

const (
	AdminRole   UserRole = "ADMIN" // Administrator role
	RegularRole UserRole = "USER"  // Regular user role
)

// User represents a user in the system
type User struct {
	gorm.Model
	Username string   `json:"username"` // User's username
	Password string   `json:"password"` // User's password (hashed)
	Role     UserRole `json:"role"`     // User's role (ADMIN or USER)
}

func MigrateUser(db *gorm.DB) error {
	err := db.AutoMigrate(&User{})
	if err != nil {
		return err
	}
	db.Model(&User{}).Where("role = ''").Update("role", RegularRole)
	return nil
}
