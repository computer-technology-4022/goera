package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// TestCase represents a single test case with input and expected output.
type TestCase struct {
	Input    string
	Expected string
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
type JudgeConfig struct {
	TimeLimitPerCase time.Duration
	MemoryLimitMB    uint64
	CPUCount         float64
	DockerImageName  string
}

const DEFAULT_DOCKER_IMAGE = "go-judge-runner:latest"

func main() {
	// --- Configuration ---
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <path/to/source.go>\n", os.Args[0])
		os.Exit(1)
	}
	sourceFilePath := os.Args[1]

	testCases := []TestCase{
		{Input: "1 2", Expected: "3"},
		{Input: "10 5", Expected: "15"},
		{Input: "-5 5", Expected: "0"},
		{Input: "100 200", Expected: "301"}, // Wrong Answer
	}

	config := JudgeConfig{
		TimeLimitPerCase: 2 * time.Second,
		MemoryLimitMB:    64,
		CPUCount:         1.0,
		DockerImageName:  DEFAULT_DOCKER_IMAGE,
	}
	// --- End Configuration ---

	// Initialize Docker client
	apiClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create Docker client: %v\n", err)
		os.Exit(1)
	}
	defer apiClient.Close()

	// --- Build Docker Image ---
	fmt.Printf("Building Docker image (%s)...\n", config.DockerImageName)
	err = buildDockerImage(apiClient, config.DockerImageName, ".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building Docker image: %v\n", err)
		fmt.Printf("Result: %s\n", CompileError)
		os.Exit(1)
	}
	fmt.Println("Docker image built successfully.")
	// --- End Build Docker Image ---

	fmt.Printf("Judging %s using Docker...\n", sourceFilePath)
	if config.MemoryLimitMB > 0 {
		fmt.Printf("Memory Limit: %d MB\n", config.MemoryLimitMB)
	}
	if config.CPUCount > 0 {
		fmt.Printf("CPU Limit: %.2f cores\n", config.CPUCount)
	}

	// Compile source code on host
	executablePath, compileLog, err := compileProgram(sourceFilePath)
	if err != nil {
		fmt.Printf("Result: %s\n", CompileError)
		fmt.Printf("Compilation Log:\n%s\n", compileLog)
		os.Exit(0)
	}
	defer os.Remove(executablePath)
	fmt.Printf("Compilation successful. Host Executable: %s\n", executablePath)

	// Get absolute path for volume mounting
	absExecutablePath, err := filepath.Abs(executablePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting absolute path for executable: %v\n", err)
		os.Exit(1)
	}
	containerExecutablePath := "/app/program_to_run"

	// Run test cases using Docker
	overallResult := Accepted
	for i, tc := range testCases {
		fmt.Printf("--- Running Test Case %d ---\n", i+1)
		fmt.Printf("Input:\n%s\n", tc.Input)

		result, output, errMsg := runTestCaseInDocker(apiClient, absExecutablePath, containerExecutablePath, tc, config)

		fmt.Printf("Expected Output:\n%s\n", tc.Expected)
		fmt.Printf("Actual Output:\n%s\n", output)
		if errMsg != "" {
			fmt.Printf("Error Details:\n%s\n", errMsg)
		}
		fmt.Printf("Test Case %d Result: %s\n", i+1, result)

		if result != Accepted {
			overallResult = result
			break
		}
	}

	fmt.Printf("--- Judge Finished ---\n")
	fmt.Printf("Overall Result: %s\n", overallResult)
}

// buildDockerImage builds the Docker image using the Docker API.
func buildDockerImage(cli *client.Client, imageName, contextDir string) error {
	fmt.Printf("Building Docker image (%s) from context '%s'\n", imageName, contextDir)
	// Note: For simplicity, this assumes the Dockerfile is in the contextDir.
	// In a production scenario, create a tarball of the context directory.
	// Here, we use an empty buffer as a placeholder; replace with actual tarball creation if needed.
	buildContext := bytes.NewReader([]byte{})
	options := types.ImageBuildOptions{
		Tags:       []string{imageName},
		Dockerfile: "Dockerfile",
	}

	resp, err := cli.ImageBuild(context.Background(), buildContext, options)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading build output: %w", err)
	}
	return nil
}

