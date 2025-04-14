package handler

import (
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// ProfileData holds the information needed for the profile template
type ProfileData struct {
	ProfileUser    models.User
	IsViewerAdmin  bool
	TotalAttempted int    // Placeholder - Add logic to calculate these later
	TotalSolved    int    // Placeholder
	SuccessRate    int    // Placeholder
	JoinDate       string // Placeholder for formatted join date
	IsAdmin        bool   // Is the profile user an admin?
	UserID         uint   // User ID of the profile user
	Username       string // Username of the profile user
	CurrentUserID  uint   // Added for dynamic profile link
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	profileUserID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		log.Printf("Invalid profile user ID format: %v", err)
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	if db == nil {
		log.Println("Database connection is nil in ProfileHandler")
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
			log.Printf("Database error fetching profile user: %v", result.Error)
			http.Error(w, "Failed to retrieve user profile", http.StatusInternalServerError)
		}
		return
	}

	// 2. Fetch the currently logged-in user (viewer)
	viewerUserID, viewerExists := auth.UserIDFromContext(r.Context())
	var isViewerAdmin bool
	if viewerExists {
		var viewerUser models.User
		result := db.First(&viewerUser, viewerUserID)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			// Log error but don't necessarily block the profile view
			log.Printf("Database error fetching viewing user: %v", result.Error)
		} else if result.Error == nil {
			isViewerAdmin = (viewerUser.Role == models.AdminRole)
		}
	}

	// 3. Prepare data for the template
	// TODO: Add logic to calculate stats (TotalAttempted, TotalSolved, SuccessRate)
	data := ProfileData{
		ProfileUser:   profileUser,
		IsViewerAdmin: isViewerAdmin,
		IsAdmin:       profileUser.Role == models.AdminRole,
		CurrentUserID: viewerUserID,
		UserID:        profileUser.ID,
		Username:      profileUser.Username,
		// Placeholder values - replace with actual calculations later
		TotalAttempted: 0,
		TotalSolved:    0,
		SuccessRate:    0,
		JoinDate:       profileUser.CreatedAt.Format("January 2006"), // Format join date
	}

	// 4. Parse and execute the template
	tmpl, err := template.ParseFiles("web/templates/profile.html", "web/templates/base.html") // Include base if needed
	if err != nil {
		log.Printf("Error parsing profile template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "profile.html", data)
	if err != nil {
		log.Printf("Error executing profile template: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
