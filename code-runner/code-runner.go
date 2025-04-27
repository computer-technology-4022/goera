package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json" // Import the encoding/json package
	"fmt"
	"io"
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
	"github.com/spf13/cobra"
)

// The Dockerfile content as a Go string
const dockerfileContent = `
# ... (dockerfile content remains the same) ...
FROM golang:1.24-alpine as builder
FROM alpine:latest
RUN apk --no-cache add ca-certificates
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
RUN mkdir /app && chown appuser:appgroup /app
WORKDIR /app
USER appuser
# ENTRYPOINT ["/app/program_to_run"] # Entrypoint defined in run command instead
`

// TestCase represents a single test case with input and expected output.
// Added JSON tags for clear mapping.
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

// JudgeConfig holds the configuration for the judging process.
// Added TestCasesPath field.
type JudgeConfig struct {
	TimeLimitPerCase time.Duration
	MemoryLimitMB    uint64
	CPUCount         float64
	DockerImageName  string
	SourceFilePath   string // Path to the user's code file
	TestCasesPath    string // Path to the test cases JSON file
}

const DEFAULT_DOCKER_IMAGE = "go-judge-runner:latest"

// Variables to hold flag values
var (
	codePath      string
	testCasesPath string // Added variable for the test cases file path
	timeLimit     time.Duration
	memoryLimit   uint64
	cpuCount      float64
	dockerImage   string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "judge",
	Short: "A simple Go code judge",
	Long: `A simple Go code judge that compiles and runs a user's Go program
against predefined test cases (loaded from a JSON file) within a Docker container.
Builds the runner image from an embedded Dockerfile string.`,
	Run: func(cmd *cobra.Command, args []string) {
		runJudge()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&codePath, "codePath", "c", "", "Path to the Go source code file to judge (required)")
	rootCmd.MarkPersistentFlagRequired("codePath")

	// Add the flag for the test cases JSON file path
	rootCmd.PersistentFlags().StringVarP(&testCasesPath, "testCasesPath", "T", "", "Path to the JSON file containing test cases (required)")
	rootCmd.MarkPersistentFlagRequired("testCasesPath") // Make it required

	rootCmd.PersistentFlags().DurationVarP(&timeLimit, "timeLimit", "t", 2*time.Second, "Time limit per test case (e.g., 1s, 500ms)")
	rootCmd.PersistentFlags().Uint64VarP(&memoryLimit, "memoryLimit", "m", 64, "Memory limit per test case in MB")
	rootCmd.PersistentFlags().Float64VarP(&cpuCount, "cpuCount", "p", 1.0, "CPU limit per test case (e.g., 0.5, 1.0)")
	rootCmd.PersistentFlags().StringVarP(&dockerImage, "dockerImage", "i", DEFAULT_DOCKER_IMAGE, "Name of the Docker image to use/build for running code")
}

func main() {
	Execute() // Start the Cobra application
}

// loadTestCasesFromFile reads a JSON file and returns a slice of TestCase structs.
func loadTestCasesFromFile(filePath string) ([]TestCase, error) {
	// Check if file exists first for a clearer error message
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("test cases file not found: %s", filePath)
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test cases file '%s': %w", filePath, err)
	}

	// Handle empty file case explicitly
	if len(bytes.TrimSpace(fileBytes)) == 0 {
		fmt.Printf("Warning: Test cases file '%s' is empty.\n", filePath)
		return []TestCase{}, nil // Return empty slice, not an error
	}
	if !json.Valid(fileBytes) {
		// You might want more sophisticated validation depending on needs
		return nil, fmt.Errorf("invalid JSON format in test cases file: %s", filePath)
	}

	var testCases []TestCase
	// Ensure the JSON is an array, otherwise Unmarshal might succeed partially
	// For simplicity, we rely on Unmarshal's behavior for now.
	// A more robust check would unmarshal into an interface{} first and check type.
	err = json.Unmarshal(fileBytes, &testCases)
	if err != nil {
		// Provide more context on JSON parsing errors if possible
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

	// Optional: Add validation for individual test cases (e.g., ensure fields aren't empty)
	// for i, tc := range testCases {
	//  if tc.Input == "" || tc.Expected == "" {
	//      fmt.Printf("Warning: Test case %d in '%s' has empty input or expected output.\n", i+1, filePath)
	//  }
	// }

	return testCases, nil
}