// compileProgram compiles the Go source file on the host.
func compileProgram(sourceFile string) (string, string, error) {
	tempDir := os.TempDir()
	execName := "judged_program_" + strconv.FormatInt(time.Now().UnixNano(), 10) + executableSuffix()
	executablePath := filepath.Join(tempDir, execName)
	os.Remove(executablePath)

	cmd := exec.Command("go", "build", "-o", executablePath, sourceFile)
	var compileOutput bytes.Buffer
	cmd.Stderr = &compileOutput
	cmd.Stdout = &compileOutput

	err := cmd.Run()
	if err != nil {
		return "", compileOutput.String(), fmt.Errorf("compilation failed: %w", err)
	}
	if _, err := os.Stat(executablePath); os.IsNotExist(err) {
		return "", compileOutput.String(), fmt.Errorf("executable not found at %s", executablePath)
	}
	return executablePath, compileOutput.String(), nil
}

// executableSuffix returns the appropriate suffix for executables based on the OS.
func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// runTestCaseInDocker runs a test case inside a Docker container using the Docker API.
func runTestCaseInDocker(apiClient *client.Client, hostExecutablePath, containerExecutablePath string, tc TestCase, config JudgeConfig) (Result, string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), config.TimeLimitPerCase+1*time.Second)
	defer cancel()

	// Container configuration
	containerConfig := &container.Config{
		Image:        config.DockerImageName,
		Cmd:          []string{containerExecutablePath},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	// Host configuration with resource limits
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   hostExecutablePath,
				Target:   containerExecutablePath,
				ReadOnly: true,
			},
		},
		NetworkMode: "none",
		SecurityOpt: []string{"no-new-privileges"},
		Resources: container.Resources{
			Memory:     int64(config.MemoryLimitMB) * 1024 * 1024,
			MemorySwap: int64(config.MemoryLimitMB) * 1024 * 1024,
			NanoCPUs:   int64(config.CPUCount * 1e9),
		},
	}

	// Create container
	resp, err := apiClient.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return RuntimeError, "", fmt.Sprintf("Failed to create container: %v", err)
	}
	containerID := resp.ID

	defer func() {
		if err := apiClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
			fmt.Printf("Failed to remove container %s: %v\n", containerID, err)
		}
	}()

	// Attach to container
	attachOptions := container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}
	hijackedResp, err := apiClient.ContainerAttach(ctx, containerID, attachOptions)
	if err != nil {
		return RuntimeError, "", fmt.Sprintf("Failed to attach to container: %v", err)
	}
	defer hijackedResp.Close()

	// Start container
	if err := apiClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return RuntimeError, "", fmt.Sprintf("Failed to start container: %v", err)
	}

	// Send input to stdin
	go func() {
		defer hijackedResp.CloseWrite()
		_, err := io.WriteString(hijackedResp.Conn, tc.Input)
		if err != nil {
			fmt.Printf("Failed to write input: %v\n", err)
		}
	}()

	// Capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, hijackedResp.Reader)
	if err != nil {
		return RuntimeError, "", fmt.Sprintf("Failed to read output: %v", err)
	}

	actualOutput := strings.TrimSpace(stdoutBuf.String())
	stderrOutput := strings.TrimSpace(stderrBuf.String())

	// Wait for container to finish
	statusCh, errCh := apiClient.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return RuntimeError, actualOutput, fmt.Sprintf("Error waiting for container: %v", err)
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			if status.StatusCode == 137 && config.MemoryLimitMB > 0 {
				return MemoryLimit, actualOutput, fmt.Sprintf("Memory Limit Exceeded (exit code %d)", status.StatusCode)
			}
			return RuntimeError, actualOutput, fmt.Sprintf("Container exited with code %d.\nStderr:\n%s", status.StatusCode, stderrOutput)
		}
	case <-ctx.Done():
		timeout := 5
		if err := apiClient.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
			fmt.Printf("Failed to stop container %s: %v\n", containerID, err)
		}
		return TimeLimit, actualOutput, "Process exceeded time limit"
	}

	// Check output
	expectedOutputTrimmed := strings.TrimSpace(tc.Expected)
	if actualOutput != expectedOutputTrimmed {
		return WrongAnswer, actualOutput, ""
	}

	return Accepted, actualOutput, ""
}
