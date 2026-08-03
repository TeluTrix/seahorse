package api

import (
	"net/http"
	"strconv"

	"github.com/TeluTrix/seahorse/internal/transcode"
)

type AudioTrackDTO struct {
	// ID is the stream's absolute container index (as a string, ready to
	// pass straight back as the stream endpoint's "?track=" parameter).
	ID       string `json:"id"`
	Language string `json:"language"`
}

func toAudioTrackDTOs(tracks []transcode.AudioTrackInfo) []AudioTrackDTO {
	dtos := make([]AudioTrackDTO, 0, len(tracks))
	for _, t := range tracks {
		dtos = append(dtos, AudioTrackDTO{ID: strconv.Itoa(t.Index), Language: t.Language})
	}
	return dtos
}

func (h *Handlers) MovieAudioTracks(w http.ResponseWriter, r *http.Request) {
	filePath, ok := h.movieFilePath(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toAudioTrackDTOs(transcode.ListAudioTracks(filePath, h.Scanner.TranscodeOptions())))
}

func (h *Handlers) EpisodeAudioTracks(w http.ResponseWriter, r *http.Request) {
	filePath, ok := h.episodeFilePath(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toAudioTrackDTOs(transcode.ListAudioTracks(filePath, h.Scanner.TranscodeOptions())))
}
