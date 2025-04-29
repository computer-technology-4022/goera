package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
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

// ... (Keep Dockerfile content, TestCase, Result, JudgeConfig, SubmissionRequest, RunResponse, DEFAULT_DOCKER_IMAGE constants as they are) ...

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
	// NOTE: We now expect err to be nil even for compile errors,
	// so we only check for truly internal/unexpected errors here.
	result, output, err := runJudge(config)
	if err != nil {
		// This error should now only represent unexpected issues,
		// not handled failures like compile errors.
		http.Error(w, fmt.Sprintf("Internal judge error: %v\nOutput Log:\n%s", err, output), http.StatusInternalServerError)
		return
	}

	resp := RunResponse{
		QuestionID: req.QuestionID,
		Status:     result,
		Output:     output, // This output string contains logs, including compile errors if any
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// Log this error server-side as it's an issue encoding the final response
		fmt.Fprintf(os.Stderr, "Error encoding response: %v\n", err)
		// Avoid writing another header if one was already partially written
		// http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: coderunner <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  serve    Start the code runner server")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		listenAddr := serveCmd.String("listen", "8081", "Port to listen on (e.g., 8081 or :8081)")
		serveCmd.Parse(os.Args[2:])

		addr := *listenAddr
		if !strings.Contains(addr, ":") {
			addr = ":" + addr
		}

		http.HandleFunc("/run", runHandler)
		fmt.Printf("CodeRunner service listening on %s\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			fmt.Printf("Server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// runJudge executes the entire judging process: build image, compile, run tests.
// It now returns Result, output string, and a nil error for handled failures
// like Docker build or Go compilation errors. It only returns a non-nil error
// for unexpected issues (e.g., Docker client creation failure).
func runJudge(config JudgeConfig) (Result, string, error) {
	var outputBuf bytes.Buffer
	logWriter := io.MultiWriter(os.Stdout, &outputBuf) // Log to stdout and capture in buffer
	fmt.Fprintln(logWriter, "Initialized judge configuration")

	testCases := config.TestCases
	fmt.Fprintf(logWriter, "Loaded %d test cases.\n", len(testCases))
	if len(testCases) == 0 {
		fmt.Fprintln(logWriter, "Warning: No test cases provided.")
	}

	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		// This is an unexpected setup error, return it.
		fmt.Fprintf(logWriter, "FATAL: Failed to create Docker client: %v\n", err)
		return RuntimeError, outputBuf.String(), fmt.Errorf("failed to create Docker client: %w", err)
	}
	defer apiClient.Close()
	fmt.Fprintln(logWriter, "Initialized Docker client")

	// Build Docker image
	fmt.Fprintf(logWriter, "Building Docker image '%s' from embedded Dockerfile string...\n", config.DockerImageName)
	err = buildDockerImageFromString(apiClient, config, logWriter) // Pass logWriter
	if err != nil {
		// Log the build error details into the buffer
		fmt.Fprintf(logWriter, "Docker Image Build Failed: %v\n", err)
		fmt.Fprintf(logWriter, "Result: %s\n", CompileError)
		// *** CHANGE HERE: Return nil error as this is a handled failure state ***
		return CompileError, outputBuf.String(), nil
	}
	fmt.Fprintln(logWriter, "Docker image built successfully.")

	// Compile source code
	executablePath, compileLog, err := compileProgram(config.SourceFilePath)
	// Always log the compile output, regardless of error
	if compileLog != "" {
		fmt.Fprintf(logWriter, "--- Compilation Log ---\n%s\n--- End Compilation Log ---\n", compileLog)
	}
	if err != nil {
		// Log compilation failure details
		fmt.Fprintf(logWriter, "Go Compilation Failed: %v\n", err) // Log the error message itself
		fmt.Fprintf(logWriter, "Result: %s\n", CompileError)
		// *** CHANGE HERE: Return nil error as this is a handled failure state ***
		return CompileError, outputBuf.String(), nil
	}
	// If compilation succeeded, remove the executable when done.
	defer os.Remove(executablePath) // Only schedule removal if compilation was successful
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
		// This is an unexpected file system error, return it.
		fmt.Fprintf(logWriter, "FATAL: Error getting absolute path for executable: %v\n", err)
		return RuntimeError, outputBuf.String(), fmt.Errorf("error getting absolute path for executable: %w", err)
	}
	containerExecutablePath := "/app/program_to_run"

	// Run test cases
	overallResult := Accepted // Default to Accepted if no test cases
	if len(testCases) == 0 {
		fmt.Fprintln(logWriter, "No test cases to run.")
	} else {
		for i, tc := range testCases {
			fmt.Fprintf(logWriter, "\n--- Running Test Case %d / %d ---\n", i+1, len(testCases))
			fmt.Fprintf(logWriter, "Input:\n%s\n", tc.Input)

			// Pass logWriter to runTestCaseInDocker for detailed logging
			result, output, errMsg := runTestCaseInDocker(
				apiClient,
				absExecutablePath,
				containerExecutablePath,
				tc,
				config,
				logWriter, // Pass log writer
			)

			fmt.Fprintf(logWriter, "Expected Output:\n%s\n", tc.Expected)
			fmt.Fprintf(logWriter, "Actual Output:\n%s\n", output) // Output from container stdout
			if errMsg != "" {
				fmt.Fprintf(logWriter, "Execution Details/Error:\n%s\n", errMsg) // Error message from container run
			}
			fmt.Fprintf(logWriter, "Test Case %d Result: %s\n", i+1, result)

			if result != Accepted {
				overallResult = result // Store the first non-Accepted result
				break                  // Stop processing further test cases
			}
		}
	}

	fmt.Fprintf(logWriter, "\n--- Judge Finished ---\n")
	fmt.Fprintf(logWriter, "Overall Result: %s\n", overallResult)

	// Return the final result, the full captured log, and nil error for handled outcomes
	return overallResult, outputBuf.String(), nil
}

