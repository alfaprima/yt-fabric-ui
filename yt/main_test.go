package yt

import (
	"testing"
)

func TestGetVideoID(t *testing.T) {
	yt := &YT{}

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "standard youtube watch URL with https and www",
			url:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "standard youtube watch URL with http",
			url:      "http://www.youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "standard youtube watch URL without www",
			url:      "https://youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "youtube watch URL without protocol",
			url:      "youtube.com/watch?v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "shortened youtu.be URL",
			url:      "https://youtu.be/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "embed URL",
			url:      "https://www.youtube.com/embed/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "v/ URL",
			url:      "https://www.youtube.com/v/dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "URL with additional parameters",
			url:      "https://www.youtube.com/watch?v=dQw4w9WgXcQ&feature=youtu.be",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "URL with additional parameters before v",
			url:      "https://www.youtube.com/watch?feature=youtu.be&v=dQw4w9WgXcQ",
			expected: "dQw4w9WgXcQ",
		},
		{
			name:     "invalid URL format",
			url:      "invalid_url",
			expected: "",
		},
		{
			name:     "empty string",
			url:      "",
			expected: "",
		},
		{
			name:     "just the video ID (no valid prefix)",
			url:      "dQw4w9WgXcQ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := yt.GetVideoID(tt.url)
			if result != tt.expected {
				t.Errorf("GetVideoID(%q) = %v, want %v", tt.url, result, tt.expected)
			}
		})
	}
}
