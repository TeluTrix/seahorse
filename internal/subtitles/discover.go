package subtitles

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/TeluTrix/seahorse/internal/ffmpeg"
)

type Track struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Language string `json:"language"`
	Source   string `json:"source"` // external | embedded
}

// languageEntry maps a language to its ISO 639-1 code (for the <track
// srclang> attribute), plus the words/abbreviations that identify it inside
// a filename or an ffprobe language tag. fullWords are whole language names
// (safe to match anywhere in a filename); abbrevs are 3-4 letter codes,
// which are only trusted either in the strict ".xx.srt" suffix position or,
// for the whole-filename scan, after every full-word candidate across all
// languages has already been checked (see detectLanguage).
var languageEntries = []struct {
	code      string
	label     string
	fullWords []string
	abbrevs   []string
}{
	{"en", "English", []string{"english"}, []string{"eng"}},
	{"de", "German", []string{"german", "deutsch"}, []string{"ger", "deu"}},
	{"fr", "French", []string{"french", "francais", "français"}, []string{"fre", "fra"}},
	{"es", "Spanish", []string{"spanish", "espanol", "español", "castellano"}, []string{"spa"}},
	{"it", "Italian", []string{"italian"}, []string{"ita"}},
	{"pt", "Portuguese", []string{"portuguese"}, []string{"por"}},
	{"nl", "Dutch", []string{"dutch"}, []string{"dut", "nld"}},
	{"ru", "Russian", []string{"russian"}, []string{"rus"}},
	{"zh", "Chinese", []string{"chinese", "mandarin", "cantonese"}, []string{"chi", "zho"}},
	{"ja", "Japanese", []string{"japanese"}, []string{"jpn"}},
	{"ko", "Korean", []string{"korean"}, []string{"kor"}},
	{"ar", "Arabic", []string{"arabic"}, []string{"ara"}},
	{"hi", "Hindi", []string{"hindi"}, []string{"hin"}},
	{"ta", "Tamil", []string{"tamil"}, []string{"tam"}},
	{"te", "Telugu", []string{"telugu"}, []string{"tel"}},
	{"tr", "Turkish", []string{"turkish"}, []string{"tur"}},
	{"pl", "Polish", []string{"polish"}, []string{"pol"}},
	{"sv", "Swedish", []string{"swedish"}, []string{"swe"}},
	{"fi", "Finnish", []string{"finnish"}, []string{"fin"}},
	{"no", "Norwegian", []string{"norwegian"}, []string{"nor", "nob", "nno"}},
	{"da", "Danish", []string{"danish"}, []string{"dan"}},
	{"el", "Greek", []string{"greek"}, []string{"ell", "gre"}},
	{"hu", "Hungarian", []string{"hungarian"}, []string{"hun"}},
	{"cs", "Czech", []string{"czech"}, []string{"cze", "ces"}},
	{"sk", "Slovak", []string{"slovak"}, []string{"slo", "slk"}},
	{"ro", "Romanian", []string{"romanian"}, []string{"rum", "ron"}},
	{"bg", "Bulgarian", []string{"bulgarian"}, []string{"bul"}},
	{"uk", "Ukrainian", []string{"ukrainian"}, []string{"ukr"}},
	{"hr", "Croatian", []string{"croatian"}, []string{"hrv"}},
	{"sr", "Serbian", []string{"serbian"}, []string{"srp"}},
	{"sl", "Slovenian", []string{"slovenian", "slovene"}, []string{"slv"}},
	{"vi", "Vietnamese", []string{"vietnamese"}, []string{"vie"}},
	{"th", "Thai", []string{"thai"}, []string{"tha"}},
	{"id", "Indonesian", []string{"indonesian"}, []string{"ind"}},
	{"ms", "Malay", []string{"malay"}, []string{"msa"}},
	{"he", "Hebrew", []string{"hebrew"}, []string{"heb"}},
	{"fa", "Persian", []string{"persian", "farsi"}, []string{"fas", "per"}},
	{"bn", "Bengali", []string{"bengali"}, []string{"ben"}},
	{"ur", "Urdu", []string{"urdu"}, []string{"urd"}},
	{"pa", "Punjabi", []string{"punjabi"}, []string{"pan"}},
	{"tl", "Filipino", []string{"filipino", "tagalog"}, []string{"fil"}},
	{"is", "Icelandic", []string{"icelandic"}, []string{"isl", "ice"}},
	{"et", "Estonian", []string{"estonian"}, []string{"est"}},
	{"lv", "Latvian", []string{"latvian"}, []string{"lav"}},
	{"lt", "Lithuanian", []string{"lithuanian"}, []string{"lit"}},
	{"sq", "Albanian", []string{"albanian"}, []string{"alb", "sqi"}},
	{"ka", "Georgian", []string{"georgian"}, []string{"geo", "kat"}},
	{"hy", "Armenian", []string{"armenian"}, []string{"arm", "hye"}},
	{"mn", "Mongolian", []string{"mongolian"}, []string{"mon"}},
	{"km", "Khmer", []string{"khmer"}, []string{"khm"}},
	{"lo", "Lao", []string{"lao"}, []string{}},
	{"my", "Burmese", []string{"burmese"}, []string{"mya", "bur"}},
	{"ne", "Nepali", []string{"nepali"}, []string{"nep"}},
	{"si", "Sinhala", []string{"sinhala", "sinhalese"}, []string{"sin"}},
	{"ml", "Malayalam", []string{"malayalam"}, []string{"mal"}},
	{"kn", "Kannada", []string{"kannada"}, []string{"kan"}},
	{"mr", "Marathi", []string{"marathi"}, []string{"mar"}},
	{"gu", "Gujarati", []string{"gujarati"}, []string{"guj"}},
	{"sw", "Swahili", []string{"swahili"}, []string{"swa"}},
	{"af", "Afrikaans", []string{"afrikaans"}, []string{"afr"}},
}

