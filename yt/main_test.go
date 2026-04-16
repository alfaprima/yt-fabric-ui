package yt

import (
	"regexp"
	"testing"
)

var globalRegex = regexp.MustCompile(`"captionTracks":(\[.*?\])`)

func BenchmarkRegexInsideLoop(b *testing.B) {
	scriptTexts := []string{
		"some other text without caption",
		`"captionTracks":[{"baseUrl":"http://example.com"}]`,
		"more text",
		`"captionTracks":[{"baseUrl":"http://example2.com"}]`,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, text := range scriptTexts {
			regex := regexp.MustCompile(`"captionTracks":(\[.*?\])`)
			regex.FindStringSubmatch(text)
		}
	}
}

func BenchmarkRegexOutsideLoop(b *testing.B) {
	scriptTexts := []string{
		"some other text without caption",
		`"captionTracks":[{"baseUrl":"http://example.com"}]`,
		"more text",
		`"captionTracks":[{"baseUrl":"http://example2.com"}]`,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, text := range scriptTexts {
			globalRegex.FindStringSubmatch(text)
		}
	}
}
