package api

import (
	"encoding/json"
	"fmt"
	"goera/serve/internal/auth"
	"goera/serve/internal/database"
	"goera/serve/internal/models"
	"net/http"
	"time"

	"goera/serve/internal/utils"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var loginData loginRequest

	// Process form data using our utility function
	formProcessor := func(r *http.Request) (interface{}, error) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == "" || password == "" {
			return nil, fmt.Errorf("username and password are required")
		}

		return loginRequest{
			Username: username,
			Password: password,
		}, nil
	}

	result, err := utils.ProcessRequestData(r, &loginData, formProcessor)
	if err != nil {
		if utils.IsFormRequest(r) {
			http.Redirect(w, r, "/login?error=invalid_form", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// If the result came from form processing, we need to update loginData
	if formData, ok := result.(loginRequest); ok {
		loginData = formData
	}

	db := database.GetDB()
	var user models.User

	if result := db.Where("username = ?", loginData.Username).First(&user); result.Error != nil {
		if utils.IsFormRequest(r) {
			http.Redirect(w, r, "/login?error=invalid_credentials", http.StatusSeeOther)
			return
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPasswordHash(loginData.Password, user.Password) {
		if utils.IsFormRequest(r) {
			http.Redirect(w, r, "/login?error=invalid_credentials", http.StatusSeeOther)
			return
		}
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(168 * time.Hour)
	token, err := auth.GenerateJWT(user.ID)
	if err != nil {
		if utils.IsFormRequest(r) {
			http.Redirect(w, r, "/login?error=server_error", http.StatusSeeOther)
			return
		}
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	utils.SetCookie(w, token, "token", expirationTime)

	user.Password = ""

	// Respond based on request type
	if utils.IsFormRequest(r) {
		http.Redirect(w, r, "/questions", http.StatusSeeOther)
		return
	}

	// Return JSON response for API clients
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"user": user,
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
