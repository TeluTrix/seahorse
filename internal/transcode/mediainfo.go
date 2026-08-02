package transcode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/TeluTrix/seahorse/internal/ffmpeg"
)

// mediaInfoProbeTimeout is generous even though ffprobe only reads
// header/stream metadata (not the whole file) — same reasoning as the
// probe timeouts in audio.go.
const mediaInfoProbeTimeout = 15 * time.Second

// MediaInfo is a read-only snapshot of a file's technical characteristics,
// for display only (e.g. an optional "Media Info" panel) — never used to
// decide anything about playback or remuxing.
type MediaInfo struct {
	Container     string `json:"container"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	BitrateKbps   int    `json:"bitrate_kbps,omitempty"`
	VideoCodec    string `json:"video_codec,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	AudioCodec    string `json:"audio_codec,omitempty"`
	AudioChannels int    `json:"audio_channels,omitempty"`
}

// ProbeMediaInfo reads videoPath's container/stream metadata via ffprobe.
// Returns an error if ffmpeg isn't installed or the probe fails — callers
// should treat this as "unavailable", not a hard failure.
func ProbeMediaInfo(videoPath string) (MediaInfo, error) {
	if !ffmpeg.Available() {
		return MediaInfo{}, fmt.Errorf("ffmpeg not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), mediaInfoProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return MediaInfo{}, fmt.Errorf("ffprobe failed (or timed out after %s): %w", mediaInfoProbeTimeout, err)
	}

	var parsed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Channels  int    `json:"channels"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return MediaInfo{}, fmt.Errorf("could not parse ffprobe output: %w", err)
	}

	info := MediaInfo{Container: parsed.Format.FormatName}
	if bps, err := strconv.ParseInt(parsed.Format.BitRate, 10, 64); err == nil {
		info.BitrateKbps = int(bps / 1000)
	}
	for _, stream := range parsed.Streams {
		switch stream.CodecType {
		case "video":
			if info.VideoCodec == "" {
				info.VideoCodec = stream.CodecName
				info.Width = stream.Width
				info.Height = stream.Height
			}
		case "audio":
			if info.AudioCodec == "" {
				info.AudioCodec = stream.CodecName
				info.AudioChannels = stream.Channels
			}
		}
	}

	// File size comes from the filesystem directly rather than ffprobe's
	// own format.size field, which some containers omit.
	if stat, err := os.Stat(videoPath); err == nil {
		info.FileSizeBytes = stat.Size()
	}

	return info, nil
}
