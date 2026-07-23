package subtitles

import (
	"regexp"
	"strings"
)

var srtTimestampRegex = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})`)

// SRTToVTT converts SubRip subtitle text into WebVTT, the only format
// native <track> elements understand. VTT files pass through unchanged
// (aside from ensuring the required header is present).
func SRTToVTT(data []byte, alreadyVTT bool) []byte {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")

	if alreadyVTT {
		if !strings.HasPrefix(strings.TrimSpace(text), "WEBVTT") {
			text = "WEBVTT\n\n" + text
		}
		return []byte(text)
	}

	text = srtTimestampRegex.ReplaceAllString(text, "$1.$2")
	return []byte("WEBVTT\n\n" + text)
}
