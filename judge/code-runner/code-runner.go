package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Dockerfile content for the judging container
const dockerfileContent = `
FROM golang:1.24-alpine as builder
FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN mkdir /app && chown appuser:appgroup /app
WORKDIR /app
USER appuser
`

// TestCase represents a single test case with input and expected output.
type TestCase struct {
	Input    string `json:"input"`
	Expected string `json:"expectedOutput"`
}

// Result represents the possible outcomes of a test case.
type Result string

const (
	Accepted     Result = "Accepted"
	CompileError Result = "CompileError"
	WrongAnswer  Result = "WrongAnswer"
	MemoryLimit  Result = "MemoryLimit"
	TimeLimit    Result = "TimeLimit"
	RuntimeError Result = "RuntimeError"
)

type JudgeConfig struct {
	TimeLimitPerCase time.Duration
	MemoryLimitMB    uint64
	CPUCount         float64
	DockerImageName  string
	SourceFilePath   string
	TestCases        []TestCase
}

type SubmissionRequest struct {
	QuestionID  uint       `json:"questionId"`
	SourceCode  string     `json:"sourceCode"`
	TestCases   []TestCase `json:"testCases"`
	TimeLimit   string     `json:"timeLimit"`
	MemoryLimit string     `json:"memoryLimit"`
	CPUCount    string     `json:"cpuCount"`
	DockerImage string     `json:"dockerImage"`
}

const DEFAULT_DOCKER_IMAGE = "go-judge-runner:latest"

