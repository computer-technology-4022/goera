package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/computer-technology-4022/goera/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

type Claims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

func SetCookie(w http.ResponseWriter, tokenString string,
	cookieName string, expirationTime time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tokenString,
		Expires:  expirationTime,
		HttpOnly: true,
		Secure:   false,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	})
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func GenerateJWT(userID uint) (string, error) {
	expirationTime := time.Now().Add(168 * time.Hour)
	claims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "your-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userID uint
		var hasValidToken bool

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

		path := r.URL.Path

		if isProtected(path, config.ProtectedPrefixes) && !hasValidToken {
			if strings.HasPrefix(path, "/api") {
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
			ctx := context.WithValue(r.Context(), "userID", userID)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

func isProtected(path string, protectedPrefixes []string) bool {
	log.Printf("Checking path: %s against prefixes: %v", path, protectedPrefixes)
	for _, prefix := range protectedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
