package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	utils "github.com/computer-technology-4022/goera/internal/utils"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Methode not allowed", http.StatusMethodNotAllowed)
		return
	}

	var loginData loginRequest
	contentType := r.Header.Get("Content-Type")

	// Handle JSON request
	if contentType == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&loginData); err != nil {
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

		// Create login data from form values
		loginData = loginRequest{
			Username: username,
			Password: password,
		}
	}

	db := database.GetDB()
	var user models.User

	// Check if form or API request
	isFormSubmission := contentType != "application/json"

	if result := db.Where("username = ?", loginData.Username).First(&user); result.Error != nil {
		if isFormSubmission {
			http.Redirect(w, r, "/login?error=invalid_credentials", http.StatusSeeOther)
			return
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPasswordHash(loginData.Password, user.Password) {
		if isFormSubmission {
			http.Redirect(w, r, "/login?error=invalid_credentials", http.StatusSeeOther)
			return
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(168 * time.Hour)
	token, err := auth.GenerateJWT(user.ID)
	if err != nil {
		if isFormSubmission {
			http.Redirect(w, r, "/login?error=server_error", http.StatusSeeOther)
			return
		}
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	utils.SetCookie(w, token, "token", expirationTime)

	user.Password = ""

	// If it was a form submission, redirect to questions page
	if isFormSubmission {
		http.Redirect(w, r, "/questions", http.StatusSeeOther)
		return
	}

	// Otherwise return JSON response for API clients
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
		// "token": token,
	})
}

// func LoginHandler(w http.ResponseWriter, r *http.Request) {
//     // Check for error message
//     errorMsg := ""
//     if r.URL.Query().Get("error") == "unauthorized" {
//         errorMsg = "Please login to access that page"
//     }

//     // Check for redirect URL
//     redirectURL := "/" // Default redirect after login
//     if cookie, err := r.Cookie("redirect_url"); err == nil {
//         redirectURL = cookie.Value
//     }

//     // Your existing login logic here
//     // When login is successful, redirect to the original URL:
//     http.SetCookie(w, &http.Cookie{
//         Name:   "redirect_url",
//         Value:  "",
//         Path:   "/",
//         MaxAge: -1, // Delete the cookie
//     })
//     http.Redirect(w, r, redirectURL, http.StatusFound)
// }
