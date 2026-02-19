package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FabricRunOptions struct {
	Temperature      *float64
	TopP             *float64
	PresencePenalty  *float64
	FrequencyPenalty *float64
	Raw              bool
}

var globalFabricRunOptions FabricRunOptions

func SetGlobalFabricRunOptions(opts FabricRunOptions) {
	globalFabricRunOptions = opts
}

func formatOptionalFloat(v *float64) string {
	if v == nil {
		return "<default>"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func buildFabricArgs(pattern, model string, opts FabricRunOptions) []string {
	args := []string{"--pattern", pattern}
	if model != "" && model != "default" {
		args = append(args, "--model", model)
	}

	if opts.Raw {
		args = append(args, "--raw")
		return args
	}

	if opts.Temperature != nil {
		args = append(args, "--temperature", strconv.FormatFloat(*opts.Temperature, 'f', -1, 64))
	}
	if opts.TopP != nil {
		args = append(args, "--topp", strconv.FormatFloat(*opts.TopP, 'f', -1, 64))
	}
	if opts.PresencePenalty != nil {
		args = append(args, "--presencepenalty", strconv.FormatFloat(*opts.PresencePenalty, 'f', -1, 64))
	}
	if opts.FrequencyPenalty != nil {
		args = append(args, "--frequencypenalty", strconv.FormatFloat(*opts.FrequencyPenalty, 'f', -1, 64))
	}

	return args
}

func detectServiceURL(model string) string {
	provider := strings.ToLower(strings.TrimSpace(model))
	if i := strings.Index(provider, "/"); i > 0 {
		provider = provider[:i]
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		envPath := filepath.Join(homeDir, ".config", "fabric", ".env")
		if content, readErr := os.ReadFile(envPath); readErr == nil {
			values := map[string]string{}
			for _, line := range strings.Split(string(content), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}
				k := strings.ToUpper(strings.TrimSpace(parts[0]))
				v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				values[k] = v
			}

			switch provider {
			case "openai", "gpt", "o1", "o3", "o4", "gpt-4", "gpt-4o", "gpt-5":
				if v := values["OPENAI_BASE_URL"]; v != "" {
					return v
				}
				if v := values["OPENAI_API_BASE"]; v != "" {
					return v
				}
				return "https://api.openai.com/v1"
			case "anthropic", "claude":
				if v := values["ANTHROPIC_BASE_URL"]; v != "" {
					return v
				}
				return "https://api.anthropic.com"
			case "groq":
				if v := values["GROQ_BASE_URL"]; v != "" {
					return v
				}
				if v := values["GROQ_API_BASE"]; v != "" {
					return v
				}
				return "https://api.groq.com/openai/v1"
			case "openrouter":
				if v := values["OPENROUTER_BASE_URL"]; v != "" {
					return v
				}
				return "https://openrouter.ai/api/v1"
			case "mistral":
				if v := values["MISTRAL_BASE_URL"]; v != "" {
					return v
				}
				return "https://api.mistral.ai/v1"
			}
		}
	}

	return ""
}

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
	fmt.Println("Running fabric with pattern:", pattern, "and model:", model)
	args := buildFabricArgs(pattern, model, globalFabricRunOptions)
	cmd := exec.Command("fabric", args...)
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
func RunFabricWithTrace(input, pattern, model, videoDir string) (string, string, error) {
	fmt.Println("Running fabric with pattern:", pattern, "and model:", model)

	startTime := time.Now()
	args := buildFabricArgs(pattern, model, globalFabricRunOptions)
	cmd := exec.Command("fabric", args...)
	cmd.Stdin = strings.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	stderrStr := stderr.String()
	duration := time.Since(startTime)

	// Log the trace
	payload := fmt.Sprintf("pattern=%s, model=%s, temperature=%s, top_p=%s, presence_penalty=%s, frequency_penalty=%s, raw=%t",
		pattern,
		model,
		formatOptionalFloat(globalFabricRunOptions.Temperature),
		formatOptionalFloat(globalFabricRunOptions.TopP),
		formatOptionalFloat(globalFabricRunOptions.PresencePenalty),
		formatOptionalFloat(globalFabricRunOptions.FrequencyPenalty),
		globalFabricRunOptions.Raw,
	)
	serviceURL := detectServiceURL(model)

	traceFileName, traceErr := logTrace(videoDir, pattern, model, input, output, stderrStr, err, duration, args, payload, serviceURL)
	if traceErr != nil {
		fmt.Printf("Warning: Could not write trace file: %v\n", traceErr)
	}

	if err != nil {
		logProcessError(pattern, model, err, stderrStr)

		// Provide more detailed error message
		errorMsg := fmt.Sprintf("error executing fabric pattern: %v", err)
		if stderrStr != "" {
			errorMsg = fmt.Sprintf("error executing fabric pattern: %v\nDetails: %s", err, stderrStr)
		}
		return "", traceFileName, fmt.Errorf("%s", errorMsg)
	}
	return output, traceFileName, nil
}

// logTrace logs debug traces of LLM interactions to a trace file in the video directory
func logTrace(videoDir, pattern, model, input, output, stderr string, err error, duration time.Duration, args []string, payload string, serviceURL string) (string, error) {
	if videoDir == "" {
		return "", nil
	}

	timestamp := time.Now().Format("20060102-150405.000")
	traceFileName := fmt.Sprintf("llm-trace-%s.log", timestamp)
	traceFile := filepath.Join(videoDir, traceFileName)

	logFile, fileErr := os.OpenFile(traceFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if fileErr != nil {
		return "", fileErr
	}
	defer logFile.Close()

	readableTimestamp := time.Now().Format("2006-01-02 15:04:05")
	separator := strings.Repeat("=", 80)

	logEntry := fmt.Sprintf("\n%s\n", separator)
	logEntry += fmt.Sprintf("[%s] LLM Interaction Trace\n", readableTimestamp)
	logEntry += fmt.Sprintf("%s\n\n", separator)

	logEntry += fmt.Sprintf("Pattern: %s\n", pattern)
	logEntry += fmt.Sprintf("Model: %s\n", model)
	logEntry += fmt.Sprintf("Duration: %v\n\n", duration)
	if len(args) > 0 {
		logEntry += fmt.Sprintf("Fabric Command: fabric %s\n", strings.Join(args, " "))
	}
	if serviceURL != "" {
		logEntry += fmt.Sprintf("Service URL: %s\n", serviceURL)
	}
	if payload != "" {
		logEntry += fmt.Sprintf("Request Payload: %s\n", payload)
	}
	logEntry += "\n"

	logEntry += fmt.Sprintf("--- INPUT (length: %d chars) ---\n", len(input))
	logEntry += fmt.Sprintf("%s\n\n", input)

	if err != nil {
		logEntry += fmt.Sprintf("--- ERROR ---\n")
		logEntry += fmt.Sprintf("%v\n\n", err)
	}

	if strings.TrimSpace(stderr) != "" {
		logEntry += fmt.Sprintf("--- WARNINGS / STDERR (length: %d chars) ---\n", len(stderr))
		logEntry += fmt.Sprintf("%s\n\n", stderr)
	}

	logEntry += fmt.Sprintf("--- OUTPUT (length: %d chars) ---\n", len(output))
	logEntry += fmt.Sprintf("%s\n\n", output)

	logEntry += fmt.Sprintf("%s\n\n", separator)

	if _, writeErr := logFile.WriteString(logEntry); writeErr != nil {
		return "", writeErr
	}

	return traceFileName, nil
}

func WriteWarningTrace(videoDir, pattern, model, message string) (string, error) {
	payload := fmt.Sprintf("pattern=%s, model=%s, warning=true", pattern, model)
	return logTrace(videoDir, pattern, model, "", message, "", nil, 0, nil, payload, detectServiceURL(model))
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

// ListConfiguredModels returns dynamic models from fabric filtered to providers
// that have API keys configured in ~/.config/fabric/.env.
func ListConfiguredModels() ([]Model, error) {
	models, err := ListModels()
	if err != nil {
		return nil, err
	}

	return FilterModelsByConfiguredProviders(models)
}

// FilterModelsByConfiguredProviders keeps only models from providers that have
// an API key configured in ~/.config/fabric/.env.
func FilterModelsByConfiguredProviders(models []Model) ([]Model, error) {

	configuredProviders, err := loadConfiguredProviders()
	if err != nil {
		return nil, err
	}

	if len(configuredProviders) == 0 {
		return []Model{}, nil
	}

	filtered := make([]Model, 0, len(models))
	for _, model := range models {
		provider := canonicalProvider(model.Provider)
		if provider != "" && configuredProviders[provider] {
			filtered = append(filtered, model)
		}
	}

	return filtered, nil
}

func loadConfiguredProviders() (map[string]bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve user home directory: %w", err)
	}

	envPath := filepath.Join(homeDir, ".config", "fabric", ".env")
	content, err := os.ReadFile(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", envPath, err)
	}

	providers := map[string]bool{}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.ToUpper(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		if value == "" {
			continue
		}

		switch key {
		case "OPENAI_API_KEY":
			providers["openai"] = true
		case "ANTHROPIC_API_KEY":
			providers["anthropic"] = true
		case "GOOGLE_API_KEY", "GEMINI_API_KEY", "GOOGLE_GENERATIVE_AI_API_KEY":
			providers["google"] = true
		case "GROQ_API_KEY":
			providers["groq"] = true
		case "MISTRAL_API_KEY":
			providers["mistral"] = true
		case "OPENROUTER_API_KEY":
			providers["openrouter"] = true
		case "AZURE_OPENAI_API_KEY":
			providers["azureopenai"] = true
		case "DEEPSEEK_API_KEY":
			providers["deepseek"] = true
		case "COHERE_API_KEY":
			providers["cohere"] = true
		case "PERPLEXITY_API_KEY":
			providers["perplexity"] = true
		case "XAI_API_KEY":
			providers["xai"] = true
		case "TOGETHER_API_KEY":
			providers["together"] = true
		case "FIREWORKS_API_KEY":
			providers["fireworks"] = true
		case "REPLICATE_API_TOKEN":
			providers["replicate"] = true
		case "HUGGINGFACE_API_KEY", "HF_TOKEN":
			providers["huggingface"] = true
		}
	}

	return providers, nil
}

func canonicalProvider(provider string) string {
	normalized := strings.ToLower(provider)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")

	switch {
	case strings.Contains(normalized, "openrouter"):
		return "openrouter"
	case strings.Contains(normalized, "azure") && strings.Contains(normalized, "openai"):
		return "azureopenai"
	case strings.Contains(normalized, "openai"):
		return "openai"
	case strings.Contains(normalized, "anthropic") || strings.Contains(normalized, "claude"):
		return "anthropic"
	case strings.Contains(normalized, "google") || strings.Contains(normalized, "gemini") || strings.Contains(normalized, "vertex"):
		return "google"
	case strings.Contains(normalized, "groq"):
		return "groq"
	case strings.Contains(normalized, "mistral"):
		return "mistral"
	case strings.Contains(normalized, "deepseek"):
		return "deepseek"
	case strings.Contains(normalized, "cohere"):
		return "cohere"
	case strings.Contains(normalized, "perplexity"):
		return "perplexity"
	case strings.Contains(normalized, "xai") || strings.Contains(normalized, "grok"):
		return "xai"
	case strings.Contains(normalized, "together"):
		return "together"
	case strings.Contains(normalized, "fireworks"):
		return "fireworks"
	case strings.Contains(normalized, "replicate"):
		return "replicate"
	case strings.Contains(normalized, "huggingface"):
		return "huggingface"
	default:
		return ""
	}
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
