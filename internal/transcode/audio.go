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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/TeluTrix/seahorse/internal/ffmpeg"
	"golang.org/x/sync/singleflight"
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

// Options bundles the tunables for NeedsAudioRemux/RemuxAudio, sourced from
// config.Config so an operator can adjust them per-deployment (e.g. a longer
// RemuxTimeout for large files over slow network storage) without a rebuild.
type Options struct {
	// ProbeTimeout bounds ffprobe calls (codec/duration checks). Generous
	// even though ffprobe only reads header/stream metadata — it should
	// never legitimately take long, so hitting this means something is
	// genuinely wrong (e.g. an unreachable network mount), not just a big
	// file.
	ProbeTimeout time.Duration
	// RemuxTimeout bounds a single audio remux. This is a stream copy for
	// video (no re-encoding), but it still has to read and write the entire
	// file, which can take a while for a large 4K file over slow/network
	// storage — bounded so one problem file can't block scanning
	// indefinitely.
	RemuxTimeout time.Duration
	// AudioBitrate is the AAC bitrate used for the re-encoded audio track,
	// e.g. "192k".
	AudioBitrate string
}

// RemuxedPath returns the deterministic sibling path used to cache an
// audio-fixed copy of videoPath, e.g. "Movie.mp4" -> "Movie.audiofix.mp4".
func RemuxedPath(videoPath string) string {
	ext := filepath.Ext(videoPath)
	base := strings.TrimSuffix(videoPath, ext)
	return base + ".audiofix" + ext
}

// generatedFileSuffix matches the tail of any path this package generates
// itself: RemuxedPath's "*.audiofix.<ext>" and AudioTrackPath's
// "*.audiofix.a<N>.<ext>". Both keep the original filename as their prefix
// (so "Show S01E01.mp4" produces "Show S01E01.audiofix.a1.mp4"), which means
// a naive scan for source video files would otherwise pick these up as
// distinct episodes/movies of their own.
var generatedFileSuffix = regexp.MustCompile(`\.audiofix(\.a\d+)?\.[^.]+$`)

// IsGeneratedFile reports whether name (a filename or path) is a cache file
// produced by RemuxedPath or AudioTrackPath, rather than an original source
// video. Callers that walk a directory for video files must skip these —
// otherwise every cached audio-track variant re-appears as a "new" file on
// the next scan, producing duplicate movie/episode rows for the same title.
func IsGeneratedFile(name string) bool {
	return generatedFileSuffix.MatchString(name)
}

// audioStream describes one audio stream as reported by ffprobe. Index is
// the stream's absolute index within the container (suitable for ffmpeg's
// "-map 0:<index>"), not its position among audio streams specifically.
type audioStream struct {
	Index     int
	CodecName string
	// Language is the stream's language tag, lowercased, or "und" if absent
	// — same fallback subtitles.ProbeSubtitleStreams uses (internal/subtitles/ffmpeg.go).
	// This is only a Go-side display/logging fallback: nothing synthesizes
	// a "und" tag into the ffmpeg output itself (see RemuxAudio).
	Language string
}

// probeAudioStreams lists every audio stream in videoPath, in container
// order. Shared by NeedsAudioRemux (which only needs CodecName) and
// RemuxAudio (which also needs Index/Language to build its ffmpeg command),
// so there's exactly one ffprobe invocation shape to maintain.
func probeAudioStreams(videoPath string, timeout time.Duration) ([]audioStream, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "stream=index,codec_name:stream_tags=language",
		"-select_streams", "a",
		videoPath,
	)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffprobe failed (or timed out after %s): %w", timeout, err)
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

	streams := make([]audioStream, 0, len(parsed.Streams))
	for _, s := range parsed.Streams {
		lang := strings.ToLower(s.Tags.Language)
		if lang == "" {
			lang = "und"
		}
		streams = append(streams, audioStream{Index: s.Index, CodecName: s.CodecName, Language: lang})
	}
	return streams, nil
}

// NeedsAudioRemux reports whether videoPath's single audio stream uses a
// codec no browser can decode natively — the common case for ripped media
// (a single AC3/DTS track). Returns false if there's no audio stream, more
// than one (see ListAudioTracks/EnsureAudioTrack below for why a file with
// multiple audio tracks is handled differently, lazily, rather than by an
// eager scan-time fix here), or ffmpeg isn't installed.
func NeedsAudioRemux(videoPath string, opts Options) (bool, error) {
	if !ffmpeg.Available() {
		return false, nil
	}

	streams, err := probeAudioStreams(videoPath, opts.ProbeTimeout)
	if err != nil {
		return false, err
	}
	if len(streams) != 1 {
		return false, nil
	}
	return !browserCompatibleAudioCodecs[streams[0].CodecName], nil
}

