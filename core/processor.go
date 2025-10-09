package core

import (
	"fabric-agents/yt"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

type Processor struct {
	logger   *slog.Logger
	filesDir string
	yt       *yt.YT
}

func NewProcessor(logger *slog.Logger, filesDir string, yt *yt.YT) *Processor {
	return &Processor{logger: logger, filesDir: filesDir, yt: yt}
}

// FetchVideo fetches a video and returns the video directory
func (p *Processor) FetchVideo(videoLink string) (string, error) {
	p.logger.Info("Fetching video", "link", videoLink)
	videoID := p.yt.GetVideoID(videoLink)
	p.logger.Debug("Video ID", "videoID", videoID)

	os.MkdirAll(filepath.Join(p.filesDir, videoID), 0755)

	dataPath := filepath.Join(p.filesDir, videoID, "data.json")
	if _, err := os.Stat(dataPath); err == nil {
		// Transcript already exists, return the existing videoDir
		return videoID, nil
	}

	video, err := p.yt.GetVideoInfo(videoLink)
	if err != nil {
		p.logger.Error("Failed to get video info", "error", err)
		return "", fmt.Errorf("failed to get video info: %v", err)
	}

	SaveVideo(*video, p.filesDir)
	return video.ID, nil
}

func (p *Processor) ProcessVideo(videoID string, model string, pattern string) (string, yt.Video, error) {
	p.logger.Info("Processing video", "videoID", videoID, "model", model, "pattern", pattern)
	video, err := LoadVideo(videoID, p.filesDir)
	if err != nil {
		return "", yt.Video{}, fmt.Errorf("failed to load video: %v", err)
	}

	// Check if transcript is available
	if video.Transcript == "" {
		p.logger.Warn("No transcript available for video", "videoID", videoID)
		output := fmt.Sprintf("# %s\n\n**Channel:** %s\n\n**Note:** No transcript was available for this video. Unable to process with pattern '%s'.\n\nThis could be because:\n- The video doesn't have captions/subtitles\n- The captions are auto-generated and not accessible\n- The video has restricted access to transcripts\n\nYou may want to try processing a different video that has available transcripts.", video.Title, video.Channel, pattern)
		SaveVideoFabricOutput(videoID, output, pattern, model, p.filesDir)
		return output, *video, nil
	}

	// Get the video directory path for trace logging
	videoDir := filepath.Join(p.filesDir, videoID)

	// Use RunFabricWithTrace to log debug traces
	output, err := RunFabricWithTrace(video.Transcript, pattern, model, videoDir)
	if err != nil {
		p.logger.Error("Failed to run fabric", "error", err)
		return "", yt.Video{}, fmt.Errorf("failed to run fabric: %v", err)
	}

	SaveVideoFabricOutput(videoID, output, pattern, model, p.filesDir)
	return output, *video, nil
}
