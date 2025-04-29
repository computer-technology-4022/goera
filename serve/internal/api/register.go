package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"goera/serve/internal/auth"
	"goera/serve/internal/database"
	"goera/serve/internal/models"
	"goera/serve/internal/utils"
)

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Processing registration request")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user models.User

	// Process form data using our utility function
	formProcessor := func(r *http.Request) (interface{}, error) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "" || password == "" {
			return nil, fmt.Errorf("username and password are required")
		}

		return models.User{
			Username: username,
			Password: password,
		}, nil
	}

	result, err := utils.ProcessRequestData(r, &user, formProcessor)
	if err != nil {
		if utils.IsFormRequest(r) {
			if err.Error() == "username and password are required" {
				http.Redirect(w, r, "/signUp?error=missing_fields", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/signUp?error=invalid_form", http.StatusSeeOther)
			}
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If the result came from form processing, we need to update user
	if formData, ok := result.(models.User); ok {
		user = formData
	}

	hashedPassword, err := auth.HashPassword(user.Password)
	if err != nil {
		if utils.IsFormRequest(r) {
			http.Redirect(w, r, "/signUp?error=server_error", http.StatusSeeOther)
			return
		}
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	user.Password = hashedPassword
	user.Role = models.RegularRole

	db := database.GetDB()
	if result := db.Create(&user); result.Error != nil {
		if utils.IsFormRequest(r) {
			// Most likely username already exists
			http.Redirect(w, r, "/signUp?error=user_exists", http.StatusSeeOther)
			return
		}
		http.Error(w, result.Error.Error(), http.StatusInternalServerError)
		return
	}

	token, err := auth.GenerateJWT(user.ID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	expirationTime := time.Now().Add(168 * time.Hour)
	utils.SetCookie(w, token, "token", expirationTime)

	user.Password = ""

	if utils.IsFormRequest(r) {
		http.Redirect(w, r, "/questions", http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
	})
}
