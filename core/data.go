package core

import (
	"encoding/json"
	"fabric-agents/yt"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LoadVideos loads all videos from the data directory
func LoadVideos(dataDir string) ([]yt.Video, error) {
	files, err := os.ReadDir(dataDir)
	if err != nil {
		return nil, err
	}

	type videoWithAddedAt struct {
		video   yt.Video
		addedAt time.Time
	}

	var videosWithAddedAt []videoWithAddedAt
	for _, file := range files {
		if file.IsDir() {
			video, err := LoadVideo(file.Name(), dataDir)
			if err != nil {
				return nil, err
			}
			if video != nil {
				addedAt := time.Time{}

				dataJSONPath := filepath.Join(dataDir, file.Name(), "data.json")
				if info, err := os.Stat(dataJSONPath); err == nil {
					addedAt = info.ModTime()
				} else if info, err := file.Info(); err == nil {
					addedAt = info.ModTime()
				}

				videosWithAddedAt = append(videosWithAddedAt, videoWithAddedAt{
					video:   *video,
					addedAt: addedAt,
				})
			}
		}
	}

	sort.Slice(videosWithAddedAt, func(i, j int) bool {
		return videosWithAddedAt[i].addedAt.After(videosWithAddedAt[j].addedAt)
	})

	videos := make([]yt.Video, 0, len(videosWithAddedAt))
	for _, v := range videosWithAddedAt {
		videos = append(videos, v.video)
	}

	return videos, nil
}

func SaveVideo(video yt.Video, dataDir string) error {
	videoDir := filepath.Join(dataDir, video.ID)
	os.MkdirAll(videoDir, 0755)
	transcriptPath := filepath.Join(videoDir, "data.json")
	videoJSON, err := json.Marshal(video)
	if err != nil {
		return err
	}

	return os.WriteFile(transcriptPath, videoJSON, 0644)
}

func SaveVideoFabricOutput(videoID string, output string, summary string, model string, dataDir string) error {
	videoDir := filepath.Join(dataDir, videoID)
	os.MkdirAll(videoDir, 0755)
	outputPath := filepath.Join(videoDir, fmt.Sprintf("%s-%s.md", summary, model))
	return os.WriteFile(outputPath, []byte(output), 0644)
}

func LoadVideo(videoID string, dataDir string) (*yt.Video, error) {
	videoDir := filepath.Join(dataDir, videoID)
	transcriptPath := filepath.Join(videoDir, "data.json")
	videoJSON, err := os.ReadFile(transcriptPath)
	if err != nil {
		return nil, nil
	}

	var video yt.Video
	err = json.Unmarshal(videoJSON, &video)
	if err != nil {
		return nil, err
	}

	return &video, nil
}

func LoadVideoSummary(videoID string, dataDir string, summaryFileName string) (string, error) {
	videoDir := filepath.Join(dataDir, videoID)
	summaryPath := filepath.Join(videoDir, summaryFileName)
	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		return "", err
	}
	return string(summary), nil
}

func LoadVideoFiles(videoID string, dataDir string) ([]string, error) {
	videoDir := filepath.Join(dataDir, videoID)
	files, err := os.ReadDir(videoDir)
	if err != nil {
		return nil, err
	}

	var filePaths []string
	for _, file := range files {
		if !file.IsDir() {
			filePaths = append(filePaths, file.Name())
		}
	}
	return filePaths, nil
}

func DeleteVideo(videoID string, dataDir string) error {
	videoDir := filepath.Join(dataDir, videoID)
	return os.RemoveAll(videoDir)
}

func LoadPatterns() ([]string, error) {
	patterns, err := os.ReadFile("data/patterns.txt")
	if err != nil {
		return nil, err
	}
	return strings.Split(string(patterns), "\n"), nil
}

func LoadModels() ([]Model, error) {
	models, err := os.ReadFile("data/models.txt")
	if err != nil {
		return nil, err
	}
	modelLines := strings.Split(string(models), "\n")
	var modelList []Model
	for _, modelLine := range modelLines {
		modelLine = strings.TrimSpace(modelLine)
		if modelLine == "" {
			continue
		}

		modelParts := strings.Split(modelLine, "/")
		if len(modelParts) == 2 {
			modelList = append(modelList, Model{
				Provider: modelParts[0],
				Name:     modelParts[1],
			})
		} else {
			// Fallback for plain model lines without provider prefix.
			modelList = append(modelList, Model{
				Provider: "",
				Name:     modelLine,
			})
		}
	}
	return modelList, nil
}

func LoadModelNames() ([]string, error) {
	content, err := os.ReadFile("data/models.txt")
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var modelNames []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		modelNames = append(modelNames, line)
	}

	return modelNames, nil
}

func SaveModelNames(modelNames []string) error {
	cleaned := make([]string, 0, len(modelNames))
	seen := map[string]bool{}

	for _, model := range modelNames {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		cleaned = append(cleaned, model)
	}

	content := strings.Join(cleaned, "\n")
	if content != "" {
		content += "\n"
	}

	return os.WriteFile("data/models.txt", []byte(content), 0644)
}

func RefreshModelNamesFromConfiguredProviders() ([]string, error) {
	models, err := ListConfiguredModels()
	if err != nil {
		return nil, err
	}

	modelNames := make([]string, 0, len(models))
	for _, model := range models {
		if strings.TrimSpace(model.Name) == "" {
			continue
		}

		provider := strings.TrimSpace(model.Provider)
		if provider != "" {
			modelNames = append(modelNames, fmt.Sprintf("%s/%s", provider, model.Name))
			continue
		}

		modelNames = append(modelNames, model.Name)
	}

	if err := SaveModelNames(modelNames); err != nil {
		return nil, err
	}

	return modelNames, nil
}
