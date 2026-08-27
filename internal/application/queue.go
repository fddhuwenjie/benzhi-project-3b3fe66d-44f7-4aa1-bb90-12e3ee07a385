package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"oralarchive/internal/domain"
	"oralarchive/internal/repository"
)

type queueCursor struct {
	UpdatedAt    string `json:"updated_at"`
	DossierID    string `json:"dossier_id"`
	FilterSHA256 string `json:"filter_sha256"`
	Signature    string `json:"signature"`
}

func (s *Service) QueryQueue(ctx context.Context, filter QueueFilter) (QueueResult, error) {
	if filter.PageSize == 0 {
		filter.PageSize = 25
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		return QueueResult{}, domain.Invalid("page_size", "必须在 1 到 100 之间")
	}
	statuses := map[domain.Status]bool{}
	for _, status := range filter.Statuses {
		if !domain.ValidStatus(string(status)) {
			return QueueResult{}, domain.Invalid("status", "包含未知状态")
		}
		statuses[status] = true
	}
	var from, to *time.Time
	var err error
	if filter.UpdatedFrom != "" {
		v, e := parseQueueTime(filter.UpdatedFrom, false)
		if e != nil {
			return QueueResult{}, domain.Invalid("updated_from", "必须为 YYYY-MM-DD 或 RFC3339 时间")
		}
		from = &v
	}
	if filter.UpdatedTo != "" {
		v, e := parseQueueTime(filter.UpdatedTo, true)
		if e != nil {
			return QueueResult{}, domain.Invalid("updated_to", "必须为 YYYY-MM-DD 或 RFC3339 时间")
		}
		to = &v
	}
	if from != nil && to != nil && to.Before(*from) {
		return QueueResult{}, domain.Invalid("updated_to", "不得早于 updated_from")
	}
	items, err := s.store.QueryDossiers(ctx, repository.DossierFilter{Statuses: statuses, EditorID: filter.EditorID, SubjectCode: filter.SubjectCode, Keyword: filter.Keyword, UpdatedFrom: from, UpdatedTo: to})
	if err != nil {
		return QueueResult{}, err
	}
	filterDigest := queueFilterDigest(filter)
	start := 0
	if filter.Cursor != "" {
		cursor, e := decodeQueueCursor(filter.Cursor)
		if e != nil || cursor.FilterSHA256 != filterDigest {
			return QueueResult{}, domain.Invalid("cursor", "游标无效或已失效")
		}
		found := false
		for i, d := range items {
			if d.DossierID == cursor.DossierID && d.UpdatedAt.UTC().Format(time.RFC3339Nano) == cursor.UpdatedAt {
				start = i + 1
				found = true
				break
			}
		}
		if !found {
			return QueueResult{}, domain.Invalid("cursor", "游标无效或已失效")
		}
	}
	result := QueueResult{Dossiers: []QueueItem{}, Stats: QueueStats{ByStatus: map[domain.Status]int{}}}
	for _, status := range domain.AllStatuses() {
		result.Stats.ByStatus[status] = 0
	}
	for _, d := range items {
		result.Stats.ByStatus[d.Status]++
		blockers := d.OpenBlockers
		result.Stats.OpenBlockers += blockers
		if d.Status == domain.StatusConfirmation {
			result.Stats.PendingConfirmations++
		}
		if d.Status == domain.StatusReview {
			result.Stats.PendingReviews++
		}
	}
	end := start + filter.PageSize
	if end > len(items) {
		end = len(items)
	}
	for _, d := range items[start:end] {
		result.Dossiers = append(result.Dossiers, QueueItem{DossierID: d.DossierID, SubjectCode: d.SubjectCode, Status: d.Status, StatusLabel: d.Status.Label(), EditorID: d.EditorID, Revision: d.Revision, UpdatedAt: d.UpdatedAt.UTC().Format(time.RFC3339Nano), OpenBlockers: d.OpenBlockers, PendingConfirmations: boolInt(d.Status == domain.StatusConfirmation), PendingReviews: boolInt(d.Status == domain.StatusReview)})
	}
	if end < len(items) {
		last := items[end-1]
		result.NextCursor = encodeQueueCursor(last.UpdatedAt.UTC().Format(time.RFC3339Nano), last.DossierID, filterDigest)
	}
	return result, nil
}

func parseQueueTime(value string, endOfDay bool) (time.Time, error) {
	if v, err := time.Parse(time.RFC3339, value); err == nil {
		return v, nil
	}
	v, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		v = v.Add(24*time.Hour - time.Nanosecond)
	}
	return v, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
func queueFilterDigest(f QueueFilter) string {
	statuses := make([]string, len(f.Statuses))
	for i, s := range f.Statuses {
		statuses[i] = string(s)
	}
	sort.Strings(statuses)
	payload, _ := json.Marshal([]any{statuses, strings.TrimSpace(f.EditorID), strings.TrimSpace(f.SubjectCode), strings.TrimSpace(f.Keyword), f.UpdatedFrom, f.UpdatedTo, f.PageSize})
	return domain.Digest(string(payload))
}
func encodeQueueCursor(at, id, filter string) string {
	signature := domain.Digest(at + "|" + id + "|" + filter + "|oralarchive-queue-v1")
	b, _ := json.Marshal(queueCursor{at, id, filter, signature})
	return base64.RawURLEncoding.EncodeToString(b)
}
func decodeQueueCursor(raw string) (queueCursor, error) {
	var c queueCursor
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if c.Signature != domain.Digest(c.UpdatedAt+"|"+c.DossierID+"|"+c.FilterSHA256+"|oralarchive-queue-v1") {
		return c, domain.ErrValidation
	}
	return c, nil
}
