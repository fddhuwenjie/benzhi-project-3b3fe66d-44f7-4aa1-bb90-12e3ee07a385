package application

import (
	"oralarchive/internal/domain"
	"sort"
)

type TimelineEntry struct {
	Sequence int    `json:"sequence"`
	Action   string `json:"action"`
	Actor    string `json:"actor"`
	At       string `json:"at"`
	Digest   string `json:"digest"`
}

func Timeline(d *domain.InterviewDossier) []TimelineEntry {
	result := make([]TimelineEntry, 0, len(d.Audit))
	for _, e := range d.Audit {
		result = append(result, TimelineEntry{Sequence: e.Sequence, Action: e.Action, Actor: e.ActorID, At: e.At.Format("2006-01-02T15:04:05Z07:00"), Digest: e.SHA256})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Sequence < result[j].Sequence })
	return result
}
func PublicSegments(d *domain.InterviewDossier) []domain.TranscriptSegment {
	result := append([]domain.TranscriptSegment(nil), d.Segments...)
	if d.Status == domain.StatusSealed {
		for i := range result {
			result[i].Text = ""
		}
	}
	return result
}
