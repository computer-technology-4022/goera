package auth

import (
	"context"
	"goera/serve/internal/config"
	"net/http"
	"strings"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID uint
		var hasValidToken bool

		path := r.URL.Path
		isApiReq := strings.HasPrefix(path, "/api")

		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := authHeader[len("Bearer "):]
			claims, err := ValidateJWT(tokenString)
			if err == nil {
				userID = claims.UserID
				hasValidToken = true
			}
		}

		if !hasValidToken {
			cookie, err := r.Cookie("token")
			if err == nil {
				claims, err := ValidateJWT(cookie.Value)
				if err == nil {
					userID = claims.UserID
					hasValidToken = true
				}
			}
		}

		if isProtected(path, config.ProtectedPrefixes) && !hasValidToken {
			if isApiReq {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// originalURL := r.URL.String()
			// http.SetCookie(w, &http.Cookie{
			// 	Name:     "redirect_url",
			// 	Value:    originalURL,
			// 	Path:     "/",
			// 	HttpOnly: true,
			// })

			http.Redirect(w, r, "/login?error=unauthorized", http.StatusFound)
			return
		}

		if hasValidToken {
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func isProtected(path string, protectedPrefixes []string) bool {
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
