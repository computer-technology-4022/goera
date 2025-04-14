package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// ProfileAPIResponse mirrors the structure needed by the profile template
type ProfileAPIResponse struct {
	ProfileUser    models.User `json:"profile_user"`
	IsViewerAdmin  bool        `json:"is_viewer_admin"`
	TotalAttempted int         `json:"total_attempted"` // Placeholder
	TotalSolved    int         `json:"total_solved"`    // Placeholder
	SuccessRate    int         `json:"success_rate"`    // Placeholder
	JoinDate       string      `json:"join_date"`       // Formatted join date
	IsAdmin        bool        `json:"is_admin"`        // Is the profile user an admin?
	UserID         uint        `json:"user_id"`         // User ID of the profile user
	Username       string      `json:"username"`        // Username of the profile user
	CurrentUserID  uint        `json:"current_user_id"` // ID of the user viewing the profile
}

// GetProfileByID handles GET requests to /api/profile/{id}
func GetProfileByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	profileUserID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		log.Printf("Invalid profile user ID format in API: %v", err)
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil in GetProfileByID API")
		http.Error(w, "Database connection error", http.StatusInternalServerError)
		return
	}

	// 1. Fetch the user whose profile is being viewed
	var profileUser models.User
	result := db.First(&profileUser, uint(profileUserID))
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.NotFound(w, r)
		} else {
			log.Printf("API DB error fetching profile user: %v", result.Error)
			http.Error(w, "Failed to retrieve user profile", http.StatusInternalServerError)
		}
		return
	}

	// 2. Fetch the currently logged-in user (viewer)
	viewerUserID, viewerExists := auth.UserIDFromContext(r.Context())
	if !viewerExists {
		// While the middleware should catch this, double-check
		http.Error(w, "Unauthorized: Viewer context missing", http.StatusUnauthorized)
		return
	}

	var isViewerAdmin bool
	var viewerUser models.User
	result = db.First(&viewerUser, viewerUserID)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		// Log error but don't necessarily block the profile view
		log.Printf("API DB error fetching viewing user: %v", result.Error)
	} else if result.Error == nil {
		isViewerAdmin = (viewerUser.Role == models.AdminRole)
	}

	// 3. Prepare data for the response
	// TODO: Add logic to calculate stats (TotalAttempted, TotalSolved, SuccessRate)
	data := ProfileAPIResponse{
		ProfileUser:    profileUser,
		IsViewerAdmin:  isViewerAdmin,
		IsAdmin:        profileUser.Role == models.AdminRole,
		CurrentUserID:  viewerUserID,
		UserID:         profileUser.ID,
		Username:       profileUser.Username,
		TotalAttempted: 0,                                            // Placeholder
		TotalSolved:    0,                                            // Placeholder
		SuccessRate:    0,                                            // Placeholder
		JoinDate:       profileUser.CreatedAt.Format("January 2006"), // Format join date
	}

	// 4. Send the response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("API JSON encoding error for profile: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
