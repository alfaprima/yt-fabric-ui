package core

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"testing"
)

func TestListPatterns(t *testing.T) {
	// Setup the test helper process mechanism
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() {
		execCommand = exec.Command
	}()

	tests := []struct {
		name          string
		mockOutput    string
		mockExitCode  string
		expected      []string
		expectedError bool
	}{
		{
			name:          "successful listing",
			mockOutput:    "Patterns:\npattern1\npattern2\npattern3",
			mockExitCode:  "0",
			expected:      []string{"pattern1", "pattern2", "pattern3"},
			expectedError: false,
		},
		{
			name:          "empty listing",
			mockOutput:    "Patterns:\n",
			mockExitCode:  "0",
			expected:      []string{},
			expectedError: false,
		},
		{
			name:          "error execution",
			mockOutput:    "error message",
			mockExitCode:  "1",
			expected:      nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pass mock data through environment variables
			os.Setenv("MOCK_OUTPUT", tt.mockOutput)
			os.Setenv("MOCK_EXIT_CODE", tt.mockExitCode)
			defer os.Unsetenv("MOCK_OUTPUT")
			defer os.Unsetenv("MOCK_EXIT_CODE")

			patterns, err := ListPatterns()

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}

			if !reflect.DeepEqual(patterns, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, patterns)
			}
		})
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// args look like: ["/path/to/binary", "-test.run=TestHelperProcess", "--", "fabric", "-l"]
	// The command we actually care about is at args[3:]
	// But we just care about returning the mocked output

	output := os.Getenv("MOCK_OUTPUT")
	exitCodeStr := os.Getenv("MOCK_EXIT_CODE")

	if exitCodeStr == "1" {
		fmt.Fprint(os.Stderr, output)
		os.Exit(1)
	}

	fmt.Fprint(os.Stdout, output)
	os.Exit(0)
}
