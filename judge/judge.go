package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"sync"
)

type Submission struct {
	SourcePath    string `json:"sourcePath"`
	TestcasesPath string `json:"testcasesPath"`
	TimeLimit     string `json:"timeLimit"`
	MemoryLimit   string `json:"memoryLimit"`
	CPUCount      string `json:"cpuCount"`
	DockerImage   string `json:"dockerImage"`
}

type RunResponse struct {
	// Define fields based on what code-runner returns
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

var (
	queue []*Submission
	mu    sync.Mutex
	busy  bool
)

func main() {
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/runner-done", runnerDoneHandler)

	log.Println("Judge service running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var sub Submission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if !busy {
		log.Println("Code-Runner is free. Sending submission immediately.")
		go processSubmission(&sub)
		busy = true
	} else {
		log.Println("Code-Runner busy. Queuing submission.")
		queue = append(queue, &sub)
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Submission accepted"))
}

func runnerDoneHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	if len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		log.Println("Sending next submission from queue.")
		go processSubmission(next)
		busy = true
	} else {
		log.Println("No more submissions. Code-Runner now idle.")
		busy = false
	}

	w.WriteHeader(http.StatusOK)
}

func processSubmission(sub *Submission) {
	result, err := sendToCodeRunner(sub)
	if err != nil {
		log.Printf("Error sending to Code-Runner: %v\n", err)
		// Optionally handle retries or failure scenarios here
		return
	}
	log.Printf("Code-Runner response: success=%v, output=%s\n", result.Success, result.Output)
}

func sendToCodeRunner(sub *Submission) (*RunResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Attach source file
	srcFile, err := os.Open(sub.SourcePath)
	if err != nil {
		return nil, err
	}
	defer srcFile.Close()
	partSrc, err := writer.CreateFormFile("source", sub.SourcePath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(partSrc, srcFile); err != nil {
		return nil, err
	}

	// Attach testcases file
	testFile, err := os.Open(sub.TestcasesPath)
	if err != nil {
		return nil, err
	}
	defer testFile.Close()
	partTest, err := writer.CreateFormFile("testcases", sub.TestcasesPath)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(partTest, testFile); err != nil {
		return nil, err
	}

	// Optional fields
	if sub.TimeLimit != "" {
		writer.WriteField("timeLimit", sub.TimeLimit)
	}
	if sub.MemoryLimit != "" {
		writer.WriteField("memoryLimit", sub.MemoryLimit)
	}
	if sub.CPUCount != "" {
		writer.WriteField("cpuCount", sub.CPUCount)
	}
	if sub.DockerImage != "" {
		writer.WriteField("dockerImage", sub.DockerImage)
	}

	writer.Close()

	req, err := http.NewRequest("POST", "http://localhost:8081/run", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("code-runner API error: %d %s", resp.StatusCode, string(body))
	}

	var result RunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}
