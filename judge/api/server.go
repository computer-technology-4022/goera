package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// Submission holds the metadata and file paths for a code submission.
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

var (
	queue []*Submission
	mu    sync.Mutex
)

// submitHandler accepts a multipart-form POST with fields: submissionId, source, testcases, and optional flags.
func submitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// Parse up to 32 MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Required fields
	submissionID := r.FormValue("submissionId")
	if submissionID == "" {
		http.Error(w, "Missing submissionId", http.StatusBadRequest)
		return
	}
	// Source file
	srcFile, _, err := r.FormFile("source")
	if err != nil {
		http.Error(w, "Missing source file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer srcFile.Close()
	// Test cases file
	testFile, _, err := r.FormFile("testcases")
	if err != nil {
		http.Error(w, "Missing testcases file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer testFile.Close()

	// Create a unique upload directory in the OS temp
	uploadDir := filepath.Join(os.TempDir(), "judge", submissionID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		http.Error(w, "Failed to create upload dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Save source
	srcPath := filepath.Join(uploadDir, "source.go")
	dst, err := os.Create(srcPath)
	if err != nil {
		http.Error(w, "Failed to write source: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, srcFile); err != nil {
		dst.Close()
		http.Error(w, "Failed to save source: "+err.Error(), http.StatusInternalServerError)
		return
	}
	dst.Close()
	// Save test cases
	testPath := filepath.Join(uploadDir, "testcases.json")
	dst2, err := os.Create(testPath)
	if err != nil {
		http.Error(w, "Failed to write testcases: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst2, testFile); err != nil {
		dst2.Close()
		http.Error(w, "Failed to save testcases: "+err.Error(), http.StatusInternalServerError)
		return
	}
	dst2.Close()

	// Build submission record
	sub := &Submission{
		SubmissionID:  submissionID,
		SourcePath:    srcPath,
		TestcasesPath: testPath,
		TimeLimit:     r.FormValue("timeLimit"),
		MemoryLimit:   r.FormValue("memoryLimit"),
		CPUCount:      r.FormValue("cpuCount"),
		DockerImage:   r.FormValue("dockerImage"),
		Status:        Pending,
	}
	// Enqueue
	mu.Lock()
	queue = append(queue, sub)
	mu.Unlock()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"message": "queued", "submissionId": submissionID})
}

// fetchNextHandler returns the next pending submission as JSON and dequeues it.
func fetchNextHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if len(queue) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sub := queue[0]
	queue = queue[1:]
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sub)
}

func main() {
	http.HandleFunc("/submit", submitHandler)
	http.HandleFunc("/fetchNext", fetchNextHandler)
	addr := ":8082"
	fmt.Printf("Judge API listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
} 