type RunResponse struct {
	QuestionID uint   `json:"questionId"`
	Status     Result `json:"status"`
	Output     string `json:"output"`
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req SubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Create temporary .go file for source code
	tmpSrc, err := os.CreateTemp("", "source-*.go")
	if err != nil {
		http.Error(w, "Failed to create temp file for source", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpSrc.Name())
	if _, err := tmpSrc.WriteString(req.SourceCode); err != nil {
		http.Error(w, "Failed to write source code", http.StatusInternalServerError)
		return
	}
	tmpSrc.Close()

	// Parse configuration
	timeLimit, err := time.ParseDuration(req.TimeLimit)
	if err != nil && req.TimeLimit != "" {
		http.Error(w, "Invalid timeLimit format", http.StatusBadRequest)
		return
	}
	if req.TimeLimit == "" {
		timeLimit = 2 * time.Second // Default
	}

	var memoryLimit uint64
	if req.MemoryLimit != "" {
		_, err := fmt.Sscanf(req.MemoryLimit, "%d", &memoryLimit)
		if err != nil {
			http.Error(w, "Invalid memoryLimit format", http.StatusBadRequest)
			return
		}
	} else {
		memoryLimit = 64 // Default
	}

	var cpuCount float64
	if req.CPUCount != "" {
		_, err := fmt.Sscanf(req.CPUCount, "%f", &cpuCount)
		if err != nil {
			http.Error(w, "Invalid cpuCount format", http.StatusBadRequest)
			return
		}
	} else {
		cpuCount = 1.0 // Default
	}

	dockerImage := req.DockerImage
	if dockerImage == "" {
		dockerImage = DEFAULT_DOCKER_IMAGE // Default
	}

	// Prepare judge configuration
	config := JudgeConfig{
		TimeLimitPerCase: timeLimit,
		MemoryLimitMB:    memoryLimit,
		CPUCount:         cpuCount,
		DockerImageName:  dockerImage,
		SourceFilePath:   tmpSrc.Name(),
		TestCases:        req.TestCases, // Direct test cases
	}

	// Run the judging logic
	result, output, err := runJudge(config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to run judge: %v", err), http.StatusInternalServerError)
		return
	}

	resp := RunResponse{
		QuestionID: req.QuestionID,
		Status:     result,
		Output:     output,
	}


	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	http.HandleFunc("/run", runHandler)
	addr := ":8081"
	fmt.Printf("CodeRunner service listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func runJudge(config JudgeConfig) (Result, string, error) {
	var outputBuf bytes.Buffer
	logWriter := io.MultiWriter(os.Stdout, &outputBuf)
	fmt.Fprintln(logWriter, "Initialized judge configuration")

	testCases := config.TestCases
	fmt.Fprintf(logWriter, "Loaded %d test cases.\n", len(testCases))
	if len(testCases) == 0 {
		fmt.Fprintln(logWriter, "Warning: No test cases provided.")
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Fprintf(logWriter, "Failed to create Docker client: %v\n", err)
		return RuntimeError, outputBuf.String(), err
	}
	defer apiClient.Close()
	fmt.Fprintln(logWriter, "Initialized Docker client")

	// Build Docker image
	fmt.Fprintf(logWriter, "Building Docker image '%s' from embedded Dockerfile string...\n", config.DockerImageName)
	err = buildDockerImageFromString(apiClient, config)
	if err != nil {
		fmt.Fprintf(logWriter, "Error building Docker image: %v\n", err)
		fmt.Fprintf(logWriter, "Result: %s\n", CompileError)
		return CompileError, outputBuf.String(), err
	}
	fmt.Fprintln(logWriter, "Docker image built successfully.")

	// Compile source code
	executablePath, compileLog, err := compileProgram(config.SourceFilePath)
	if err != nil {
		fmt.Fprintf(logWriter, "Compilation Log:\n%s\n", compileLog)
		return CompileError, outputBuf.String(), err
	}
	defer os.Remove(executablePath)
	fmt.Fprintf(logWriter, "Compilation successful. Host Executable: %s\n", executablePath)

	// Log resource limits
	if config.MemoryLimitMB > 0 {
		fmt.Fprintf(logWriter, "Memory Limit per Test Case: %d MB\n", config.MemoryLimitMB)
	}
	if config.CPUCount > 0 {
		fmt.Fprintf(logWriter, "CPU Limit per Test Case: %.2f cores\n", config.CPUCount)
	}
	fmt.Fprintf(logWriter, "Time Limit per Test Case: %s\n", config.TimeLimitPerCase)

	// Get absolute path for volume mounting
	absExecutablePath, err := filepath.Abs(executablePath)
	if err != nil {
		fmt.Fprintf(logWriter, "Error getting absolute path for executable: %v\n", err)
		return RuntimeError, outputBuf.String(), err
	}
	containerExecutablePath := "/app/program_to_run"

	// Run test cases
	overallResult := Accepted
	if len(testCases) == 0 {
		fmt.Fprintln(logWriter, "No test cases to run.")
		overallResult = Accepted
	} else {
		for i, tc := range testCases {
			fmt.Fprintf(logWriter, "\n--- Running Test Case %d / %d ---\n", i+1, len(testCases))
			fmt.Fprintf(logWriter, "Input:\n%s\n", tc.Input)

			result, output, errMsg := runTestCaseInDocker(
				apiClient,
				absExecutablePath,
				containerExecutablePath,
				tc,
				config,
			)

			fmt.Fprintf(logWriter, "Expected Output:\n%s\n", tc.Expected)
			fmt.Fprintf(logWriter, "Actual Output:\n%s\n", output)
			if errMsg != "" {
				fmt.Fprintf(logWriter, "Error Details:\n%s\n", errMsg)
			}
			fmt.Fprintf(logWriter, "Test Case %d Result: %s\n", i+1, result)

			if result != Accepted {
				overallResult = result
				break
			}
		}
	}

	fmt.Fprintf(logWriter, "\n--- Judge Finished ---\n")
	fmt.Fprintf(logWriter, "Overall Result: %s\n", overallResult)
	return overallResult, outputBuf.String(), nil
}

// loadTestCasesFromFile reads a JSON file and returns a slice of TestCase structs.
func loadTestCasesFromFile(filePath string) ([]TestCase, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("test cases file not found: %s", filePath)
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cases file '%s': %w", filePath, err)
	}

	if len(bytes.TrimSpace(fileBytes)) == 0 {
		fmt.Printf("Warning: Test cases file '%s' is empty.\n", filePath)
		return []TestCase{}, nil
	}
	if !json.Valid(fileBytes) {
		return nil, fmt.Errorf("invalid JSON format in test cases file: %s", filePath)
	}

	var testCases []TestCase
	err = json.Unmarshal(fileBytes, &testCases)
	if err != nil {
		syntaxErr, ok := err.(*json.SyntaxError)
		if ok {
			return nil, fmt.Errorf("JSON syntax error in '%s' at offset %d: %w", filePath, syntaxErr.Offset, err)
		}
		typeErr, ok := err.(*json.UnmarshalTypeError)
		if ok {
			return nil, fmt.Errorf("JSON type error in '%s': expected %v but got %s at offset %d: %w", filePath, typeErr.Type, typeErr.Value, typeErr.Offset, err)
		}
		return nil, fmt.Errorf("failed to parse JSON test cases from '%s': %w", filePath, err)
	}

	return testCases, nil
}

// buildDockerImageFromString builds a Docker image from the Dockerfile string.
func buildDockerImageFromString(cli *client.Client, config JudgeConfig) error {
	ctx := context.Background()
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)
	defer tw.Close()

	header := &tar.Header{
		Name:    "Dockerfile",
		Size:    int64(len(dockerfileContent)),
		Mode:    0644,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(header); err != nil {
		return fmt.Errorf("failed to write tar header for Dockerfile: %w", err)
	}
	if _, err := tw.Write([]byte(dockerfileContent)); err != nil {
		return fmt.Errorf("failed to write Dockerfile content to tar: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	dockerBuildContext := bytes.NewReader(tarBuf.Bytes())
	options := types.ImageBuildOptions{
		Tags:        []string{config.DockerImageName},
		Dockerfile:  "Dockerfile",
		Remove:      true,
		ForceRemove: true,
	}
	resp, err := cli.ImageBuild(ctx, dockerBuildContext, options)
	if err != nil {
		return fmt.Errorf("failed to initiate image build: %w", err)
	}
	defer resp.Body.Close()

	fmt.Println("--- Docker Build Output ---")
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Error reading build output stream: %v\n", err)
	}
	fmt.Println("--- End Docker Build Output ---")
	return nil
}

// compileProgram compiles the Go source code.
func compileProgram(sourceFile string) (executablePath string, compileLog string, err error) {
	tempDir := os.TempDir()
	baseName := strings.TrimSuffix(filepath.Base(sourceFile), filepath.Ext(sourceFile))
	execName := fmt.Sprintf("%s_judged_%d%s", baseName, time.Now().UnixNano(), executableSuffix())
	executablePath = filepath.Join(tempDir, execName)
	os.Remove(executablePath)

	cmd := exec.Command("go", "build", "-o", executablePath, sourceFile)
	var compileOutput bytes.Buffer
	cmd.Stderr = &compileOutput
	cmd.Stdout = &compileOutput

	fmt.Printf("Running compile command: %s\n", cmd.String())
	err = cmd.Run()
	compileLog = compileOutput.String()
	if err != nil {
		if _, statErr := os.Stat(executablePath); os.IsNotExist(statErr) {
			return "", compileLog, fmt.Errorf("compilation command failed and no executable produced: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Warning: Compilation command finished with error (but executable exists): %v\n", err)
		err = nil
	}
	if _, statErr := os.Stat(executablePath); os.IsNotExist(statErr) {
		return "", compileLog, fmt.Errorf("compilation finished but executable not found at %s (Compiler Output:\n%s)", executablePath, compileLog)
	}
	return executablePath, compileLog, nil
}

// executableSuffix returns the executable file extension based on OS.
func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// runTestCaseInDocker runs a single test case in a Docker container.
func runTestCaseInDocker(
	apiClient *client.Client,
	hostExecutablePath string,
	containerExecutablePath string,
	tc TestCase,
	config JudgeConfig,
) (result Result, output string, errMsg string) {
	ctx, cancel := context.WithTimeout(context.Background(), config.TimeLimitPerCase+5*time.Second)
	defer cancel()

	containerConfig := &container.Config{
		Image:       config.DockerImageName,
		Cmd:         []string{containerExecutablePath},
		AttachStdin: true, AttachStdout: true, AttachStderr: true,
		Tty:        false,
		OpenStdin:  true,
		StdinOnce:  true,
		User:       "appuser",
		WorkingDir: "/app",
	}
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{Type: mount.TypeBind, Source: hostExecutablePath, Target: containerExecutablePath, ReadOnly: true},
		},
		NetworkMode: "none",
		SecurityOpt: []string{"no-new-privileges"},
		Resources: container.Resources{
			Memory:     int64(config.MemoryLimitMB) * 1024 * 1024,
			MemorySwap: int64(config.MemoryLimitMB) * 1024 * 1024,
			NanoCPUs:   int64(config.CPUCount * 1e9),
		},
	}

	fmt.Printf("Creating container for test case...\n")
	resp, err := apiClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return RuntimeError, "", fmt.Sprintf("Failed to create container: %v", err)
	}
	containerID := resp.ID
	fmt.Printf("Container created: %s\n", containerID)

	defer func() {
		removeCtx, removeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer removeCancel()
		stopTimeout := 5
		stopErr := apiClient.ContainerStop(removeCtx, containerID, container.StopOptions{Timeout: &stopTimeout})
		if stopErr != nil && !client.IsErrNotFound(stopErr) && !strings.Contains(stopErr.Error(), "is already stopped") {
			fmt.Fprintf(os.Stderr, "Warning: Failed to stop container %s before removing: %v\n", containerID, stopErr)
		}
		fmt.Printf("Removing container %s...\n", containerID)
		removeOpts := container.RemoveOptions{Force: true}
		if err := apiClient.ContainerRemove(removeCtx, containerID, removeOpts); err != nil && !client.IsErrNotFound(err) {
			fmt.Fprintf(os.Stderr, "Warning: Failed to remove container %s: %v\n", containerID, err)
		} else {
			if stopErr == nil || (!client.IsErrNotFound(stopErr) && !strings.Contains(stopErr.Error(), "is already stopped")) {
				fmt.Printf("Container %s removed.\n", containerID)
			}
		}
	}()

	attachOptions := container.AttachOptions{Stream: true, Stdin: true, Stdout: true, Stderr: true}
	hijackedResp, err := apiClient.ContainerAttach(ctx, containerID, attachOptions)
	if err != nil {
		return RuntimeError, "", fmt.Sprintf("Failed to attach to container %s: %v", containerID, err)
	}
	defer hijackedResp.Close()

	fmt.Printf("Starting container %s...\n", containerID)
	if err := apiClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		if client.IsErrNotFound(err) {
			return RuntimeError, "", fmt.Sprintf("Failed to start container %s: container not found (possibly removed prematurely)", containerID)
		}
		return RuntimeError, "", fmt.Sprintf("Failed to start container %s: %v", containerID, err)
	}
	fmt.Printf("Container %s started.\n", containerID)

	inputErrChan := make(chan error, 1)
	go func() {
		defer func() {
			if err := hijackedResp.CloseWrite(); err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					// fmt.Fprintf(os.Stderr, "Warning: Error closing write stream for container %s: %v\n", containerID, err)
				}
			}
			close(inputErrChan)
		}()
		_, err := io.WriteString(hijackedResp.Conn, tc.Input+"\n")
		if err != nil {
			if err != io.ErrClosedPipe && !strings.Contains(err.Error(), "use of closed network connection") {
				inputErrChan <- fmt.Errorf("failed to write input to container %s: %w", containerID, err)
			}
		}
	}()

	var stdoutBuf, stderrBuf bytes.Buffer
	outputErrChan := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, hijackedResp.Reader)
		outputErrChan <- err
	}()

	statusCh, waitErrCh := apiClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	finalResult := Accepted
	finalOutput := ""
	finalErrMsg := ""

	select {
	case err := <-waitErrCh:
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				finalResult = TimeLimit
				finalErrMsg = fmt.Sprintf("Time Limit Exceeded (> %s)", config.TimeLimitPerCase)
				fmt.Printf("Context timed out (%s) while waiting for container %s.\n", config.TimeLimitPerCase, containerID)
				stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
				stopTimeout := 1
				_ = apiClient.ContainerStop(stopCtx, containerID, container.StopOptions{Timeout: &stopTimeout})
				stopCancel()
			} else {
				finalResult = RuntimeError
				finalErrMsg = fmt.Sprintf("Error waiting for container %s: %v", containerID, err)
			}
		}

	case status := <-statusCh:
		fmt.Printf("Container %s exited with status code: %d\n", containerID, status.StatusCode)
		if status.Error != nil {
			fmt.Printf("Container %s exit error message from Docker: %s\n", containerID, status.Error.Message)
		}

		outputWaitCtx, outputWaitCancel := context.WithTimeout(context.Background(), 2*time.Second)
		select {
		case copyErr := <-outputErrChan:
			if copyErr != nil && copyErr != io.EOF {
				fmt.Fprintf(os.Stderr, "Warning: Error copying container output streams for %s: %v\n", containerID, copyErr)
			}
		case <-outputWaitCtx.Done():
			fmt.Fprintf(os.Stderr, "Warning: Timed out waiting for output stream copy to finish for container %s\n", containerID)
		}
		outputWaitCancel()

		actualOutput := strings.TrimSpace(stdoutBuf.String())
		stderrOutput := strings.TrimSpace(stderrBuf.String())
		finalOutput = actualOutput

		if status.StatusCode != 0 {
			if status.StatusCode == 137 && config.MemoryLimitMB > 0 {
				finalResult = MemoryLimit
				finalErrMsg = fmt.Sprintf("Memory Limit Exceeded (exit code %d)", status.StatusCode)
			} else {
				finalResult = RuntimeError
				finalErrMsg = fmt.Sprintf("Container exited with non-zero status code %d.", status.StatusCode)
				if stderrOutput != "" {
					finalErrMsg += fmt.Sprintf("\nStderr:\n%s", stderrOutput)
				}
			}
		} else {
			expectedOutputTrimmed := strings.TrimSpace(tc.Expected)
			if actualOutput != expectedOutputTrimmed {
				finalResult = WrongAnswer
			} else {
				finalResult = Accepted
			}
		}
	}

	inputWaitCtx, inputWaitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	select {
	case inputErr := <-inputErrChan:
		if inputErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Input writing goroutine error for container %s: %v\n", containerID, inputErr)
		}
	case <-inputWaitCtx.Done():
		fmt.Fprintf(os.Stderr, "Warning: Timed out waiting for input writing goroutine to finish for container %s\n", containerID)
	}
	inputWaitCancel()

	return finalResult, finalOutput, finalErrMsg
}
