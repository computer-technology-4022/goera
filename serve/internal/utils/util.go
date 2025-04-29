package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

func IsJSONRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return contentType == "application/json" || contentType == "application/json; charset=UTF-8"
}

func IsFormRequest(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return contentType == "application/x-www-form-urlencoded" ||
		strings.HasPrefix(contentType, "multipart/form-data")
}

func ProcessRequestData(r *http.Request, jsonTarget interface{}, formProcessor func(*http.Request) (interface{}, error)) (interface{}, error) {
	if IsJSONRequest(r) {
		if err := json.NewDecoder(r.Body).Decode(jsonTarget); err != nil {
			return nil, err
		}
		return jsonTarget, nil
	} else if IsFormRequest(r) {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}

		return formProcessor(r)
	}

	return nil, fmt.Errorf("unsupported content type: %s", r.Header.Get("Content-Type"))
}

func GetContentType(r *http.Request) string {
	if IsJSONRequest(r) {
		return "json"
	} else if IsFormRequest(r) {
		return "form"
	}
	return "unknown"
}
