package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// UserPromoteRequest represents the request body for promoting a user to admin
type UserPromoteRequest struct {
	UserID uint `json:"userId"`
}

func UsersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getUserById(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// PromoteUserHandler handles requests to promote a user to admin role
func PromoteUserHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut, http.MethodPost: // Handle POST as well
		promoteUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// promoteUser promotes a regular user to admin role
func promoteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r) // Get path parameters
	userIDStr := vars["id"]
	if userIDStr == "" {
		http.Error(w, "User ID missing in URL path", http.StatusBadRequest)
		return
	}

	// Convert userIDStr to uint
	var targetUserID uint
	if _, err := fmt.Sscan(userIDStr, &targetUserID); err != nil {
		http.Error(w, "Invalid User ID format", http.StatusBadRequest)
		return
	}

	// Get current user ID from context
	adminID, adminExists := auth.UserIDFromContext(r.Context())
	if !adminExists {
		log.Println("User ID not found in context")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	// Verify current user is admin
	var admin models.User
	result := db.First(&admin, adminID)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		return
	}

	if admin.Role != models.AdminRole {
		http.Error(w, "Only administrators can promote users", http.StatusForbidden)
		return
	}

	// Get the user to promote
	var user models.User
	result = db.First(&user, targetUserID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			log.Printf("Database error: %v", result.Error)
			http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		}
		return
	}

	// Update user role
	user.Role = models.AdminRole
	result = db.Save(&user)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	log.Printf("User ID %d promoted to admin successfully.", targetUserID)

	// Redirect back to the profile page instead of returning JSON
	http.Redirect(w, r, fmt.Sprintf("/profile/%d", targetUserID), http.StatusSeeOther)
}

func getAllUsers(w http.ResponseWriter, r *http.Request) {
	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	var users []models.User

	result := db.Find(&users)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		http.Error(w, "Failed to retrieve users", http.StatusInternalServerError)
		return
	}

	if len(users) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]models.User{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(users); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func getUserById(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r) // Get path parameters
	id := vars["id"]    // Get ID from path
	if id == "" {
		http.Error(w, "User ID missing in URL path", http.StatusBadRequest)
		return
	}
	db := database.GetDB()
	var user models.User
	result := db.First(&user, id)
	if result.Error != nil {
		log.Printf("Database error: %v", result.Error)
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to retrieve user", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
