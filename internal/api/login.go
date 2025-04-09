package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/computer-technology-4022/goera/internal/auth"
	"github.com/computer-technology-4022/goera/internal/database"
	"github.com/computer-technology-4022/goera/internal/models"
	utils "github.com/computer-technology-4022/goera/internal/util"
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

	var loginRequest loginRequest
	if err := json.NewDecoder(r.Body).Decode(&loginRequest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	var user models.User
	if result := db.Where("username = ?", loginRequest.Username).First(&user); result.Error != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !auth.CheckPasswordHash(loginRequest.Password, user.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(168 * time.Hour)
	token, err := auth.GenerateJWT(user.ID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	utils.SetCookie(w, token, "token", expirationTime)

	user.Password = ""
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
