package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Result string

const (
	Accepted     Result = "Accepted"
	CompileError Result = "CompileError"
	WrongAnswer  Result = "WrongAnswer"
	MemoryLimit  Result = "MemoryLimit"
	TimeLimit    Result = "TimeLimit"
	RuntimeError Result = "RuntimeError"
)

type RunResponse struct {
	SubmissionID uint   `json:"submissionId"`
	Status       Result `json:"status"`
	Output       string `json:"output"`
}

type TestCase struct {
	Input          string `json:"input"`
	ExpectedOutput string `json:"expectedOutput"`
}

type PendingSubmission struct {
	SubmissionID uint       `json:"submissionId"`
	SourceCode   string     `json:"sourceCode"`
	TestCases    []TestCase `json:"testCases"`
	TimeLimit    string     `json:"timeLimit"`
	MemoryLimit  string     `json:"memoryLimit"`
	CPUCount     string     `json:"cpuCount"`
	DockerImage  string     `json:"dockerImage"`
}

var (
	queue []*PendingSubmission
	mu    sync.Mutex
	busy  bool
)

func main() {
	http.HandleFunc("/submit", submitHandler)

	log.Println("Judge service running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	var sub PendingSubmission
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Printf("ID=%v", sub.SubmissionID)

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

func runnerDoneHandler() {
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
}

func processSubmission(sub *PendingSubmission) {
	result, err := sendToCodeRunner(sub)
	if err != nil {
		log.Printf("Error sending to Code-Runner: %v\n", err)
		return
	}
	log.Printf("Code-Runner response: result=%v\n", result.Status)
	runnerDoneHandler()

	apiURL := fmt.Sprintf("http://localhost:5000/internalapi/judge/%d", sub.SubmissionID)

	requestBody, err := json.Marshal(result)
	if err != nil {
		log.Printf("Error marshaling result: %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("INTERNAL_API_KEY")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to internal API: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Internal API returned non-OK status: %d, body: %s\n", resp.StatusCode, string(body))
		return
	}

	log.Println("Successfully sent result to internal API")

}

func sendToCodeRunner(sub *PendingSubmission) (*RunResponse, error) {
	payload, err := json.Marshal(sub)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal submission: %w", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:8081/run", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("INTERNAL_API_KEY")
	req.Header.Set("X-API-Key", apiKey)

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
