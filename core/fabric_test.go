package core

import (
	"os/exec"
	"strings"
	"testing"
)

func TestRunFabricValidation(t *testing.T) {
	if _, err := exec.LookPath("fabric"); err != nil {
		t.Skip("skipping test; fabric command not found")
	}

	tests := []struct {
		name          string
		pattern       string
		model         string
		expectedError string
	}{
		{
			name:          "Valid pattern and valid model",
			pattern:       "summarize",
			model:         "gpt-3.5-turbo",
			expectedError: "",
		},
		{
			name:          "Valid pattern and default model",
			pattern:       "extract_wisdom",
			model:         "default",
			expectedError: "",
		},
		{
			name:          "Invalid pattern",
			pattern:       "; rm -rf /",
			model:         "gpt-3.5-turbo",
			expectedError: "invalid pattern",
		},
		{
			name:          "Invalid model",
			pattern:       "summarize",
			model:         "--version",
			expectedError: "invalid model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RunFabric("input data", tt.pattern, tt.model)

			if tt.expectedError == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.expectedError)
				} else if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error containing %q, got %v", tt.expectedError, err)
				}
			}
		})
	}
}
