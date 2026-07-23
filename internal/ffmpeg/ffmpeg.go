// Package ffmpeg provides a shared "is ffmpeg/ffprobe installed" check used
// by every feature that shells out to them (subtitle extraction, audio
// remuxing, cover image conversion) — never for video transcoding/playback,
// which stays direct-play.
package ffmpeg

import (
	"log/slog"
	"os/exec"
	"strings"
	"sync"
)

var (
	availableOnce sync.Once
	available     bool

	webpOnce      sync.Once
	webpSupported bool
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

// WebPSupported reports whether the installed ffmpeg has a WebP encoder.
// Some distro packages (e.g. certain RPM Fusion ffmpeg builds) ship without
// one at all, in which case every cover conversion would otherwise fail
// individually and log a warning per file. Checked once and cached, with a
// single warning logged here instead, so callers can just skip WebP
// conversion outright (falling back to keeping the source JPEG) rather than
// retrying and failing on every cover.
func WebPSupported() bool {
	webpOnce.Do(func() {
		if !Available() {
			return
		}
		out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
		if err != nil {
			return
		}
		webpSupported = strings.Contains(string(out), "webp")
		if !webpSupported {
			slog.Warn("ffmpeg has no WebP encoder available; covers will be cached as JPEG instead of WebP")
		}
	})
	return webpSupported
}
