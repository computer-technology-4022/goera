package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
)

type APIClient struct {
	Client *http.Client
}

var (
	instance *APIClient
	once     sync.Once
)

func GetAPIClient() *APIClient {
	once.Do(func() {
		instance = &APIClient{
			Client: &http.Client{},
		}
	})
	return instance
}

func NewAPIClient() *APIClient {
	return &APIClient{
		Client: &http.Client{},
	}
}

func (a *APIClient) SendRequest(originalRequest *http.Request, path string, method string, body io.Reader, result interface{}) error {
	scheme := "http"
	if originalRequest.TLS != nil {
		scheme = "https"
	}
	host := originalRequest.Host
	url := fmt.Sprintf("%s://%s%s", scheme, host, path)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return fmt.Errorf("error creating request: %v", err)
	}

	for _, cookie := range originalRequest.Cookies() {
		req.AddCookie(cookie)
	}

	if authHeader := originalRequest.Header.Get("Authorization"); authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		log.Printf("Error making API request: %v", err)
		return fmt.Errorf("error making API request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		log.Printf("API returned non-success status: %d", resp.StatusCode)
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	if result != nil {
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
			return fmt.Errorf("error reading response body: %v", err)
		}

		if err := json.Unmarshal(respBody, result); err != nil {
			log.Printf("Error parsing API response: %v", err)
			return fmt.Errorf("error parsing API response: %v", err)
		}
	}

	return nil
}

// Get sends a GET request to the API
func (a *APIClient) Get(originalRequest *http.Request, path string, result interface{}) error {
	return a.SendRequest(originalRequest, path, http.MethodGet, nil, result)
}

// Post sends a POST request to the API
func (a *APIClient) Post(originalRequest *http.Request, path string, body io.Reader, result interface{}) error {
	return a.SendRequest(originalRequest, path, http.MethodPost, body, result)
}

// Put sends a PUT request to the API
func (a *APIClient) Put(originalRequest *http.Request, path string, body io.Reader, result interface{}) error {
	return a.SendRequest(originalRequest, path, http.MethodPut, body, result)
}

// Delete sends a DELETE request to the API
func (a *APIClient) Delete(originalRequest *http.Request, path string) error {
	return a.SendRequest(originalRequest, path, http.MethodDelete, nil, nil)
}