type langMatch struct {
	code, label string
}

// codeLookup accepts full words, abbreviations, and the bare ISO code
// itself — used for the strict ".xx.srt"-style suffix check, where the
// position is unambiguous enough that even a bare 2-letter code is safe.
var codeLookup = func() map[string]langMatch {
	m := map[string]langMatch{}
	for _, e := range languageEntries {
		m[e.code] = langMatch{e.code, e.label}
		for _, w := range e.fullWords {
			m[w] = langMatch{e.code, e.label}
		}
		for _, a := range e.abbrevs {
			m[a] = langMatch{e.code, e.label}
		}
	}
	return m
}()

type keywordRegex struct {
	code, label string
	re          *regexp.Regexp
}

func buildKeywordRegexes(pick func(e struct {
	code      string
	label     string
	fullWords []string
	abbrevs   []string
}) []string) []keywordRegex {
	var out []keywordRegex
	for _, e := range languageEntries {
		for _, kw := range pick(e) {
			out = append(out, keywordRegex{e.code, e.label, regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(kw) + `\b`)})
		}
	}
	return out
}

// fullWordRegexes/abbrevRegexes match anywhere in a filename. Full words are
// checked first across every language before any abbreviation is considered,
// so a clear "German"/"english" beats a coincidental 3-letter code collision.
var fullWordRegexes = buildKeywordRegexes(func(e struct {
	code      string
	label     string
	fullWords []string
	abbrevs   []string
}) []string {
	return e.fullWords
})

var abbrevRegexes = buildKeywordRegexes(func(e struct {
	code      string
	label     string
	fullWords []string
	abbrevs   []string
}) []string {
	return e.abbrevs
})

