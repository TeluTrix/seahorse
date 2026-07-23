package subtitles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
)

var (
	availableOnce sync.Once
	available     bool
)

// Available reports whether ffprobe/ffmpeg are installed. Used only to
// extract subtitle streams already muxed into a video container — never for
// video transcoding/playback, which stays direct-play. Checked once and
// cached; if unavailable, embedded-subtitle discovery is silently skipped
// and external subtitle files still work normally.
func Available() bool {
	availableOnce.Do(func() {
		_, ffprobeErr := exec.LookPath("ffprobe")
		_, ffmpegErr := exec.LookPath("ffmpeg")
		available = ffprobeErr == nil && ffmpegErr == nil
	})
	return available
}

type EmbeddedStream struct {
	Index    int
	Language string
}

var textSubtitleCodecs = map[string]bool{
	"subrip":   true,
	"srt":      true,
	"ass":      true,
	"ssa":      true,
	"mov_text": true,
	"webvtt":   true,
	"text":     true,
}

// ProbeSubtitleStreams lists text-based subtitle streams muxed into
// videoPath. Bitmap subtitle formats (dvd_subtitle, hdmv_pgs_subtitle, ...)
// are skipped since they can't be converted to WebVTT text.
func ProbeSubtitleStreams(videoPath string) ([]EmbeddedStream, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "stream=index,codec_name:stream_tags=language",
		"-select_streams", "s",
		videoPath,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var parsed struct {
		Streams []struct {
			Index     int    `json:"index"`
			CodecName string `json:"codec_name"`
			Tags      struct {
				Language string `json:"language"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return nil, fmt.Errorf("could not parse ffprobe output: %w", err)
	}

	streams := make([]EmbeddedStream, 0, len(parsed.Streams))
	for _, s := range parsed.Streams {
		if !textSubtitleCodecs[s.CodecName] {
			continue
		}
		lang := s.Tags.Language
		if lang == "" {
			lang = "und"
		}
		streams = append(streams, EmbeddedStream{Index: s.Index, Language: lang})
	}
	return streams, nil
}

// ExtractToVTT extracts the subtitle stream at streamIndex from videoPath
// and writes it as WebVTT to destPath.
func ExtractToVTT(videoPath string, streamIndex int, destPath string) error {
	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", "webvtt",
		destPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg subtitle extraction failed: %w: %s", err, stderr.String())
	}
	return nil
}
