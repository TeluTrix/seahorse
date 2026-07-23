// Package ffmpeg provides a shared "is ffmpeg/ffprobe installed" check used
// by every feature that shells out to them (subtitle extraction, audio
// remuxing, cover image conversion) — never for video transcoding/playback,
// which stays direct-play.
package ffmpeg

import (
	"os/exec"
	"sync"
)

var (
	availableOnce sync.Once
	available     bool
)

// Available reports whether ffprobe/ffmpeg are installed. Checked once and
// cached; callers should degrade gracefully (skip the feature, log a line)
// when it's false rather than fail hard.
func Available() bool {
	availableOnce.Do(func() {
		_, ffprobeErr := exec.LookPath("ffprobe")
		_, ffmpegErr := exec.LookPath("ffmpeg")
		available = ffprobeErr == nil && ffmpegErr == nil
	})
	return available
}
