package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
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

// CodeRunner represents a code-runner instance
type CodeRunner struct {
	Port    int
	Busy    bool
	Process *exec.Cmd
}

// PortConfig stores information about all code-runner ports
type PortConfig struct {
	Ports []int `json:"ports"` // List of all ports used by code-runners
}

// RunnerProcess stores information about a running code-runner
type RunnerProcess struct {
	Port  int       `json:"port"`
	PID   int       `json:"pid"`
	State string    `json:"state"`
	Time  time.Time `json:"startTime"`
}

// RunnerState stores the state of all running code-runners
type RunnerState struct {
	Runners []RunnerProcess `json:"runners"`
}

const (
	ConfigFile      = "runner_config.json"
	DefaultPort     = 8081
	RunnerStateFile = "runner_state.json"
)

var (
	queue []*PendingSubmission
	mu    sync.Mutex
)

// loadPortConfig loads the port configuration from JSON file
func loadPortConfig() PortConfig {
	config := PortConfig{Ports: []int{DefaultPort}}

	// Check if config file exists
	if _, err := os.Stat(ConfigFile); os.IsNotExist(err) {
		// Create default config file
		savePortConfig(config)
		return config
	}

	// Read config file
	data, err := os.ReadFile(ConfigFile)
	if err != nil {
		log.Printf("Error reading config file: %v, using default config", err)
		return config
	}

	// Parse config
	err = json.Unmarshal(data, &config)
	if err != nil {
		log.Printf("Error parsing config file: %v, using default config", err)
		return config
	}

	return config
}

// savePortConfig saves the port configuration to JSON file
func savePortConfig(config PortConfig) {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		log.Printf("Error encoding config: %v", err)
		return
	}

	err = os.WriteFile(ConfigFile, data, 0644)
	if err != nil {
		log.Printf("Error writing config file: %v", err)
	}
}

// addPort adds a port to the port configuration
func addPort(port int) {
	config := loadPortConfig()

	// Check if port already exists
	for _, p := range config.Ports {
		if p == port {
			return // Port already in list
		}
	}

	// Add port to list
	config.Ports = append(config.Ports, port)
	savePortConfig(config)
}

// removePort removes a port from the port configuration
func removePort(port int) {
	config := loadPortConfig()

	// Filter out the port
	newPorts := make([]int, 0)
	for _, p := range config.Ports {
		if p != port {
			newPorts = append(newPorts, p)
		}
	}

	config.Ports = newPorts
	savePortConfig(config)
}

// getNextPort gets the next available port
func getNextPort() int {
	config := loadPortConfig()

	if len(config.Ports) == 0 {
		return DefaultPort + 1
	}

	// Find highest port number
	highestPort := DefaultPort
	for _, port := range config.Ports {
		if port > highestPort {
			highestPort = port
		}
	}

	return highestPort + 1
}

// listAllPorts returns a list of all ports in use
func listAllPorts() []int {
	config := loadPortConfig()
	return config.Ports
}

// loadRunnerState loads the state of running code-runners
func loadRunnerState() RunnerState {
	state := RunnerState{Runners: make([]RunnerProcess, 0)}

	// Check if state file exists
	if _, err := os.Stat(RunnerStateFile); os.IsNotExist(err) {
		return state
	}

	// Read state file
	data, err := os.ReadFile(RunnerStateFile)
	if err != nil {
		log.Printf("Error reading runner state file: %v", err)
		return state
	}

	// Parse state
	err = json.Unmarshal(data, &state)
	if err != nil {
		log.Printf("Error parsing runner state file: %v", err)
		return state
	}

	return state
}

// saveRunnerState saves the state of running code-runners
func saveRunnerState(state RunnerState) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("Error encoding runner state: %v", err)
		return
	}

	err = os.WriteFile(RunnerStateFile, data, 0644)
	if err != nil {
		log.Printf("Error writing runner state file: %v", err)
	}
}

// addRunnerToState adds a runner process to the state file
func addRunnerToState(port, pid int) {
	state := loadRunnerState()

	// Check if runner already exists and update it
	for i, runner := range state.Runners {
		if runner.Port == port {
			state.Runners[i].PID = pid
			state.Runners[i].State = "running"
			state.Runners[i].Time = time.Now()
			saveRunnerState(state)
			return
		}
	}

	// Add new runner
	state.Runners = append(state.Runners, RunnerProcess{
		Port:  port,
		PID:   pid,
		State: "running",
		Time:  time.Now(),
	})

	saveRunnerState(state)
}