// probeDuration returns videoPath's duration in seconds, or 0 if it can't be
// determined (e.g. ffprobe missing/fails) — callers treat 0 as "no progress
// percentage available" rather than an error, since duration is only needed
// for the optional progress callback, not for the remux itself.
func probeDuration(videoPath string, timeout time.Duration) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
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

// RemuxAudio produces a browser-playable copy of videoPath at RemuxedPath,
// for the single-incompatible-audio-stream case NeedsAudioRemux detects (the
// video stream copied untouched, the audio stream re-encoded to AAC). Its
// argument-building loop is written generally over every audio stream
// found (compatible ones stream-copied, incompatible ones re-encoded, each
// stream's language tag explicitly carried over via "-map_metadata:s:a:i"
// since relying on ffmpeg's implicit per-stream metadata propagation isn't
// reliable once a stream goes through the encoder rather than a pure copy),
// but in practice NeedsAudioRemux only ever returns true for exactly one
// audio stream, so the loop always runs once. A file with genuinely
// multiple audio streams is handled by ListAudioTracks/EnsureAudioTrack
// instead — merging every track into one file here wouldn't actually help,
// since browsers have no reliable way to choose among multiple audio
// streams muxed into a single file (see EnsureAudioTrack's doc comment).
//
// Subtitle streams are intentionally dropped — this app's subtitle features
// (internal/subtitles) always read from the original file, never the
// remuxed copy, so there's nothing to preserve here and doing so would risk
// a subtitle-codec-copy failure for no reason. A no-op if the remuxed copy
// already exists.
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
func RemuxAudio(videoPath string, opts Options, onProgress func(percent float64)) error {
	dest := RemuxedPath(videoPath)
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	streams, err := probeAudioStreams(videoPath, opts.ProbeTimeout)
	if err != nil {
		return fmt.Errorf("could not probe audio streams: %w", err)
	}

	// The temp path keeps dest's extension at the very end (rather than
	// appending ".tmp" after it) since ffmpeg picks its output container
	// format from the file extension — a ".mkv.tmp" file fails to mux at
	// all ("Unable to choose an output format").
	destExt := filepath.Ext(dest)
	tmpDest := strings.TrimSuffix(dest, destExt) + ".tmp" + destExt
	defer os.Remove(tmpDest) // best-effort cleanup on any non-success exit path

	ctx, cancel := context.WithTimeout(context.Background(), opts.RemuxTimeout)
	defer cancel()

	args := []string{"-y", "-i", videoPath, "-map", "0:v:0", "-c:v", "copy"}
	for i, s := range streams {
		out := strconv.Itoa(i)
		args = append(args, "-map", fmt.Sprintf("0:%d", s.Index))
		if browserCompatibleAudioCodecs[s.CodecName] {
			args = append(args, "-c:a:"+out, "copy")
		} else {
			args = append(args, "-c:a:"+out, "aac", "-b:a:"+out, opts.AudioBitrate)
		}
		// Every audio stream is mapped in file order with none skipped, so
		// output position i always corresponds to input audio-stream
		// position i — this alignment is what makes "0:s:a:i" on the
		// right-hand side correct.
		args = append(args, "-map_metadata:s:a:"+out, "0:s:a:"+out)
	}
	args = append(args, "-progress", "pipe:1", "-nostats", tmpDest)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	totalSeconds := 0.0
	if onProgress != nil {
		totalSeconds = probeDuration(videoPath, opts.ProbeTimeout)
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
		return fmt.Errorf("ffmpeg audio remux failed (or timed out after %s): %w: %s", opts.RemuxTimeout, err, stderr.String())
	}
	<-progressDone

	return os.Rename(tmpDest, dest)
}

// AudioTrackInfo describes one audio track available for playback, for the
// frontend to offer as a language choice.
type AudioTrackInfo struct {
	// Index is the stream's absolute index within the container — also its
	// identity in the API/URL (see EnsureAudioTrack/AudioTrackPath).
	Index    int
	Language string
}

// ListAudioTracks returns every audio stream in videoPath. Returns nil (not
// an error) if ffmpeg isn't installed or probing fails — like subtitle
// discovery, this is supplementary information, never something that should
// break playback.
func ListAudioTracks(videoPath string, opts Options) []AudioTrackInfo {
	if !ffmpeg.Available() {
		return nil
	}
	streams, err := probeAudioStreams(videoPath, opts.ProbeTimeout)
	if err != nil {
		return nil
	}
	tracks := make([]AudioTrackInfo, 0, len(streams))
	for _, s := range streams {
		tracks = append(tracks, AudioTrackInfo{Index: s.Index, Language: s.Language})
	}
	return tracks
}