// ... (Keep loadTestCasesFromFile as it is) ...
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
// Added io.Writer for logging build output.
func buildDockerImageFromString(cli *client.Client, config JudgeConfig, logWriter io.Writer) error {
	ctx := context.Background()
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)
	// No need to defer tw.Close() here, it's closed explicitly before reading

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
		// If write fails, still try to close to release resources, then return write error
		tw.Close()
		return fmt.Errorf("failed to write Dockerfile content to tar: %w", err)
	}
	// Close the tar writer *before* using the buffer
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}

	dockerBuildContext := bytes.NewReader(tarBuf.Bytes())
	options := types.ImageBuildOptions{
		Tags:        []string{config.DockerImageName},
		Dockerfile:  "Dockerfile", // Refers to the Dockerfile within the tar context
		Remove:      true,         // Attempt to remove intermediate containers
		ForceRemove: true,         // Force removal of intermediate containers
		// Consider adding NoCache: true if needed during development
	}
	resp, err := cli.ImageBuild(ctx, dockerBuildContext, options)
	if err != nil {
		return fmt.Errorf("failed to initiate image build request: %w", err)
	}
	defer resp.Body.Close()

	// Stream build output to the provided logWriter
	fmt.Fprintln(logWriter, "--- Docker Build Output ---")
	buildOutputBuf := new(bytes.Buffer) // Capture build output separately for error reporting
	buildLogAndCaptureWriter := io.MultiWriter(logWriter, buildOutputBuf)

	scanner := bufio.NewScanner(resp.Body)
	var buildErr error // Variable to store potential JSON error message from Docker daemon
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(buildLogAndCaptureWriter, line) // Write line to main log and capture buffer

		// Try to detect errors reported in the JSON stream from Docker
		var msg struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}
		if json.Unmarshal([]byte(line), &msg) == nil {
			if msg.Error != "" {
				buildErr = fmt.Errorf("docker build error: %s", msg.Error)
				// Don't break, continue reading the full log
			} else if msg.ErrorDetail.Message != "" {
				buildErr = fmt.Errorf("docker build error: %s", msg.ErrorDetail.Message)
				// Don't break, continue reading the full log
			}
		}
	}

	scanErr := scanner.Err()
	fmt.Fprintln(logWriter, "--- End Docker Build Output ---")

	// Check for errors during scanning or reported by Docker
	if scanErr != nil {
		return fmt.Errorf("error reading docker build output stream: %w. Partial log:\n%s", scanErr, buildOutputBuf.String())
	}
	if buildErr != nil {
		// Return the specific error message captured from the Docker build log
		return fmt.Errorf("docker build failed: %w. Full log:\n%s", buildErr, buildOutputBuf.String())
	}

	// If no errors were detected, return nil
	return nil
}