// detectLanguage looks for a language in filename: first a strict
// ".xx.srt"-style suffix right before the extension (trusts bare 2-letter
// codes too, since that position is unambiguous), then scans the whole
// filename for a full language-name word, then finally for an abbreviation.
// Returns ("und", "Unknown") if nothing is recognized.
func detectLanguage(filename string) (code, label string) {
	if m := langSuffixRegex.FindStringSubmatch(filename); m != nil {
		if match, ok := codeLookup[strings.ToLower(m[1])]; ok {
			return match.code, match.label
		}
	}

	for _, candidate := range fullWordRegexes {
		if candidate.re.MatchString(filename) {
			return candidate.code, candidate.label
		}
	}
	for _, candidate := range abbrevRegexes {
		if candidate.re.MatchString(filename) {
			return candidate.code, candidate.label
		}
	}

	return "und", "Unknown"
}

func labelFor(lang string) string {
	if match, ok := codeLookup[strings.ToLower(lang)]; ok {
		return match.label
	}
	return "Unknown"
}

var (
	subtitleExtRegex = regexp.MustCompile(`(?i)\.(srt|vtt|ass)$`)
	langSuffixRegex  = regexp.MustCompile(`(?i)\.([a-zA-Z]{2,3})\.(?:srt|vtt|ass)$`)
	episodeTagRegex  = regexp.MustCompile(`(?i)s\d{2}e\d{2}`)
	videoExts        = map[string]bool{".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".webm": true}
)

// belongsToVideo decides whether a candidate subtitle filename in the same
// folder as videoPath should be treated as a track for it. Real-world
// subtitle files (esp. ones grabbed separately from the video) very often
// have nothing in common with the video's filename — e.g. a scene-release
// name like "Movie [1080p-x265]-GROUP.srt" sitting next to a video the user
// renamed to "trailer.mp4" — so name-matching alone is too strict.
func belongsToVideo(videoName, candidateName string, singleVideoInFolder bool) bool {
	// Only one video in this folder (the common movie-folder case): any
	// subtitle file in it is unambiguous.
	if singleVideoInFolder {
		return true
	}

	// Multiple videos in the folder (a TV season folder): match by the
	// SxxEyy episode tag if both filenames carry one, since scene-release
	// subtitle names commonly differ from the video's filename but keep the
	// episode tag intact.
	videoTag := episodeTagRegex.FindString(videoName)
	candidateTag := episodeTagRegex.FindString(candidateName)
	if videoTag != "" && strings.EqualFold(videoTag, candidateTag) {
		return true
	}

	// Fallback: the classic "same basename" convention.
	base := strings.TrimSuffix(videoName, filepath.Ext(videoName))
	return strings.HasPrefix(candidateName, base)
}

// Discover finds all subtitle tracks available for a video: external
// .srt/.vtt/.ass files in the same folder (see belongsToVideo for how they're
// matched to this specific video), plus (if ffmpeg is installed) text-based
// subtitle streams muxed into the file. Every track's Label is a proper
// English language name (e.g. "German"), or "Unknown" if none could be
// identified from the filename/metadata.
func Discover(videoPath string) []Track {
	var tracks []Track

	dir := filepath.Dir(videoPath)
	videoName := filepath.Base(videoPath)

	if entries, err := os.ReadDir(dir); err == nil {
		videoCount := 0
		for _, entry := range entries {
			if !entry.IsDir() && videoExts[strings.ToLower(filepath.Ext(entry.Name()))] {
				videoCount++
			}
		}
		singleVideo := videoCount <= 1

		for _, entry := range entries {
			if entry.IsDir() || !subtitleExtRegex.MatchString(entry.Name()) {
				continue
			}
			if !belongsToVideo(videoName, entry.Name(), singleVideo) {
				continue
			}

			code, label := detectLanguage(entry.Name())
			tracks = append(tracks, Track{
				ID:       "ext:" + entry.Name(),
				Label:    label,
				Language: code,
				Source:   "external",
			})
		}
	}

	if ffmpeg.Available() {
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