// removeRunnerFromState removes a runner process from the state file
func removeRunnerFromState(port int) {
	state := loadRunnerState()

	// Filter out the runner with the given port
	newRunners := make([]RunnerProcess, 0)
	for _, runner := range state.Runners {
		if runner.Port != port {
			newRunners = append(newRunners, runner)
		}
	}

	state.Runners = newRunners
	saveRunnerState(state)
}

// killCodeRunner kills a code-runner by port
func killCodeRunner(port int) error {
	state := loadRunnerState()

	// Find the runner with the given port
	var targetPID int
	found := false

	for _, runner := range state.Runners {
		if runner.Port == port {
			targetPID = runner.PID
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("no code-runner found on port %d", port)
	}

	// Kill the process
	process, err := os.FindProcess(targetPID)
	if err != nil {
		return fmt.Errorf("failed to find process with PID %d: %v", targetPID, err)
	}

	err = process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill process with PID %d: %v", targetPID, err)
	}

	// Remove from state file
	removeRunnerFromState(port)

	// Remove from port config
	removePort(port)

	log.Printf("Killed code-runner on port %d (PID: %d)\n", port, targetPID)
	return nil
}

// killAllCodeRunners kills all running code-runners
func killAllCodeRunners() {
	state := loadRunnerState()

	if len(state.Runners) == 0 {
		log.Println("No running code-runners found")
		return
	}

	success := 0
	failed := 0

	for _, runner := range state.Runners {
		process, err := os.FindProcess(runner.PID)
		if err != nil {
			log.Printf("Failed to find process for code-runner on port %d (PID: %d): %v\n",
				runner.Port, runner.PID, err)
			failed++
			continue
		}

		err = process.Kill()
		if err != nil {
			log.Printf("Failed to kill code-runner on port %d (PID: %d): %v\n",
				runner.Port, runner.PID, err)
			failed++
		} else {
			log.Printf("Killed code-runner on port %d (PID: %d)\n", runner.Port, runner.PID)
			removePort(runner.Port)
			success++
		}
	}

	// Clear the state file
	saveRunnerState(RunnerState{Runners: make([]RunnerProcess, 0)})

	log.Printf("Successfully killed %d code-runners, failed to kill %d\n", success, failed)
}

// cleanup deletes configuration files
func cleanup() {
	log.Println("Cleaning up configuration files...")

	// Remove configuration files
	if err := os.Remove(ConfigFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Error removing %s: %v", ConfigFile, err)
	} else {
		log.Printf("Removed %s", ConfigFile)
	}

	if err := os.Remove(RunnerStateFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Error removing %s: %v", RunnerStateFile, err)
	} else {
		log.Printf("Removed %s", RunnerStateFile)
	}

	log.Println("Cleanup complete")
}

