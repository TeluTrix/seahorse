// Package transcode provides ffmpeg-based fixups for browser-incompatible
// media: re-encoding audio tracks no browser can decode natively (common in
// ripped Blu-ray/DVD media — AC3/DTS/E-AC3/TrueHD), and converting cached
// cover art to WebP for faster loading. Video streams are always copied
// untouched — this is never used for actual video transcoding/playback,
// which stays direct-play.
package transcode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

// probeTimeout is generous even though ffprobe only reads header/stream
// metadata (not the whole file) — it should never legitimately take this
// long, so hitting this timeout means something is actually wrong (e.g. an
// unreachable network mount), not just a big file.
const probeTimeout = 30 * time.Second

// remuxTimeout bounds a single audio remux. This is a stream copy for
// video (no re-encoding), but it still has to read and write the entire
// file, which can take a while for a large 4K file over slow/network
// storage — generous on purpose, but bounded so one problem file can't
// block scanning indefinitely.
const remuxTimeout = 60 * time.Minute

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

	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "stream=codec_name",
		"-select_streams", "a:0",
		videoPath,
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("ffprobe failed (or timed out after %s): %w", probeTimeout, err)
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

// probeDuration returns videoPath's duration in seconds, or 0 if it can't be
// determined (e.g. ffprobe missing/fails) — callers treat 0 as "no progress
// percentage available" rather than an error, since duration is only needed
// for the optional progress callback, not for the remux itself.
func probeDuration(videoPath string) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0
	}
	return seconds
}

// parseFFmpegTimestamp parses ffmpeg's "-progress" out_time value
// ("HH:MM:SS.ffffff") into total seconds.
func parseFFmpegTimestamp(s string) float64 {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)
	sec, _ := strconv.ParseFloat(parts[2], 64)
	return h*3600 + m*60 + sec
}

// watchProgress reads ffmpeg's "-progress pipe:1" key=value stream and
// reports percent complete (0-100) via onProgress as "out_time=" lines
// arrive, relative to totalSeconds. Returns once the stream is exhausted
// (ffmpeg exited or closed its stdout).
func watchProgress(r io.Reader, totalSeconds float64, onProgress func(percent float64)) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "out_time="):
			seconds := parseFFmpegTimestamp(strings.TrimPrefix(line, "out_time="))
			percent := seconds / totalSeconds * 100
			if percent > 100 {
				percent = 100
			}
			onProgress(percent)
		case line == "progress=end":
			onProgress(100)
		}
	}
}

// RemuxAudio produces a browser-playable copy of videoPath at RemuxedPath:
// the first video stream copied untouched, the first audio stream
// re-encoded to AAC. Subtitle streams are intentionally dropped — this
// app's subtitle features (internal/subtitles) always read from the
// original file, never the remuxed copy, so there's nothing to preserve
// here and doing so would risk a subtitle-codec-copy failure for no reason.
// A no-op if the remuxed copy already exists.
//
// If onProgress is non-nil and the source file's duration can be determined,
// it's called with the estimated percent complete (0-100) as ffmpeg reports
// progress. This is best-effort: if duration can't be determined, onProgress
// is simply never called.
//
// Writes to a temporary path first and only renames to the final RemuxedPath
// on success — otherwise a timed-out or killed run would leave a partial,
// broken file sitting at the "done" path, which future scans would then
// mistake for a completed remux and never retry.
func RemuxAudio(videoPath string, onProgress func(percent float64)) error {
	dest := RemuxedPath(videoPath)
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	// The temp path keeps dest's extension at the very end (rather than
	// appending ".tmp" after it) since ffmpeg picks its output container
	// format from the file extension — a ".mkv.tmp" file fails to mux at
	// all ("Unable to choose an output format").
	destExt := filepath.Ext(dest)
	tmpDest := strings.TrimSuffix(dest, destExt) + ".tmp" + destExt
	defer os.Remove(tmpDest) // best-effort cleanup on any non-success exit path

	ctx, cancel := context.WithTimeout(context.Background(), remuxTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-y",
		"-i", videoPath,
		"-map", "0:v:0",
		"-map", "0:a:0",
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		"-progress", "pipe:1",
		"-nostats",
		tmpDest,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	totalSeconds := 0.0
	if onProgress != nil {
		totalSeconds = probeDuration(videoPath)
	}

	progressDone := make(chan struct{})
	if onProgress != nil && totalSeconds > 0 {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return fmt.Errorf("could not attach to ffmpeg stdout: %w", err)
		}
		go func() {
			defer close(progressDone)
			watchProgress(stdout, totalSeconds, onProgress)
		}()
	} else {
		close(progressDone)
	}

	if err := cmd.Run(); err != nil {
		<-progressDone
		return fmt.Errorf("ffmpeg audio remux failed (or timed out after %s): %w: %s", remuxTimeout, err, stderr.String())
	}
	<-progressDone

	return os.Rename(tmpDest, dest)
}
