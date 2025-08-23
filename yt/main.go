package yt

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/andybalholm/brotli"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type YT struct {
	apiKey  string
	service *youtube.Service
}

type Video struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Channel    string   `json:"channel"`
	Transcript string   `json:"transcript"`
	Comments   []string `json:"comments"`
	Duration   int      `json:"duration"`
	URL        string   `json:"url"`
}

func NewYT(apiKey string) *YT {
	var service *youtube.Service
	var err error
	if apiKey == "" {
		service = nil
	} else {
		service, err = youtube.NewService(context.Background(), option.WithAPIKey(apiKey))
		if err != nil {
			log.Fatalf("Error creating YouTube client: %v", err)
		}
	}
	return &YT{apiKey: apiKey, service: service}
}

func (y *YT) GetVideoInfo(url string) (*Video, error) {
	fmt.Println("Getting video info for", url)
	videoID := y.GetVideoID(url)
	if videoID == "" {
		return nil, fmt.Errorf("invalid YouTube URL")
	}

	fmt.Printf("Debug: Extracted video ID: %s\n", videoID)

	output := &Video{
		ID: videoID,
	}

	fmt.Printf("Debug: About to call getVideoDetails for video ID: %s\n", videoID)
	videoDetails, err := y.getVideoDetails(videoID)
	if err != nil {
		fmt.Printf("Error getVideoDetails: %v\n", err)
		// Don't return error - getVideoDetails now handles missing transcripts gracefully
		// Create a minimal video object with empty transcript
		fmt.Printf("Debug: Returning minimal video object due to getVideoDetails error\n")
		return &Video{
			ID:         videoID,
			Title:      fmt.Sprintf("Video %s", videoID),
			Channel:    "",
			Transcript: "",
			URL:        url,
		}, nil
	}
	fmt.Printf("Debug: getVideoDetails succeeded, transcript length: %d\n", len(videoDetails["transcript"]))
	output.Transcript = videoDetails["transcript"]
	output.Title = videoDetails["title"]
	output.Channel = videoDetails["channel"]
	fmt.Printf("Debug: Final video object - Title: '%s', Channel: '%s', Transcript length: %d\n", output.Title, output.Channel, len(output.Transcript))
	return output, nil
}

