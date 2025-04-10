package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	utils "github.com/computer-technology-4022/goera/internal/util"
)

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("asdayhsdbijnfasfasf")
	if r.Method != http.MethodPost {
		http.Error(w, "Methode not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user models.User
	contentType := r.Header.Get("Content-Type")

	// Handle JSON request
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Handle form data
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form data", http.StatusBadRequest)
			return
		}

		// Get form values
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "" || password == "" {
			http.Error(w, "Username and password are required", http.StatusBadRequest)
			return
		}

		// Create user with form values
		user = models.User{
			Username: username,
			Password: password,
		}
	}

	hasedPassword, err := auth.HashPassword(user.Password)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user.Password = hasedPassword
	user.Role = models.RegularRole

	db := database.GetDB()
	if result := db.Create(&user); result.Error != nil {
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

	if contentType != "application/json" {
		http.Redirect(w, r, "/questions", http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
	})
}
