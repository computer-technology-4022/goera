package handler

import (
	"goera/serve/internal/models"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"goera/serve/internal/auth"
	"goera/serve/internal/utils"

	"github.com/gorilla/mux"
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
	// Validate idStr is a number before using it? (Optional, depends on desired robustness)
	_, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		log.Printf("Invalid profile user ID format: %v", err)
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}

	apiClient := utils.GetAPIClient()

	// 1. Fetch the user whose profile is being viewed via API
	var profileUser models.User

	err = apiClient.Get(r, "/api/user/"+idStr, &profileUser)
	if err != nil {
		if err.Error() == "API returned status 404" {
			http.NotFound(w, r)
		} else {
			log.Printf("Error fetching profile user via API: %v", err)
			http.Error(w, "Failed to retrieve user profile", http.StatusInternalServerError)
		}
		return
	}

	// 2. Fetch the currently logged-in user (viewer) via API
	viewerUserID, viewerExists := auth.UserIDFromContext(r.Context())
	var isViewerAdmin bool
	var viewerUser models.User
	if viewerExists {
		// Clone the request to avoid modifying the original
		viewerReq := r.Clone(r.Context())
		viewerReq.Header.Set("userID", strconv.FormatUint(uint64(viewerUserID), 10))
		err = apiClient.Get(viewerReq, "/api/users", &viewerUser)
		if err != nil {
			if err.Error() != "API returned status 404" {
				log.Printf("Error fetching viewing user via API: %v", err)
			}
		} else {
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