// runJudge contains the core logic
func runJudge() {
	// Configuration for the judge (Defaults can be overridden by flags)
	config := JudgeConfig{
		TimeLimitPerCase: timeLimit,     // Use flag value
		MemoryLimitMB:    memoryLimit,   // Use flag value
		CPUCount:         cpuCount,      // Use flag value
		DockerImageName:  dockerImage,   // Use flag value
		SourceFilePath:   codePath,      // Use flag value
		TestCasesPath:    testCasesPath, // Use flag value
	}

	fmt.Println("initialized judge configuration")
	fmt.Printf("Loading test cases from: %s\n", config.TestCasesPath)

	// Load test cases from the specified file
	testCases, err := loadTestCasesFromFile(config.TestCasesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading test cases: %v\n", err)
		os.Exit(1) // Exit if test cases cannot be loaded
	}
	fmt.Printf("Loaded %d test cases.\n", len(testCases))
	if len(testCases) == 0 {
		fmt.Println("Warning: No test cases loaded. Judge will finish without running tests.")
		// Decide whether to exit or continue
		// os.Exit(0) // Or just let it proceed and report "Accepted" vacuously
	}

	// Initialize Docker client
	apiClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Docker client: %v\n", err)
		os.Exit(1)
	}
	defer apiClient.Close()
	fmt.Println("initialized docker client")

	// Build Docker Image from string
	fmt.Printf("Building Docker image '%s' from embedded Dockerfile string...\n", config.DockerImageName)
	err = buildDockerImageFromString(apiClient, config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building Docker image: %v\n", err)
		fmt.Printf("Result: %s\n", CompileError)
		os.Exit(1)
	}
	fmt.Println("Docker image built successfully.")

	// Compile User's Source Code
	fmt.Printf("Judging source file: %s\n", config.SourceFilePath)
	fmt.Println("Compiling source code on the host...")
	executablePath, compileLog, err := compileProgram(config.SourceFilePath)
	if err != nil {
		fmt.Printf("Result: %s\n", CompileError)
		fmt.Printf("Compilation Log:\n%s\n", compileLog)
		os.Exit(0)
	}
	defer os.Remove(executablePath)
	fmt.Printf("Compilation successful. Host Executable: %s\n", executablePath)

	// --- Resource Limits Info ---
	if config.MemoryLimitMB > 0 {
		fmt.Printf("Memory Limit per Test Case: %d MB\n", config.MemoryLimitMB)
	}
	if config.CPUCount > 0 {
		fmt.Printf("CPU Limit per Test Case: %.2f cores\n", config.CPUCount)
	}
	fmt.Printf("Time Limit per Test Case: %s\n", config.TimeLimitPerCase)
	// --- End Resource Limits Info ---

	// Get absolute path for volume mounting
	absExecutablePath, err := filepath.Abs(executablePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting absolute path for executable: %v\n", err)
		os.Exit(1)
	}
	containerExecutablePath := "/app/program_to_run"

	// Run test cases from the loaded slice
	overallResult := Accepted
	// Handle the case where no test cases were loaded
	if len(testCases) == 0 {
		fmt.Println("No test cases to run.")
		overallResult = Accepted // Or perhaps a different status like "NoTests"
	} else {
		for i, tc := range testCases {
			fmt.Printf("\n--- Running Test Case %d / %d ---\n", i+1, len(testCases))
			fmt.Printf("Input:\n%s\n", tc.Input) // Use tc.Input

			result, output, errMsg := runTestCaseInDocker(
				apiClient,
				absExecutablePath,
				containerExecutablePath,
				tc, // Pass the TestCase struct directly
				config,
			)

			fmt.Printf("Expected Output:\n%s\n", tc.Expected) // Use tc.Expected
			fmt.Printf("Actual Output:\n%s\n", output)
			if errMsg != "" {
				fmt.Printf("Error Details:\n%s\n", errMsg)
			}
			fmt.Printf("Test Case %d Result: %s\n", i+1, result)

			if result != Accepted {
				overallResult = result
				break // Stop on the first failed test case
			}
		}
	}

	fmt.Printf("\n--- Judge Finished ---\n")
	fmt.Printf("Overall Result: %s\n", overallResult)
}

// --- Other Functions (buildDockerImageFromString, compileProgram, executableSuffix, runTestCaseInDocker) remain unchanged ---
// buildDockerImageFromString function remains the same
func buildDockerImageFromString(cli *client.Client, config JudgeConfig) error {
	ctx := context.Background()
	tarBuf := new(bytes.Buffer)
	tw := tar.NewWriter(tarBuf)
	defer tw.Close() // Ensure writer is closed to flush data

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
	// IMPORTANT: Close the tar writer *before* creating the reader
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

// compileProgram function remains the same
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
		err = nil // Proceed if executable exists despite warnings
	}
	if _, statErr := os.Stat(executablePath); os.IsNotExist(statErr) {
		return "", compileLog, fmt.Errorf("compilation finished but executable not found at %s (Compiler Output:\n%s)", executablePath, compileLog)
	}
	return executablePath, compileLog, nil
}

// executableSuffix function remains the same
func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// runTestCaseInDocker function remains the same
func runTestCaseInDocker(
	apiClient *client.Client,
	hostExecutablePath string,
	containerExecutablePath string,
	tc TestCase, // Takes the TestCase struct
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
			// Suppress "removed" message if it was already not found during stop
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
		// Use tc.Input directly
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
		outputWaitCancel() // Ensure cancel is called

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
			// Use tc.Expected directly
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
	inputWaitCancel() // Ensure cancel is called

	return finalResult, finalOutput, finalErrMsg
}