func (y *YT) GetVideoID(url string) string {
	pattern := `(?:https?:\/\/)?(?:www\.)?(?:youtube\.com\/(?:[^\/\n\s]+\/\S+\/|(?:v|e(?:mbed)?)\/|\S*?[?&]v=)|youtu\.be\/)([a-zA-Z0-9_-]{11})`
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(url)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func (y *YT) getVideoDetails(videoID string) (map[string]string, error) {
	url := "https://www.youtube.com/watch?v=" + videoID
	resp, err := robustHTTPGet(url)
	if err != nil {
		fmt.Printf("Error getVideoDetails: %v\n", err)
		return nil, err
	}

	// Debug: Check what we received
	fmt.Printf("Debug: Received HTML response length: %d bytes\n", len(resp))
	if len(resp) > 0 {
		// Show first 500 characters to see what we got
		preview := resp
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Printf("Debug: HTML preview: %s\n", preview)
	}

	doc := soup.HTMLParse(resp)

	// Try to get transcript, but don't fail if it's not available
	transcript, err := y.getTranscript(doc)
	if err != nil {
		fmt.Printf("Warning: Could not get transcript: %v\n", err)
		transcript = "" // Set empty transcript instead of failing
	}

	title := getTitle(doc)
	channel := getCreator(doc)

	// If we couldn't extract the title, use the video ID as fallback
	if title == "" {
		title = fmt.Sprintf("Video %s", videoID)
		fmt.Printf("Warning: Could not extract video title, using fallback: %s\n", title)
	}

	return map[string]string{
		"transcript": transcript,
		"title":      title,
		"channel":    channel,
	}, nil
}

func (y *YT) getTranscript(doc soup.Root) (string, error) {
	// Get video ID from the current context
	videoID := y.extractVideoIDFromHTML(doc)
	if videoID == "" {
		return "", fmt.Errorf("failed to extract video ID from HTML")
	}

	// Try multiple methods in order of reliability
	methods := []func(string, soup.Root) (string, error){
		y.getTranscriptFromYouTubeAPI,
		y.getTranscriptFromHTML,
		y.getTranscriptFromInnerTube,
	}

	var lastErr error
	for i, method := range methods {
		fmt.Printf("Trying transcript method %d...\n", i+1)
		transcript, err := method(videoID, doc)
		if err == nil && transcript != "" {
			fmt.Printf("Successfully extracted transcript using method %d\n", i+1)
			return transcript, nil
		}
		lastErr = err
		fmt.Printf("Method %d failed: %v\n", i+1, err)
	}

	return "", fmt.Errorf("all transcript extraction methods failed, last error: %v", lastErr)
}

func (y *YT) getTranscriptFromYouTubeAPI(videoID string, doc soup.Root) (string, error) {
	// This method uses a direct approach similar to youtube-transcript-api
	// Extract transcript data from the initial page load
	scriptTags := doc.FindAll("script")

	// Look for ytInitialPlayerResponse which contains transcript info
	for _, scriptTag := range scriptTags {
		text := scriptTag.Text()
		if strings.Contains(text, "ytInitialPlayerResponse") && strings.Contains(text, "captions") {
			// Extract the ytInitialPlayerResponse JSON
			regex := regexp.MustCompile(`ytInitialPlayerResponse\s*=\s*({.+?});`)
			match := regex.FindStringSubmatch(text)
			if len(match) > 1 {
				var playerResponse map[string]interface{}
				if err := json.Unmarshal([]byte(match[1]), &playerResponse); err != nil {
					continue
				}

				// Navigate to captions
				if captions, ok := playerResponse["captions"].(map[string]interface{}); ok {
					if renderer, ok := captions["playerCaptionsTracklistRenderer"].(map[string]interface{}); ok {
						if tracks, ok := renderer["captionTracks"].([]interface{}); ok && len(tracks) > 0 {
							// Get the first available track
							if track, ok := tracks[0].(map[string]interface{}); ok {
								if baseURL, ok := track["baseUrl"].(string); ok {
									fmt.Printf("Found transcript URL in ytInitialPlayerResponse: %s\n", baseURL)
									return y.fetchAndParseTranscript(baseURL, videoID)
								}
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("transcript not found in ytInitialPlayerResponse")
}

func (y *YT) getTranscriptFromHTML(videoID string, doc soup.Root) (string, error) {
	scriptTags := doc.FindAll("script")
	for _, scriptTag := range scriptTags {
		if strings.Contains(scriptTag.Text(), "captionTracks") {
			regex := regexp.MustCompile(`"captionTracks":(\[.*?\])`)
			match := regex.FindStringSubmatch(scriptTag.Text())
			if len(match) > 1 {
				var captionTracks []struct {
					BaseURL string `json:"baseUrl"`
				}
				json.Unmarshal([]byte(match[1]), &captionTracks)
				if len(captionTracks) > 0 {
					transcriptURL := captionTracks[0].BaseURL
					fmt.Printf("Fetching transcript from HTML: %s\n", transcriptURL)
					transcriptResp, err := robustHTTPGet(transcriptURL)

					if err != nil {
						fmt.Printf("Error getTranscript robustHTTPGet(transcriptURL): %v\n", err)
						return "", err
					}

					// Save raw XML transcript for debugging
					rawXMLPath := fmt.Sprintf("data/videos/%s/transcript_raw.xml", videoID)
					err = saveToFile(rawXMLPath, transcriptResp)
					if err != nil {
						fmt.Printf("Warning: Could not save raw XML: %v\n", err)
					} else {
						fmt.Printf("Debug: Raw XML saved to http://localhost:8090/debug/%s/transcript_raw.xml\n", videoID)
					}

					transcript, err := unmarshalTranscript([]byte(transcriptResp))
					if err != nil {
						fmt.Printf("Error getTranscript unmarshalTranscript: %v\n", err)
						return "", err
					}
					var transcriptLines []string
					fmt.Printf("Debug: Processing %d transcript segments in HTML method\n", len(transcript.Texts))
					for i, track := range transcript.Texts {
						originalText := track.Value
						cleanedText := strings.ReplaceAll(track.Value, "&#39;", "'")
						cleanedText = strings.ReplaceAll(cleanedText, "&quot;", "\"")
						cleanedText = strings.ReplaceAll(cleanedText, "&amp;", "&")
						cleanedText = strings.ReplaceAll(cleanedText, "&lt;", "<")
						cleanedText = strings.ReplaceAll(cleanedText, "&gt;", ">")

						if i < 5 { // Debug first 5 segments
							fmt.Printf("Debug HTML segment %d: original='%s', cleaned='%s', trimmed='%s'\n", i, originalText, cleanedText, strings.TrimSpace(cleanedText))
						}

						if strings.TrimSpace(cleanedText) != "" {
							transcriptLines = append(transcriptLines, cleanedText)
						}
					}

					if len(transcriptLines) == 0 {
						return "", fmt.Errorf("no valid transcript text found in HTML method")
					}

					cleanedTranscript := strings.Join(transcriptLines, " ")

					// Save processed transcript for debugging
					processedPath := fmt.Sprintf("data/videos/%s/transcript_processed.txt", videoID)
					err = saveToFile(processedPath, cleanedTranscript)
					if err != nil {
						fmt.Printf("Warning: Could not save processed transcript: %v\n", err)
					} else {
						fmt.Printf("Debug: Processed transcript saved to http://localhost:8090/debug/%s/transcript_processed.txt\n", videoID)
					}

					fmt.Printf("Successfully extracted transcript with %d segments, total length: %d characters (HTML method)\n", len(transcriptLines), len(cleanedTranscript))
					return cleanedTranscript, nil
				}
			}
		}
	}
	return "", fmt.Errorf("transcript not found in HTML")
}

func (y *YT) getTranscriptFromInnerTube(videoID string, doc soup.Root) (string, error) {
	// Extract API key from HTML
	apiKey, err := y.extractInnerTubeAPIKey(doc)
	if err != nil {
		return "", fmt.Errorf("failed to extract API key: %v", err)
	}

	// Call InnerTube API
	innerTubeData, err := y.callInnerTubeAPI(videoID, apiKey)
	if err != nil {
		return "", fmt.Errorf("InnerTube API call failed: %v", err)
	}

	// Extract transcript URL from InnerTube response
	transcriptURL, err := y.extractTranscriptURLFromInnerTube(innerTubeData)
	if err != nil {
		return "", fmt.Errorf("failed to extract transcript URL from InnerTube: %v", err)
	}

	// Fetch and parse transcript
	fmt.Printf("Fetching transcript from InnerTube API: %s\n", transcriptURL)
	transcriptResp, err := robustHTTPGet(transcriptURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch transcript: %v", err)
	}

	// Save raw XML transcript for debugging
	rawXMLPath := fmt.Sprintf("data/videos/%s/transcript_raw.xml", videoID)
	err = saveToFile(rawXMLPath, transcriptResp)
	if err != nil {
		fmt.Printf("Warning: Could not save raw XML: %v\n", err)
	} else {
		fmt.Printf("Debug: Raw XML saved to http://localhost:8090/debug/%s/transcript_raw.xml\n", videoID)
	}

	// Debug: Show raw XML content
	fmt.Printf("Debug: Raw transcript XML length: %d bytes\n", len(transcriptResp))
	if len(transcriptResp) > 0 {
		// Show first 1000 characters to see the XML structure
		preview := transcriptResp
		if len(preview) > 1000 {
			preview = preview[:1000] + "..."
		}
		fmt.Printf("Debug: Raw XML preview: %s\n", preview)
	}

	transcript, err := unmarshalTranscript([]byte(transcriptResp))
	if err != nil {
		return "", fmt.Errorf("failed to parse transcript: %v", err)
	}

	var transcriptLines []string
	fmt.Printf("Debug: Processing %d transcript segments in InnerTube method\n", len(transcript.Texts))
	for i, track := range transcript.Texts {
		originalText := track.Value
		cleanedText := strings.ReplaceAll(track.Value, "&#39;", "'")
		cleanedText = strings.ReplaceAll(cleanedText, "&quot;", "\"")
		cleanedText = strings.ReplaceAll(cleanedText, "&amp;", "&")
		cleanedText = strings.ReplaceAll(cleanedText, "&lt;", "<")
		cleanedText = strings.ReplaceAll(cleanedText, "&gt;", ">")

		if i < 5 { // Debug first 5 segments
			fmt.Printf("Debug InnerTube segment %d: original='%s', cleaned='%s', trimmed='%s'\n", i, originalText, cleanedText, strings.TrimSpace(cleanedText))
		}

		if strings.TrimSpace(cleanedText) != "" {
			transcriptLines = append(transcriptLines, cleanedText)
		}
	}

	if len(transcriptLines) == 0 {
		return "", fmt.Errorf("no valid transcript text found in InnerTube method")
	}

	cleanedTranscript := strings.Join(transcriptLines, " ")

	// Save processed transcript for debugging
	processedPath := fmt.Sprintf("data/videos/%s/transcript_processed.txt", videoID)
	err = saveToFile(processedPath, cleanedTranscript)
	if err != nil {
		fmt.Printf("Warning: Could not save processed transcript: %v\n", err)
	} else {
		fmt.Printf("Debug: Processed transcript saved to http://localhost:8090/debug/%s/transcript_processed.txt\n", videoID)
	}

	fmt.Printf("Successfully extracted transcript with %d segments, total length: %d characters (InnerTube method)\n", len(transcriptLines), len(cleanedTranscript))
	return cleanedTranscript, nil
}

func (y *YT) fetchAndParseTranscript(transcriptURL string, videoID string) (string, error) {
	fmt.Printf("Fetching transcript from: %s\n", transcriptURL)

	// Fetch the transcript XML
	transcriptResp, err := robustHTTPGet(transcriptURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch transcript: %v", err)
	}

	// Save raw XML transcript for debugging
	rawXMLPath := fmt.Sprintf("data/videos/%s/transcript_raw.xml", videoID)
	err = saveToFile(rawXMLPath, transcriptResp)
	if err != nil {
		fmt.Printf("Warning: Could not save raw XML: %v\n", err)
	} else {
		fmt.Printf("Debug: Raw XML saved to http://localhost:8090/debug/%s/transcript_raw.xml\n", videoID)
	}

	// Parse the XML transcript
	transcript, err := unmarshalTranscript([]byte(transcriptResp))
	if err != nil {
		return "", fmt.Errorf("failed to parse transcript XML: %v", err)
	}

	if len(transcript.Texts) == 0 {
		return "", fmt.Errorf("transcript is empty")
	}

	// Convert to text
	var transcriptLines []string
	fmt.Printf("Debug: Processing %d transcript segments in YouTube API method\n", len(transcript.Texts))
	for i, track := range transcript.Texts {
		originalText := track.Value
		cleanedText := strings.ReplaceAll(track.Value, "&#39;", "'")
		cleanedText = strings.ReplaceAll(cleanedText, "&quot;", "\"")
		cleanedText = strings.ReplaceAll(cleanedText, "&amp;", "&")
		cleanedText = strings.ReplaceAll(cleanedText, "&lt;", "<")
		cleanedText = strings.ReplaceAll(cleanedText, "&gt;", ">")

		if i < 5 { // Debug first 5 segments
			fmt.Printf("Debug YouTube API segment %d: original='%s', cleaned='%s', trimmed='%s'\n", i, originalText, cleanedText, strings.TrimSpace(cleanedText))
		}

		if strings.TrimSpace(cleanedText) != "" {
			transcriptLines = append(transcriptLines, cleanedText)
		}
	}

	if len(transcriptLines) == 0 {
		return "", fmt.Errorf("no valid transcript text found")
	}

	cleanedTranscript := strings.Join(transcriptLines, " ")

	// Save processed transcript for debugging
	processedPath := fmt.Sprintf("data/videos/%s/transcript_processed.txt", videoID)
	err = saveToFile(processedPath, cleanedTranscript)
	if err != nil {
		fmt.Printf("Warning: Could not save processed transcript: %v\n", err)
	} else {
		fmt.Printf("Debug: Processed transcript saved to http://localhost:8090/debug/%s/transcript_processed.txt\n", videoID)
	}

	fmt.Printf("Successfully extracted transcript with %d segments, total length: %d characters (YouTube API method)\n", len(transcriptLines), len(cleanedTranscript))
	return cleanedTranscript, nil
}

func (y *YT) extractInnerTubeAPIKey(doc soup.Root) (string, error) {
	scriptTags := doc.FindAll("script")

	// Multiple patterns YouTube uses for the API key
	patterns := []string{
		`"INNERTUBE_API_KEY":\s*"([a-zA-Z0-9_-]+)"`,
		`'INNERTUBE_API_KEY':\s*'([a-zA-Z0-9_-]+)'`,
		`INNERTUBE_API_KEY:\s*"([a-zA-Z0-9_-]+)"`,
		`"innertubeApiKey":\s*"([a-zA-Z0-9_-]+)"`,
		`'innertubeApiKey':\s*'([a-zA-Z0-9_-]+)'`,
		`"key":\s*"([a-zA-Z0-9_-]+)".*"context"`,
		`ytInitialData.*"key":\s*"([a-zA-Z0-9_-]+)"`,
	}

	for _, scriptTag := range scriptTags {
		text := scriptTag.Text()

		// Try each pattern
		for _, pattern := range patterns {
			regex := regexp.MustCompile(pattern)
			match := regex.FindStringSubmatch(text)
			if len(match) > 1 {
				fmt.Printf("Found API key using pattern: %s\n", pattern)
				return match[1], nil
			}
		}
	}

	// If no API key found, let's debug what we have
	fmt.Printf("Debug: Searching for API key patterns in %d script tags\n", len(scriptTags))
	for i, scriptTag := range scriptTags {
		text := scriptTag.Text()
		if strings.Contains(text, "API") || strings.Contains(text, "key") {
			// Show a snippet of scripts that contain "API" or "key"
			snippet := text
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			fmt.Printf("Script %d contains API/key: %s\n", i, snippet)
		}
	}

	return "", fmt.Errorf("INNERTUBE_API_KEY not found with any pattern")
}

func (y *YT) extractVideoIDFromHTML(doc soup.Root) string {
	scriptTags := doc.FindAll("script")
	for _, scriptTag := range scriptTags {
		text := scriptTag.Text()
		if strings.Contains(text, "videoId") {
			regex := regexp.MustCompile(`"videoId":\s*"([a-zA-Z0-9_-]{11})"`)
			match := regex.FindStringSubmatch(text)
			if len(match) > 1 {
				return match[1]
			}
		}
	}
	return ""
}

func (y *YT) callInnerTubeAPI(videoID, apiKey string) (map[string]interface{}, error) {
	url := fmt.Sprintf("https://www.youtube.com/youtubei/v1/player?key=%s", apiKey)

	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "ANDROID",
				"clientVersion": "20.10.38",
			},
		},
		"videoId": videoID,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; SM-G973F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return result, nil
}

func (y *YT) extractTranscriptURLFromInnerTube(data map[string]interface{}) (string, error) {
	// Navigate through the nested JSON structure
	captions, ok := data["captions"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("captions not found in response")
	}

	renderer, ok := captions["playerCaptionsTracklistRenderer"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("playerCaptionsTracklistRenderer not found")
	}

	captionTracks, ok := renderer["captionTracks"].([]interface{})
	if !ok || len(captionTracks) == 0 {
		return "", fmt.Errorf("captionTracks not found or empty")
	}

	// Get the first available caption track
	firstTrack, ok := captionTracks[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid caption track format")
	}

	baseURL, ok := firstTrack["baseUrl"].(string)
	if !ok {
		return "", fmt.Errorf("baseUrl not found in caption track")
	}

	return baseURL, nil
}

func getTitle(doc soup.Root) string {
	titleElement := doc.Find("title")
	if titleElement.Error != nil {
		fmt.Printf("Warning: Could not find title element: %v\n", titleElement.Error)
		return ""
	}
	title := titleElement.Text()
	title = strings.TrimSuffix(title, " - YouTube")
	return title
}

func getCreator(doc soup.Root) string {
	scriptTags := doc.FindAll("script")
	for _, scriptTag := range scriptTags {
		if strings.Contains(scriptTag.Text(), "ownerChannelName") {
			regex := regexp.MustCompile(`"ownerChannelName":"(.*?)"`)
			match := regex.FindStringSubmatch(scriptTag.Text())
			if len(match) > 1 {
				return match[1]
			}
		}
	}
	return ""
}

func (y *YT) getComments(videoID string) []string {
	var comments []string
	call := y.service.CommentThreads.List([]string{"snippet", "replies"}).VideoId(videoID).TextFormat("plainText").MaxResults(100)
	response, err := call.Do()
	if err != nil {
		log.Fatalf("getComments Failed to fetch comments: %v", err)
		return comments
	}

	for _, item := range response.Items {
		topLevelComment := item.Snippet.TopLevelComment.Snippet.TextDisplay
		comments = append(comments, topLevelComment)

		if item.Replies != nil {
			for _, reply := range item.Replies.Comments {
				replyText := reply.Snippet.TextDisplay
				comments = append(comments, "    - "+replyText)
			}
		}
	}
	return comments
}

func parseDuration(durationStr string) (int, error) {
	matches := regexp.MustCompile(`(?i)PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`).FindStringSubmatch(durationStr)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration string: %s", durationStr)
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])

	return hours*60 + minutes + seconds/60, nil
}

func (y *YT) getVideoDuration(videoID string) (int, error) {
	videoResponse, err := y.service.Videos.List([]string{"contentDetails"}).Id(videoID).Do()
	if err != nil {
		return 0, fmt.Errorf("error getting video details: %v", err)
	}
	durationStr := videoResponse.Items[0].ContentDetails.Duration
	return parseDuration(durationStr)
}

type Transcript struct {
	XMLName xml.Name `xml:"transcript"`
	Texts   []Text   `xml:"text"`
}

type TimedText struct {
	XMLName xml.Name `xml:"timedtext"`
	Texts   []Text   `xml:"text"`
}

type TimedTextV3 struct {
	XMLName xml.Name `xml:"timedtext"`
	Format  string   `xml:"format,attr"`
	Body    Body     `xml:"body"`
}

type Body struct {
	Paragraphs []Paragraph `xml:"p"`
}

type Paragraph struct {
	Time     string    `xml:"t,attr"`
	Duration string    `xml:"d,attr"`
	Segments []Segment `xml:"s"`
}

type Segment struct {
	Time  string `xml:"t,attr"`
	Value string `xml:",chardata"`
}

type Text struct {
	Start string `xml:"start,attr"`
	Dur   string `xml:"dur,attr"`
	Value string `xml:",chardata"`
}

func unmarshalTranscript(xmlData []byte) (*Transcript, error) {
	fmt.Printf("Debug unmarshalTranscript: XML data length: %d bytes\n", len(xmlData))

	// Show first 500 characters of XML for debugging
	if len(xmlData) > 0 {
		preview := string(xmlData)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Printf("Debug unmarshalTranscript: XML preview: %s\n", preview)
	}

	// First try to parse as <transcript> format
	var transcript Transcript
	err := xml.Unmarshal(xmlData, &transcript)
	if err == nil {
		fmt.Printf("Debug unmarshalTranscript: Successfully parsed as <transcript> format with %d texts\n", len(transcript.Texts))
		return &transcript, nil
	}
	fmt.Printf("Debug unmarshalTranscript: Failed to parse as <transcript>: %v\n", err)

	// Try to parse as <timedtext format="3"> format first (more specific)
	var timedTextV3 TimedTextV3
	err = xml.Unmarshal(xmlData, &timedTextV3)
	if err == nil && len(timedTextV3.Body.Paragraphs) > 0 {
		fmt.Printf("Debug unmarshalTranscript: Successfully parsed as <timedtext format=\"3\"> with %d paragraphs\n", len(timedTextV3.Body.Paragraphs))
	} else {
		fmt.Printf("Debug unmarshalTranscript: Failed to parse as <timedtext format=\"3\"> or no paragraphs found: %v\n", err)

		// If that fails, try to parse as basic <timedtext> format
		var timedText TimedText
		err = xml.Unmarshal(xmlData, &timedText)
		if err == nil {
			fmt.Printf("Debug unmarshalTranscript: Successfully parsed as <timedtext> format with %d texts\n", len(timedText.Texts))
			// Convert TimedText to Transcript format
			transcript = Transcript{
				XMLName: xml.Name{Local: "transcript"},
				Texts:   timedText.Texts,
			}
			return &transcript, nil
		}
		fmt.Printf("Debug unmarshalTranscript: Failed to parse as <timedtext>: %v\n", err)

		return nil, fmt.Errorf("failed to parse as transcript, timedtext, or timedtext v3 formats: %v", err)
	}

	// Convert TimedTextV3 to Transcript format
	var texts []Text
	fmt.Printf("Debug: Found %d paragraphs in TimedTextV3\n", len(timedTextV3.Body.Paragraphs))

	// Debug: Show first few paragraphs structure
	for i := 0; i < len(timedTextV3.Body.Paragraphs) && i < 5; i++ {
		paragraph := timedTextV3.Body.Paragraphs[i]
		fmt.Printf("Debug: Paragraph %d has %d segments, time='%s', duration='%s'\n", i, len(paragraph.Segments), paragraph.Time, paragraph.Duration)

		// Show all segments for first few paragraphs
		for j, segment := range paragraph.Segments {
			fmt.Printf("Debug: Paragraph %d, Segment %d: time='%s', value='%s', trimmed='%s'\n", i, j, segment.Time, segment.Value, strings.TrimSpace(segment.Value))
		}
	}

	for i, paragraph := range timedTextV3.Body.Paragraphs {
		if len(paragraph.Segments) > 0 {
			// Handle paragraphs with segments - collect all segment text for this paragraph
			var paragraphText strings.Builder
			for _, segment := range paragraph.Segments {
				if strings.TrimSpace(segment.Value) != "" {
					paragraphText.WriteString(segment.Value)
				}
			}

			// Add the complete paragraph text as a single entry
			if paragraphText.Len() > 0 {
				texts = append(texts, Text{
					Start: paragraph.Time,
					Dur:   paragraph.Duration,
					Value: paragraphText.String(),
				})
				if i < 5 { // Debug first 5 paragraphs
					fmt.Printf("Debug: Added paragraph %d text: '%s'\n", i, paragraphText.String())
				}
			}
		} else {
			// Handle paragraphs with direct text content (no segments)
			// We need to extract the text content from the paragraph itself
			// This requires a different approach since the XML structure varies

			// For now, let's try to extract any text content from the paragraph
			// We'll need to parse the raw XML to get the direct text content
			paragraphText := extractTextFromParagraph(xmlData, paragraph.Time)
			if paragraphText != "" {
				fmt.Printf("Debug: Paragraph %d direct text: '%s'\n", i, paragraphText)
				texts = append(texts, Text{
					Start: paragraph.Time,
					Dur:   paragraph.Duration,
					Value: paragraphText,
				})
			}
		}
	}

	fmt.Printf("Debug: Converted TimedTextV3 to %d text segments\n", len(texts))

	transcript = Transcript{
		XMLName: xml.Name{Local: "transcript"},
		Texts:   texts,
	}

	return &transcript, nil
}

// extractTextFromParagraph extracts direct text content from a paragraph element
func extractTextFromParagraph(xmlData []byte, timeAttr string) string {
	// Convert to string for regex processing
	xmlString := string(xmlData)

	// Create a regex pattern to find the paragraph with the specific time attribute
	// and extract its direct text content (not from nested <s> elements)
	pattern := fmt.Sprintf(`<p[^>]*t="%s"[^>]*>([^<]*)</p>`, regexp.QuoteMeta(timeAttr))
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(xmlString)

	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// If no direct text found, try a more flexible pattern that captures content between <p> tags
	// but excludes nested <s> elements
	pattern2 := fmt.Sprintf(`<p[^>]*t="%s"[^>]*>(.*?)</p>`, regexp.QuoteMeta(timeAttr))
	re2 := regexp.MustCompile(pattern2)
	match2 := re2.FindStringSubmatch(xmlString)

	if len(match2) > 1 {
		content := match2[1]
		// Remove any nested <s> tags and their content since we handle those separately
		sTagPattern := `<s[^>]*>.*?</s>`
		sRe := regexp.MustCompile(sTagPattern)
		content = sRe.ReplaceAllString(content, "")

		// Clean up any remaining tags and whitespace
		content = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(content, "")
		content = strings.TrimSpace(content)

		if content != "" {
			return content
		}
	}

	return ""
}

// robustHTTPGet performs HTTP GET with retry logic, proper headers, and error handling
func robustHTTPGet(url string) (string, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	maxRetries := 3
	baseDelay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("failed to create request: %v", err)
		}

		// Add more realistic browser headers to avoid detection
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
		req.Header.Set("Sec-Fetch-Dest", "document")
		req.Header.Set("Sec-Fetch-Mode", "navigate")
		req.Header.Set("Sec-Fetch-Site", "none")
		req.Header.Set("Sec-Fetch-User", "?1")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		if strings.Contains(url, "youtube.com") {
			req.Header.Set("Referer", "https://www.youtube.com/")
		}

		resp, err := client.Do(req)
		if err != nil {
			if attempt == maxRetries {
				return "", fmt.Errorf("failed after %d attempts: %v", maxRetries+1, err)
			}
			// Calculate delay with exponential backoff and jitter
			multiplier := 1
			for i := 0; i < attempt; i++ {
				multiplier *= 2
			}
			delay := time.Duration(float64(baseDelay) * float64(multiplier) * (0.5 + rand.Float64()*0.5))
			fmt.Printf("Request failed (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			if attempt == maxRetries {
				return "", fmt.Errorf("HTTP error after %d attempts: %d %s", maxRetries+1, resp.StatusCode, resp.Status)
			}
			// Calculate delay with exponential backoff and jitter
			multiplier := 1
			for i := 0; i < attempt; i++ {
				multiplier *= 2
			}
			delay := time.Duration(float64(baseDelay) * float64(multiplier) * (0.5 + rand.Float64()*0.5))
			fmt.Printf("HTTP error %d (attempt %d/%d), retrying in %v\n", resp.StatusCode, attempt+1, maxRetries+1, delay)
			time.Sleep(delay)
			continue
		}

		// Debug: Check response headers
		contentEncoding := resp.Header.Get("Content-Encoding")
		fmt.Printf("Debug: Content-Encoding header: '%s'\n", contentEncoding)
		fmt.Printf("Debug: Content-Type header: '%s'\n", resp.Header.Get("Content-Type"))
		fmt.Printf("Debug: Content-Length header: '%s'\n", resp.Header.Get("Content-Length"))

		// Handle decompression if needed
		var reader io.Reader = resp.Body
		if contentEncoding == "gzip" {
			fmt.Printf("Debug: Attempting gzip decompression\n")
			gzipReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				if attempt == maxRetries {
					return "", fmt.Errorf("failed to create gzip reader after %d attempts: %v", maxRetries+1, err)
				}
				// Calculate delay with exponential backoff and jitter
				multiplier := 1
				for i := 0; i < attempt; i++ {
					multiplier *= 2
				}
				delay := time.Duration(float64(baseDelay) * float64(multiplier) * (0.5 + rand.Float64()*0.5))
				fmt.Printf("Failed to create gzip reader (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
				time.Sleep(delay)
				continue
			}
			defer gzipReader.Close()
			reader = gzipReader
		} else if contentEncoding == "br" {
			fmt.Printf("Debug: Attempting Brotli decompression\n")
			// Import brotli locally to avoid auto-formatter issues
			brotliReader := brotli.NewReader(resp.Body)
			reader = brotliReader
		} else if contentEncoding == "deflate" {
			fmt.Printf("Debug: Deflate compression detected but not supported - reading as-is\n")
		} else if contentEncoding != "" {
			fmt.Printf("Debug: Unknown compression '%s' - reading as-is\n", contentEncoding)
		} else {
			fmt.Printf("Debug: No compression detected\n")
		}

		body, err := io.ReadAll(reader)
		if err != nil {
			if attempt == maxRetries {
				return "", fmt.Errorf("failed to read response after %d attempts: %v", maxRetries+1, err)
			}
			// Calculate delay with exponential backoff and jitter
			multiplier := 1
			for i := 0; i < attempt; i++ {
				multiplier *= 2
			}
			delay := time.Duration(float64(baseDelay) * float64(multiplier) * (0.5 + rand.Float64()*0.5))
			fmt.Printf("Failed to read response (attempt %d/%d), retrying in %v: %v\n", attempt+1, maxRetries+1, delay, err)
			time.Sleep(delay)
			continue
		}

		return string(body), nil
	}

	return "", fmt.Errorf("unexpected error: should not reach here")
}

// saveToFile saves content to a file, creating directories as needed
func saveToFile(filePath, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}

	// Write content to file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", filePath, err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %v", filePath, err)
	}

	return nil
}
