package handler

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/computer-technology-4022/goera/internal/api"
	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/utils"
	"github.com/gorilla/mux"
)

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	_, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		log.Printf("Invalid profile user ID format in handler: %v", err)
		http.Error(w, "Invalid User ID", http.StatusBadRequest)
		return
	}

	_, viewerExists := auth.UserIDFromContext(r.Context())
	if !viewerExists {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	apiPath := fmt.Sprintf("/api/profile/%s", idStr)
	apiClient := utils.GetAPIClient()
	var profileData api.ProfileAPIResponse
	err = apiClient.Get(r, apiPath, &profileData)
	if err != nil {
		log.Printf("Error received from profile API call: %v", err)
		http.Error(w, "Failed to fetch profile data", http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFiles("web/templates/profile.html")
	if err != nil {
		log.Printf("Error parsing profile template: %v", err)
		http.Error(w, "Internal Server Error (Template Parse)", http.StatusInternalServerError)
		return
	}

	err = tmpl.ExecuteTemplate(w, "profile.html", profileData)
	if err != nil {
		log.Printf("Error executing profile template: %v", err)
	}
}