// AudioTrackPath returns the deterministic per-stream cache path used to
// isolate one audio stream (by its absolute container index, as reported by
// ListAudioTracks) into its own single-audio-track file, e.g. "Movie.mp4"
// + index 2 -> "Movie.audiofix.a2.mp4".
func AudioTrackPath(videoPath string, streamIndex int) string {
	ext := filepath.Ext(videoPath)
	base := strings.TrimSuffix(videoPath, ext)
	return fmt.Sprintf("%s.audiofix.a%d%s", base, streamIndex, ext)
}

// EnsureAudioTrack makes sure AudioTrackPath(videoPath, streamIndex) exists,
// generating it on demand if not (a no-op if already cached): the video
// stream copied untouched, plus exactly the one requested audio stream
// (stream-copied if already browser-compatible, re-encoded to AAC if not).
//
// This — a dedicated single-audio-track file per language — is what actually
// lets a viewer pick a language out of a multi-audio-track source file.
// Unlike subtitles, where a browser's native menu lets you choose among
// several <track> elements attached to one <video>, there is no working
// equivalent for audio: HTMLMediaElement.audioTracks/videoTracks are
// unimplemented in mainstream Chrome and Firefox despite being spec'd
// (confirmed empirically against a real, current Chromium build — not just
// undocumented or assumed), so a browser given a single file with several
// muxed audio streams has no reliable way to switch between them, and just
// plays whichever one it likes. "Pick a language" therefore has to mean
// "serve a different, single-track file" instead of a client-side
// track-selection API — see internal/api/stream_handler.go's serveVideoFile,
// which resolves a "?track=" query parameter to this function.
//
// Deliberately lazy (generated the first time a specific track is actually
// requested) rather than eager during scanning, mirroring how this
// codebase's embedded-subtitle extraction already works
// (internal/api/subtitles_handler.go's serveSubtitleTrack) — most viewers
// never touch the language picker, and a multi-track file can have several
// tracks nobody ever selects.
//
// A <video> element commonly issues more than one overlapping request for a
// freshly-selected track (an initial probe plus a ranged fetch, or a
// buffering-ahead request racing the first one) — without trackGroup below,
// each landed here concurrently, saw dest not yet exist, and started its own
// ffmpeg process writing the same tmpDest path, corrupting or truncating
// each other's output and failing most of the requests with a 500. Keying
// on dest serializes concurrent callers for the *same* track onto a single
// ffmpeg run that they all share the result of, while different tracks (or
// different videos) still generate fully in parallel.
var trackGroup singleflight.Group

func EnsureAudioTrack(videoPath string, streamIndex int, opts Options) error {
	dest := AudioTrackPath(videoPath, streamIndex)
	if _, err := os.Stat(dest); err == nil {
		return nil
	}

	_, err, _ := trackGroup.Do(dest, func() (any, error) {
		// Re-check now that we hold this key's turn: another caller may have
		// already generated dest while we were waiting to enter Do.
		if _, err := os.Stat(dest); err == nil {
			return nil, nil
		}

		streams, err := probeAudioStreams(videoPath, opts.ProbeTimeout)
		if err != nil {
			return nil, fmt.Errorf("could not probe audio streams: %w", err)
		}
		ordinal := -1
		var codec string
		for i, s := range streams {
			if s.Index == streamIndex {
				ordinal = i
				codec = s.CodecName
				break
			}
		}
		if ordinal == -1 {
			return nil, fmt.Errorf("no audio stream with index %d in %s", streamIndex, videoPath)
		}

		destExt := filepath.Ext(dest)
		tmpDest := strings.TrimSuffix(dest, destExt) + ".tmp" + destExt
		defer os.Remove(tmpDest)

		ctx, cancel := context.WithTimeout(context.Background(), opts.RemuxTimeout)
		defer cancel()

		args := []string{
			"-y", "-i", videoPath,
			"-map", "0:v:0", "-c:v", "copy",
			"-map", fmt.Sprintf("0:%d", streamIndex),
			"-map_metadata:s:a:0", fmt.Sprintf("0:s:a:%d", ordinal),
		}
		if browserCompatibleAudioCodecs[codec] {
			args = append(args, "-c:a:0", "copy")
		} else {
			args = append(args, "-c:a:0", "aac", "-b:a:0", opts.AudioBitrate)
		}
		args = append(args, tmpDest)

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("ffmpeg audio track extraction failed (or timed out after %s): %w: %s", opts.RemuxTimeout, err, stderr.String())
		}
		return nil, os.Rename(tmpDest, dest)
	})
	return err
}
