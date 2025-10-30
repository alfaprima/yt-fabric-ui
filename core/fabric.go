package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// logProcessError logs errors to Process.log file
func logProcessError(pattern, model string, err error, stderr string) {
	logFile, fileErr := os.OpenFile("Process.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		fmt.Printf("Warning: Could not open Process.log: %v\n", fileErr)
		return
	}
	defer logFile.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logEntry := fmt.Sprintf("[%s] Pattern: %s, Model: %s\nError: %v\n", timestamp, pattern, model, err)
	if stderr != "" {
		logEntry += fmt.Sprintf("Stderr: %s\n", stderr)
	}
	logEntry += "---\n"

	if _, writeErr := logFile.WriteString(logEntry); writeErr != nil {
		fmt.Printf("Warning: Could not write to Process.log: %v\n", writeErr)
	}
}

// RunFabric runs the fabric command with the given pattern and model
func RunFabric(input, pattern, model string) (string, error) {
	var cmd *exec.Cmd
	fmt.Println("Running fabric with pattern:", pattern, "and model:", model)
	if model != "" && model != "default" {
		cmd = exec.Command("fabric", "--pattern", pattern, "--model", model)
	} else {
		cmd = exec.Command("fabric", "--pattern", pattern)
	}
	cmd.Stdin = strings.NewReader(input)

	output, err := cmd.CombinedOutput()
	if err != nil {
		stderr := string(output)
		logProcessError(pattern, model, err, stderr)

		// Provide more detailed error message
		errorMsg := fmt.Sprintf("error executing fabric pattern: %v", err)
		if stderr != "" {
			errorMsg = fmt.Sprintf("error executing fabric pattern: %v\nDetails: %s", err, stderr)
		}
		return "", fmt.Errorf("%s", errorMsg)
	}
	return string(output), nil
}

// RunFabricWithTrace runs the fabric command with the given pattern and model, and logs debug traces
func RunFabricWithTrace(input, pattern, model, videoDir string) (string, error) {
	var cmd *exec.Cmd
	fmt.Println("Running fabric with pattern:", pattern, "and model:", model)

	startTime := time.Now()

	if model != "" && model != "default" {
		cmd = exec.Command("fabric", "--pattern", pattern, "--model", model)
	} else {
		cmd = exec.Command("fabric", "--pattern", pattern)
	}
	cmd.Stdin = strings.NewReader(input)

	output, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	// Log the trace
	logTrace(videoDir, pattern, model, input, string(output), err, duration)

	if err != nil {
		stderr := string(output)
		logProcessError(pattern, model, err, stderr)

		// Provide more detailed error message
		errorMsg := fmt.Sprintf("error executing fabric pattern: %v", err)
		if stderr != "" {
			errorMsg = fmt.Sprintf("error executing fabric pattern: %v\nDetails: %s", err, stderr)
		}
		return "", fmt.Errorf("%s", errorMsg)
	}
	return string(output), nil
}

// logTrace logs debug traces of LLM interactions to a trace file in the video directory
func logTrace(videoDir, pattern, model, input, output string, err error, duration time.Duration) {
	if videoDir == "" {
		return
	}

	traceFile := fmt.Sprintf("%s/llm-trace.log", videoDir)
	logFile, fileErr := os.OpenFile(traceFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		fmt.Printf("Warning: Could not open trace file: %v\n", fileErr)
		return
	}
	defer logFile.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	separator := strings.Repeat("=", 80)

	logEntry := fmt.Sprintf("\n%s\n", separator)
	logEntry += fmt.Sprintf("[%s] LLM Interaction Trace\n", timestamp)
	logEntry += fmt.Sprintf("%s\n\n", separator)

	logEntry += fmt.Sprintf("Pattern: %s\n", pattern)
	logEntry += fmt.Sprintf("Model: %s\n", model)
	logEntry += fmt.Sprintf("Duration: %v\n\n", duration)

	logEntry += fmt.Sprintf("--- INPUT (length: %d chars) ---\n", len(input))
	logEntry += fmt.Sprintf("%s\n\n", input)

	if err != nil {
		logEntry += fmt.Sprintf("--- ERROR ---\n")
		logEntry += fmt.Sprintf("%v\n\n", err)
	}

	logEntry += fmt.Sprintf("--- OUTPUT (length: %d chars) ---\n", len(output))
	logEntry += fmt.Sprintf("%s\n\n", output)

	logEntry += fmt.Sprintf("%s\n\n", separator)

	if _, writeErr := logFile.WriteString(logEntry); writeErr != nil {
		fmt.Printf("Warning: Could not write to trace file: %v\n", writeErr)
	}
}

func ListPatterns() ([]string, error) {
	cmd := exec.Command("fabric", "-l")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error listing patterns: %v", err)
	}
	lines := strings.Split(string(output), "\n")
	patterns := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" && !strings.HasPrefix(line, "Patterns:") {
			patterns = append(patterns, strings.TrimSpace(line))
		}
	}
	return patterns, nil
}

func ListModels() ([]Model, error) {
	cmd := exec.Command("fabric", "-L")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error listing models: %v", err)
	}
	return CleanModels(string(output)), nil
}

// Model represents a model with its provider and name
type Model struct {
	Provider string
	Name     string
}

// ParseModels parses the output of the fabric -L command into a list of Models
func ParseModels(output string) ([]Model, error) {
	lines := strings.Split(output, "\n")
	var models []Model
	var currentProvider string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, ":") {
			currentProvider = strings.TrimSuffix(line, ":")
		} else if currentProvider != "" {
			models = append(models, Model{
				Provider: currentProvider,
				Name:     line,
			})
		}
	}
	return models, nil
}

// CleanModels takes the raw output from fabric -L and returns a cleaned list of Models
func CleanModels(output string) []Model {
	lines := strings.Split(output, "\n")
	var models []Model
	var currentProvider string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "[") {
			currentProvider = strings.TrimSuffix(line, ":")
		} else if currentProvider != "" {
			// Remove the index number and any leading/trailing whitespace
			parts := strings.SplitN(line, "]", 2)
			if len(parts) == 2 {
				modelName := strings.TrimSpace(parts[1])
				models = append(models, Model{
					Provider: currentProvider,
					Name:     modelName,
				})
			}
		}
	}
	return models
}

// PrintCleanModels prints the cleaned models in the desired format
func PrintCleanModels(models []Model) {
	for _, model := range models {
		fmt.Printf("{\n  provider: %s,\n  model: \"%s\"\n},\n", model.Provider, model.Name)
	}
}