// compileProgram compiles the Go source code.
func compileProgram(sourceFile string) (executablePath string, compileLog string, err error) {
	tempDir := os.TempDir()
	// Ensure baseName is safe for file system use (though unlikely problematic here)
	safeBaseName := strings.ReplaceAll(filepath.Base(sourceFile), "..", "_")
	baseName := strings.TrimSuffix(safeBaseName, filepath.Ext(safeBaseName))

	// Use a more unique name to avoid potential collisions
	execName := fmt.Sprintf("%s_judged_%d%s", baseName, time.Now().UnixNano(), executableSuffix())
	executablePath = filepath.Join(tempDir, execName)
	os.Remove(executablePath) // Clean up any potential leftovers first

	// Use context for potential timeout (though less critical for local compilation)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30-second compile timeout
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-o", executablePath, sourceFile)
	var compileOutput bytes.Buffer
	cmd.Stderr = &compileOutput
	cmd.Stdout = &compileOutput // Capture stdout as well

	fmt.Printf("Running compile command: %s\n", cmd.String()) // Log the command being run
	startTime := time.Now()
	err = cmd.Run()
	duration := time.Since(startTime)
	compileLog = compileOutput.String() // Capture log regardless of error

	fmt.Printf("Compile command finished in %s. Error (if any): %v\n", duration, err)

	if ctx.Err() == context.DeadlineExceeded {
		// Explicitly handle timeout
		return "", compileLog, fmt.Errorf("compilation timed out after %s: %w\nCompiler Output:\n%s", duration, ctx.Err(), compileLog)
	}

	if err != nil {
		// If 'go build' returned any error (including non-zero exit status).
		// The error object often includes useful info like "exit status 1".
		// No need to stat the file here, `cmd.Run()` error is sufficient indication of failure.
		return "", compileLog, fmt.Errorf("compilation command failed: %w\nCompiler Output:\n%s", err, compileLog)
	}

	// Double-check executable exists *only* if cmd.Run() reported success (err == nil).
	// This is a safeguard against unexpected behavior where 'go build' exits 0 but fails silently.
	if _, statErr := os.Stat(executablePath); os.IsNotExist(statErr) {
		return "", compileLog, fmt.Errorf("compilation command succeeded but executable '%s' not found. Compiler Output:\n%s", executablePath, compileLog)
	}

	// Compilation successful
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
// Added io.Writer for logging internal steps.
func runTestCaseInDocker(
	apiClient *client.Client,
	hostExecutablePath string,
	containerExecutablePath string,
	tc TestCase,
	config JudgeConfig,
	logWriter io.Writer, // Added log writer
) (result Result, output string, errMsg string) {
	// Increase parent context timeout slightly to allow for cleanup
	ctx, cancel := context.WithTimeout(context.Background(), config.TimeLimitPerCase+10*time.Second)
	defer cancel()

	// Use a specific logger for this function's internal steps
	logf := func(format string, args ...interface{}) {
		fmt.Fprintf(logWriter, " [ContainerRunner] "+format+"\n", args...)
	}

	containerConfig := &container.Config{
		Image:       config.DockerImageName,
		Cmd:         []string{containerExecutablePath}, // Command to run inside
		AttachStdin: true, AttachStdout: true, AttachStderr: true,
		Tty:        false,     // Important for non-interactive execution
		OpenStdin:  true,      // Keep stdin open to write input
		StdinOnce:  true,      // Close stdin after first write (standard for competitive programming)
		User:       "appuser", // Run as non-root user specified in Dockerfile
		WorkingDir: "/app",    // Working directory inside container
	}
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,          // Bind mount the executable
				Source:   hostExecutablePath,      // Path on the host
				Target:   containerExecutablePath, // Path inside the container
				ReadOnly: true,                    // Mount read-only for security
			},
		},
		NetworkMode: "none",                        // Disable networking for security
		SecurityOpt: []string{"no-new-privileges"}, // Prevent privilege escalation
		Resources: container.Resources{
			// Memory limit in bytes. MemorySwap = Memory enforces no swap usage.
			Memory: int64(config.MemoryLimitMB) * 1024 * 1024,
			// Setting MemorySwap to the same value as Memory disables swap usage effectively.
			// Set to -1 to allow unlimited swap (not recommended for judging).
			MemorySwap: int64(config.MemoryLimitMB) * 1024 * 1024,
			// CPU limit in units of 1e9 nanoCPUs (e.g., 1.0 * 1e9 = 1 full core)
			NanoCPUs: int64(config.CPUCount * 1e9),
			// Consider adding PidsLimit if needed
		},
	}

	logf("Creating container with image '%s'...", config.DockerImageName)
	resp, err := apiClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "") // Auto-generates container name
	if err != nil {
		// Use specific Result type? Maybe RuntimeError is okay.
		return RuntimeError, "", fmt.Sprintf("Failed to create container: %v", err)
	}
	containerID := resp.ID
	logf("Container created: %s", containerID)

	// Defer container stop and removal
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second) // Generous timeout for cleanup
		defer stopCancel()

		logf("Stopping container %s...", containerID)
		// Use a short timeout for stop, otherwise force remove later
		stopTimeoutSecs := 2
		stopErr := apiClient.ContainerStop(stopCtx, containerID, container.StopOptions{Timeout: &stopTimeoutSecs})
		if stopErr != nil && !client.IsErrNotFound(stopErr) && !strings.Contains(stopErr.Error(), "is already stopped") {
			logf("Warning: Failed to stop container %s gracefully: %v. Will force remove.", containerID, stopErr)
		} else if stopErr == nil {
			logf("Container %s stopped.", containerID)
		}

		logf("Removing container %s...", containerID)
		removeOpts := container.RemoveOptions{
			Force:         true,  // Force removal if stop failed or it's stuck
			RemoveVolumes: false, // We didn't create volumes, but good practice
		}
		if removeErr := apiClient.ContainerRemove(stopCtx, containerID, removeOpts); removeErr != nil && !client.IsErrNotFound(removeErr) {
			// Log error but don't fail the entire judge process just for cleanup failure
			logf("Warning: Failed to remove container %s: %v", containerID, removeErr)
		} else if removeErr == nil {
			logf("Container %s removed.", containerID)
		}
	}()

	// Attach to container streams before starting
	attachOptions := container.AttachOptions{Stream: true, Stdin: true, Stdout: true, Stderr: true}
	logf("Attaching to container %s streams...", containerID)
	hijackedResp, err := apiClient.ContainerAttach(ctx, containerID, attachOptions)
	if err != nil {
		return RuntimeError, "", fmt.Sprintf("Failed to attach to container %s: %v", containerID, err)
	}
	defer hijackedResp.Close() // Close the connection when done

	// Start the container
	logf("Starting container %s...", containerID)
	startCtx, startCancel := context.WithTimeout(ctx, 5*time.Second) // Timeout for start itself
	err = apiClient.ContainerStart(startCtx, containerID, container.StartOptions{})
	startCancel() // Release start context resources
	if err != nil {
		// Check if the error is context deadline exceeded from the *parent* context
		if ctx.Err() == context.DeadlineExceeded {
			return TimeLimit, "", fmt.Sprintf("Time limit exceeded before container %s could start", containerID)
		}
		// Check specifically if the start timed out
		if err == context.DeadlineExceeded { // This checks startCtx timeout
			return RuntimeError, "", fmt.Sprintf("Timed out starting container %s: %v", containerID, err)
		}
		if client.IsErrNotFound(err) {
			return RuntimeError, "", fmt.Sprintf("Failed to start container %s: container not found (possible premature removal?)", containerID)
		}
		return RuntimeError, "", fmt.Sprintf("Failed to start container %s: %v", containerID, err)
	}
	logf("Container %s started and attached.", containerID)

	// Goroutine to write input to container's stdin
	inputErrChan := make(chan error, 1)
	go func() {
		defer func() {
			// Close the write half of the connection to signal EOF to the container process
			if err := hijackedResp.CloseWrite(); err != nil {
				// Ignore "use of closed network connection" as it's expected if context cancels early
				if !strings.Contains(err.Error(), "use of closed network connection") && !strings.Contains(err.Error(), "file already closed") {
					logf("Warning: Error closing write stream for container %s: %v", containerID, err)
				}
			}
			close(inputErrChan) // Signal that writing is done
			logf("Input goroutine finished for %s.", containerID)
		}()

		logf("Writing input to container %s stdin...", containerID)
		// Use a buffer and ensure a newline if input doesn't end with one
		inputToWrite := tc.Input
		if !strings.HasSuffix(inputToWrite, "\n") {
			inputToWrite += "\n"
		}

		written, err := io.WriteString(hijackedResp.Conn, inputToWrite)
		if err != nil {
			// Ignore ErrClosedPipe which can happen if container exits before reading all input
			if err != io.ErrClosedPipe && !strings.Contains(err.Error(), "use of closed network connection") {
				inputErrChan <- fmt.Errorf("failed to write input to container %s (%d bytes written): %w", containerID, written, err)
			} else {
				logf("Input stream closed while writing to %s (container likely exited). Bytes written: %d", containerID, written)
			}
		} else {
			logf("Successfully wrote %d bytes of input to %s.", written, containerID)
		}
	}()

	// Goroutine to copy stdout/stderr from container
	var stdoutBuf, stderrBuf bytes.Buffer
	outputErrChan := make(chan error, 1)
	go func() {
		logf("Starting output stream copy for %s...", containerID)
		// stdcopy.StdCopy demultiplexes the stream into separate stdout/stderr buffers
		_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, hijackedResp.Reader)
		outputErrChan <- err // Send error (or nil) when copying finishes
		logf("Output stream copy finished for %s. Error (if any): %v", containerID, err)
	}()

	// Wait for container to exit or timeout
	// Use a specific timeout context based on the *test case time limit*
	waitCtx, waitCancel := context.WithTimeout(ctx, config.TimeLimitPerCase)
	defer waitCancel() // Ensure wait context is cancelled

	statusCh, waitErrCh := apiClient.ContainerWait(waitCtx, containerID, container.WaitConditionNotRunning)

	finalResult := Accepted // Assume success initially
	finalOutput := ""
	finalErrMsg := ""

	logf("Waiting for container %s to exit (Timeout: %s)...", containerID, config.TimeLimitPerCase)

	select {
	case err := <-waitErrCh:
		// Error occurred while waiting (could be context cancelled, Docker daemon issue)
		if err != nil {
			// Check if the error is specifically the context deadline being exceeded (TLE)
			if waitCtx.Err() == context.DeadlineExceeded || ctx.Err() == context.DeadlineExceeded {
				logf("Container %s hit time limit (%s).", containerID, config.TimeLimitPerCase)
				finalResult = TimeLimit
				finalErrMsg = fmt.Sprintf("Time Limit Exceeded (> %s)", config.TimeLimitPerCase)
				// Attempt to get partial output if available
				<-outputErrChan // Wait briefly for output copy goroutine
				finalOutput = strings.TrimSpace(stdoutBuf.String())
				stderrStr := strings.TrimSpace(stderrBuf.String())
				if stderrStr != "" {
					finalErrMsg += fmt.Sprintf("\nPartial Stderr:\n%s", stderrStr)
				}
			} else {
				logf("Error waiting for container %s: %v", containerID, err)
				finalResult = RuntimeError
				finalErrMsg = fmt.Sprintf("Error waiting for container: %v", err)
				<-outputErrChan                                     // Wait briefly for output copy goroutine
				finalOutput = strings.TrimSpace(stdoutBuf.String()) // Capture any output before error
			}
		}
		// If err is nil here, it means waiting succeeded but maybe statusCh has the result. Should not happen often with WaitConditionNotRunning.

	case status := <-statusCh:
		// Container exited normally (status code might be non-zero)
		logf("Container %s exited with status code: %d. Docker Error Msg: '%s'", containerID, status.StatusCode, status.Error)

		// Wait for the output streaming goroutine to finish copying *after* container exits.
		// Use a short timeout for this wait.
		outputWaitCtx, outputWaitCancel := context.WithTimeout(context.Background(), 5*time.Second)
		select {
		case copyErr := <-outputErrChan:
			if copyErr != nil && copyErr != io.EOF {
				// Log error but proceed, output might be incomplete
				logf("Warning: Error reading container output streams for %s: %v", containerID, copyErr)
				finalErrMsg += fmt.Sprintf("\nWarning: Error reading container output: %v", copyErr)
			} else {
				logf("Output streams copied successfully for %s.", containerID)
			}
		case <-outputWaitCtx.Done():
			logf("Warning: Timed out waiting for output stream copy to finish for container %s. Output might be incomplete.", containerID)
			finalErrMsg += "\nWarning: Timed out reading full container output."
		}
		outputWaitCancel()

		// Process the captured output and status code
		actualOutput := strings.TrimSpace(stdoutBuf.String())
		stderrOutput := strings.TrimSpace(stderrBuf.String())
		finalOutput = actualOutput // Use stdout as the primary output

		if status.StatusCode != 0 {
			// OOM Killer typically results in 137. Check if memory limit was set.
			if status.StatusCode == 137 && config.MemoryLimitMB > 0 {
				logf("Container %s likely hit memory limit (exit code 137).", containerID)
				finalResult = MemoryLimit
				finalErrMsg = fmt.Sprintf("Memory Limit Exceeded (%d MB, exit code %d)", config.MemoryLimitMB, status.StatusCode)
				if stderrOutput != "" {
					finalErrMsg += fmt.Sprintf("\nStderr:\n%s", stderrOutput)
				}
			} else if status.StatusCode == 139 { // Segmentation fault
				logf("Container %s caused a segmentation fault (exit code 139).", containerID)
				finalResult = RuntimeError
				finalErrMsg = fmt.Sprintf("Runtime Error: Segmentation Fault (exit code %d)", status.StatusCode)
				if stderrOutput != "" {
					finalErrMsg += fmt.Sprintf("\nStderr:\n%s", stderrOutput)
				}
			} else {
				logf("Container %s exited with non-zero status: %d.", containerID, status.StatusCode)
				finalResult = RuntimeError
				finalErrMsg = fmt.Sprintf("Runtime Error: Container exited with non-zero status code %d.", status.StatusCode)
				if stderrOutput != "" {
					finalErrMsg += fmt.Sprintf("\nStderr:\n%s", stderrOutput)
				}
			}
		} else {
			// Exit code 0, check against expected output
			expectedOutputTrimmed := strings.TrimSpace(tc.Expected)
			// Normalize line endings for comparison (replace \r\n with \n)
			actualOutputNormalized := strings.ReplaceAll(actualOutput, "\r\n", "\n")
			expectedOutputNormalized := strings.ReplaceAll(expectedOutputTrimmed, "\r\n", "\n")

			if actualOutputNormalized != expectedOutputNormalized {
				logf("Container %s output mismatch.", containerID)
				finalResult = WrongAnswer
				// Optionally include diff or snippets in errMsg for debugging
				finalErrMsg = "Output does not match expected output."
				// Keep finalOutput as the actual program output for the user
			} else {
				logf("Container %s output matched expected output.", containerID)
				finalResult = Accepted
				// No error message needed for Accepted
			}
		}
	}

	logf("runTestCaseInDocker finished for %s. Result: %s", containerID, finalResult)
	return finalResult, finalOutput, finalErrMsg
}
