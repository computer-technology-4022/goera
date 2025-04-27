package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
)

// RunRequest holds the configuration for a judge run.
type RunRequest struct {
	CodePath      string  `json:"codePath"`
	TestCasesPath string  `json:"testCasesPath"`
	TimeLimit     string  `json:"timeLimit,omitempty"`
	MemoryLimit   uint64  `json:"memoryLimit,omitempty"`
	CPUCount      float64 `json:"cpuCount,omitempty"`
	DockerImage   string  `json:"dockerImage,omitempty"`
}

// RunResponse returns the output from the code-runner CLI.
type RunResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Error  string `json:"error,omitempty"`
}

// runHandler handles POST /run and calls the code-runner CLI.
func runHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form to get uploaded files and parameters
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}
	// Handle Go source file upload
	srcFile, _, err := r.FormFile("source")
	if err != nil {
		http.Error(w, "Missing source file", http.StatusBadRequest)
		return
	}
	defer srcFile.Close()
	tmpSrc, err := os.CreateTemp("", "source-*.go")
	if err != nil {
		http.Error(w, "Failed to create temp file for source", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpSrc.Name())
	defer tmpSrc.Close()
	if _, err := io.Copy(tmpSrc, srcFile); err != nil {
		http.Error(w, "Failed to save source file", http.StatusInternalServerError)
		return
	}
	// Handle testcases JSON upload
	testFile, _, err := r.FormFile("testcases")
	if err != nil {
		http.Error(w, "Missing testcases file", http.StatusBadRequest)
		return
	}
	defer testFile.Close()
	tmpTest, err := os.CreateTemp("", "testcases-*.json")
	if err != nil {
		http.Error(w, "Failed to create temp file for test cases", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpTest.Name())
	defer tmpTest.Close()
	if _, err := io.Copy(tmpTest, testFile); err != nil {
		http.Error(w, "Failed to save testcases file", http.StatusInternalServerError)
		return
	}
	// Read optional form values
	timeLimit := r.FormValue("timeLimit")
	memoryLimit := r.FormValue("memoryLimit")
	cpuCount := r.FormValue("cpuCount")
	dockerImage := r.FormValue("dockerImage")

	// Prepare CLI arguments using temp files and form values
	args := []string{
		fmt.Sprintf("--codePath=%s", tmpSrc.Name()),
		fmt.Sprintf("--testCasesPath=%s", tmpTest.Name()),
	}
	if timeLimit != "" {
		args = append(args, "--timeLimit="+timeLimit)
	}
	if memoryLimit != "" {
		args = append(args, "--memoryLimit="+memoryLimit)
	}
	if cpuCount != "" {
		args = append(args, "--cpuCount="+cpuCount)
	}
	if dockerImage != "" {
		args = append(args, "--dockerImage="+dockerImage)
	}

	// Determine absolute path to the code-runner binary
	exeName := "code-runner"
	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	// Use fixed absolute path to the code-runner binary
	codeRunnerPath := "/mnt/c/Users/ASUS/Desktop/goera/code-runner/" + exeName

	cmd := exec.Command(codeRunnerPath, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	// Stream code-runner logs to both buffer (for HTTP response) and server console
	cmd.Stdout = io.MultiWriter(&stdoutBuf, os.Stdout)
	cmd.Stderr = io.MultiWriter(&stderrBuf, os.Stderr)

	err = cmd.Run()

	resp := RunResponse{
		Stdout: stdoutBuf.String(),
		Stderr: stderrBuf.String(),
	}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/run", runHandler)
	addr := ":8081" // separate from internal APIs
	fmt.Printf("CodeRunner API listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
