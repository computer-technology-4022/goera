package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	//"time" // Uncomment for time limit testing
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line) // Split by whitespace
		if len(parts) == 2 {
			a, err1 := strconv.Atoi(parts[0])
			b, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil {
				// --- Simulate work for Time Limit Test (optional) ---
				// if a == 1000 { // Example condition to trigger delay
				//     time.Sleep(3 * time.Second)
				// }
				// --- End Simulation ---
				fmt.Println(a + b)
				return // Success
			}
		}
	}
	// If input is bad or conversion fails, exit non-zero (RuntimeError)
	fmt.Fprintln(os.Stderr, "Invalid input provided")
	os.Exit(1)
}
