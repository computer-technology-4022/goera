package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// JudgeStatus represents the current status of a submission.
type JudgeStatus string

const (
	Pending             JudgeStatus = "pending"
	Judging             JudgeStatus = "judging"
	Accepted            JudgeStatus = "accepted"
	Rejected            JudgeStatus = "rejected"
	TimeLimitExceeded   JudgeStatus = "time_limit_exceeded"
	MemoryLimitExceeded JudgeStatus = "memory_limit_exceeded"
	RuntimeError        JudgeStatus = "runtime_error"
	CompilationError    JudgeStatus = "compilation_error"
)

// Submission mirrors the struct returned by /fetchNext
type Submission struct {
	SubmissionID   string    `json:"submissionId"`
	SourcePath     string    `json:"sourcePath"`
	TestcasesPath  string    `json:"testCasesPath"`
	TimeLimit      string    `json:"timeLimit,omitempty"`
	MemoryLimit    string    `json:"memoryLimit,omitempty"`
	CPUCount       string    `json:"cpuCount,omitempty"`
	DockerImage    string    `json:"dockerImage,omitempty"`
	Status         JudgeStatus `json:"status"`
}

// RunResponse is the JSON returned by CodeRunner API
type RunResponse struct {
	Status string `json:"status"`
}

// fetchNext polls the Judge API for the next submission
func fetchNext() (*Submission, error) {
	req, err := http.NewRequest("POST", "http://localhost:8082/fetchNext", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		// no submissions pending
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetchNext failed: %d %s", resp.StatusCode, string(body))
	}

	var sub Submission
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// sendToCodeRunner uploads source/testcases and returns the run result
func sendToCodeRunner(sub *Submission) (*RunResponse, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// attach source file
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

	// attach testcases file
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

	// optional fields
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

func main() {
	fmt.Println("Judge worker started, polling for submissions...")
	for {
		sub, err := fetchNext()
		if err != nil {
			fmt.Printf("Error fetching next: %v\n", err)
		} else if sub != nil {
			fmt.Printf("Processing submission %s\n", sub.SubmissionID)
			res, err := sendToCodeRunner(sub)
			if err != nil {
				fmt.Printf("Error running submission %s: %v\n", sub.SubmissionID, err)
			} else {
				// Update submission status based on code-runner result
				sub.Status = JudgeStatus(res.Status)
				fmt.Printf("Submission %s status: %s\n", sub.SubmissionID, sub.Status)
			}
		}
		time.Sleep(1 * time.Second)
	}
} 