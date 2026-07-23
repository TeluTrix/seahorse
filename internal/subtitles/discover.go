package subtitles

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Track struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Source   string `json:"source"` // external | embedded
}

var languageNames = map[string]string{
	"en": "English", "eng": "English",
	"de": "German", "ger": "German", "deu": "German",
	"fr": "French", "fre": "French", "fra": "French",
	"es": "Spanish", "spa": "Spanish",
	"it": "Italian", "ita": "Italian",
	"ja": "Japanese", "jpn": "Japanese",
	"und": "Unknown",
}

func labelFor(lang string) string {
	if name, ok := languageNames[strings.ToLower(lang)]; ok {
		return name
	}
	return strings.ToUpper(lang)
}

// Discover finds all subtitle tracks available for a video: external
// .srt/.vtt/.ass files sharing its basename in the same folder, plus (if
// ffmpeg is installed) text-based subtitle streams muxed into the file.
func Discover(videoPath string) []Track {
	var tracks []Track

	dir := filepath.Dir(videoPath)
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	pattern := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(base) + `(?:\.([a-zA-Z]{2,4}))?\.(srt|vtt|ass)$`)

	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			matches := pattern.FindStringSubmatch(entry.Name())
			if matches == nil {
				continue
			}
			lang := matches[1]
			if lang == "" {
				lang = "und"
			}
			tracks = append(tracks, Track{
				ID:       "ext:" + entry.Name(),
				Label:    labelFor(lang),
				Language: strings.ToLower(lang),
				Source:   "external",
			})
		}
	}

	if Available() {
		streams, err := ProbeSubtitleStreams(videoPath)
		if err == nil {
			for _, st := range streams {
				tracks = append(tracks, Track{
					ID:       fmt.Sprintf("embedded-%d", st.Index),
					Label:    labelFor(st.Language),
					Language: strings.ToLower(st.Language),
					Source:   "embedded",
				})
			}
		}
	}

	return tracks
}
