// Package transcode provides ffmpeg-based fixups for browser-incompatible
// media: re-encoding audio tracks no browser can decode natively (common in
// ripped Blu-ray/DVD media — AC3/DTS/E-AC3/TrueHD), and converting cached
// cover art to WebP for faster loading. Video streams are always copied
// untouched — this is never used for actual video transcoding/playback,
// which stays direct-play.
package transcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/TeluTrix/seahorse/internal/ffmpeg"
)

var browserCompatibleAudioCodecs = map[string]bool{
	"aac":       true,
	"mp3":       true,
	"opus":      true,
	"vorbis":    true,
	"flac":      true,
	"pcm_s16le": true,
	"pcm_u8":    true,
}

// RemuxedPath returns the deterministic sibling path used to cache an
// audio-fixed copy of videoPath, e.g. "Movie.mp4" -> "Movie.audiofix.mp4".
func RemuxedPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	base := strings.TrimSuffix(videoPath, ext)
	return base + ".audiofix" + ext
}

// NeedsAudioRemux reports whether videoPath's first audio stream uses a
// codec no browser can decode natively. Returns false (nothing to do) if
// there's no audio stream at all, or ffmpeg isn't installed.
func NeedsAudioRemux(videoPath string) (bool, error) {
	if !ffmpeg.Available() {
		return false, nil
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "stream=codec_name",
		"-select_streams", "a:0",
		videoPath,
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("ffprobe failed: %w", err)
	}

	var parsed struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return false, fmt.Errorf("could not parse ffprobe output: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return false, nil // no audio stream to fix
	}

	return !browserCompatibleAudioCodecs[parsed.Streams[0].CodecName], nil
}

// RemuxAudio produces a browser-playable copy of videoPath at RemuxedPath:
// the first video stream copied untouched, the first audio stream
// re-encoded to AAC. Subtitle streams are intentionally dropped — this
// app's subtitle features (internal/subtitles) always read from the
// original file, never the remuxed copy, so there's nothing to preserve
// here and doing so would risk a subtitle-codec-copy failure for no reason.
// A no-op if the remuxed copy already exists.
func RemuxAudio(videoPath string) error {
	dest := RemuxedPath(videoPath)
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	cmd := exec.Command("ffmpeg",
		"-y",
		"-i", videoPath,
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		dest,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg audio remux failed: %w: %s", err, stderr.String())
	}
	return nil
}
