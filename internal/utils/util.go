package utils

import (
	"net/http"
	"time"
)

func SetCookie(w http.ResponseWriter, tokenString string, cookieName string, expirationTime time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tokenString,
		Expires:  expirationTime,
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	})
}