// setupCleanupHandler sets up signal handling for clean shutdown
func setupCleanupHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		log.Println("Shutdown signal received...")
		cleanup()
		os.Exit(0)
	}()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: judge <command> [options]")
		fmt.Println("Commands:")
		fmt.Println("  serve              Start the judge serve")
		fmt.Println("  coderunner         Start a new code-runner")
		fmt.Println("  killcoderunner     Kill a specific code-runner")
		fmt.Println("  killallcoderunners Kill all code-runners")
		fmt.Println("  allcoderunners     List all code-runner ports")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
		listenAddr := serveCmd.String("listen", "8080", "Port to listen on (e.g., 8080 or :8080)")
		serveCmd.Parse(os.Args[2:])

		addr := *listenAddr
		if !strings.Contains(addr, ":") {
			addr = ":" + addr
		}

		// Setup cleanup handler for SIGINT/SIGTERM
		setupCleanupHandler()

		// Also cleanup on normal exit
		defer cleanup()

		http.HandleFunc("/submit", submitHandler)

		log.Printf("Judge service running on %s\n", addr)
		log.Printf("Press Ctrl+C to exit (config files will be deleted)\n")
		log.Fatal(http.ListenAndServe(addr, nil))

	case "coderunner":
		runnerCmd := flag.NewFlagSet("coderunner", flag.ExitOnError)
		port := runnerCmd.Int("port", 0, "Port for the new code-runner (0 = auto-assign)")
		runnerCmd.Parse(os.Args[2:])

		// If port is not specified (or is 0), get the next available port
		if *port == 0 {
			*port = getNextPort()
		}

		startCodeRunner(*port)

	case "killcoderunner":
		killCmd := flag.NewFlagSet("killcoderunner", flag.ExitOnError)
		port := killCmd.Int("port", 0, "Port of the code-runner to kill")
		killCmd.Parse(os.Args[2:])

		if *port == 0 {
			fmt.Println("Error: --port is required")
			killCmd.PrintDefaults()
			os.Exit(1)
		}

		err := killCodeRunner(*port)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "killallcoderunners":
		killAllCodeRunners()

	case "allcoderunners":
		ports := listAllPorts()
		if len(ports) == 0 {
			fmt.Println("No code-runners found")
		} else {
			fmt.Println("Code-runner ports:")
			for _, port := range ports {
				fmt.Printf("  %d\n", port)
			}
			fmt.Printf("Total: %d code-runners\n", len(ports))
		}

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func startCodeRunner(port int) {
	log.Printf("Starting code-runner on port %d\n", port)
	cmd := exec.Command("./code-runner/code-runner", "serve", "--listen", fmt.Sprintf("%d", port))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start code-runner: %v", err)
	}

	// Store process info
	pid := cmd.Process.Pid
	addRunnerToState(port, pid)

	// Add port to configuration
	addPort(port)

	log.Printf("Code-runner started on port %d with PID %d\n", port, pid)

	// Wait for process in background
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("Code-runner on port %d exited with error: %v\n", port, err)
		} else {
			log.Printf("Code-runner on port %d exited normally\n", port)
		}
		// Update state when process ends
		removeRunnerFromState(port)
		// Don't remove port from configuration automatically
		// as it's part of the history
	}()
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

	state := loadRunnerState()
	mu.Lock()
	defer mu.Unlock()

	// Check if any code-runner is available
	for _, runner := range state.Runners {
		// Skip non-running or already busy runners
		if runner.State != "running" {
			continue
		}

		// Try to find an available runner
		if isBusy, _ := isRunnerBusy(runner.Port); !isBusy {
			log.Printf("Code-runner on port %d is free. Sending submission immediately.", runner.Port)
			go processSubmission(&sub, runner.Port)
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte("Submission accepted"))
			return
		}
	}

	// All code-runners are busy, queue the submission
	log.Println("All code-runners busy. Queuing submission.")
	queue = append(queue, &sub)
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Submission queued"))
}

// isRunnerBusy checks if a runner is currently busy
func isRunnerBusy(port int) (bool, error) {
	// For now, we'll assume runners are not busy by default
	return false, nil
}

func runnerDoneHandler(port int) {
	mu.Lock()
	defer mu.Unlock()

	if len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		log.Printf("Sending next submission from queue to code-runner on port %d.", port)
		go processSubmission(next, port)
	} else {
		log.Printf("No more submissions. Code-runner on port %d now idle.", port)
	}
}

func processSubmission(sub *PendingSubmission, port int) {
	result, err := sendToCodeRunner(sub, port)
	if err != nil {
		log.Printf("Error sending to Code-Runner on port %d: %v\n", port, err)
		runnerDoneHandler(port)
		return
	}
	log.Printf("Code-Runner on port %d response: result=%v\n", port, result.Status)

	apiURL := fmt.Sprintf("http://serve:5000/internalapi/judge/%d", sub.SubmissionID)

	requestBody, err := json.Marshal(result)
	if err != nil {
		log.Printf("Error marshaling result: %v\n", err)
		runnerDoneHandler(port)
		return
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(requestBody))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		runnerDoneHandler(port)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("INTERNAL_API_KEY")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to internal API: %v\n", err)
		runnerDoneHandler(port)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Internal API returned non-OK status: %d, body: %s\n", resp.StatusCode, string(body))
	} else {
		log.Println("Successfully sent result to internal API")
	}

	runnerDoneHandler(port)
}

func sendToCodeRunner(sub *PendingSubmission, port int) (*RunResponse, error) {
	payload, err := json.Marshal(sub)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal submission: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%d/run", port), bytes.NewReader(payload))
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
