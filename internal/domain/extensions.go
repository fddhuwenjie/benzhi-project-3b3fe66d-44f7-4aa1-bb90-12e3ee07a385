package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type AuthorizationInput struct {
	SubjectCode           string   `json:"subject_code"`
	AudioRef              string   `json:"audio_ref"`
	AudioSHA256           string   `json:"audio_sha256"`
	InterviewedAt         string   `json:"interviewed_at"`
	AllowedUses           []string `json:"allowed_uses"`
	EmbargoUntil          string   `json:"embargo_until"`
	ConsentEvidenceDigest string   `json:"consent_evidence_digest"`
}

func (d *InterviewDossier) ReviseAuthorization(in AuthorizationInput, actor string, now time.Time) (*AuthorizationRevision, error) {
	if d.Status != StatusDraft {
		return nil, ErrInvalidState
	}
	if Clean(actor) == "" {
		return nil, Invalid("actor_id", "不能为空")
	}
	uses, err := NormalizeUses(in.AllowedUses)
	if err != nil {
		return nil, err
	}
	if err = ValidateAuthorization(in.SubjectCode, in.AudioRef, in.AudioSHA256, in.InterviewedAt, in.ConsentEvidenceDigest, uses); err != nil {
		return nil, err
	}
	if err = ValidateEmbargo(in.InterviewedAt, in.EmbargoUntil); err != nil {
		return nil, err
	}
	in.SubjectCode, in.AudioRef = Clean(in.SubjectCode), Clean(in.AudioRef)
	in.AudioSHA256, in.ConsentEvidenceDigest = strings.ToLower(in.AudioSHA256), strings.ToLower(in.ConsentEvidenceDigest)
	in.AllowedUses = uses
	before := AuthorizationInput{d.SubjectCode, d.AudioRef, d.AudioSHA256, d.InterviewedAt, append([]string(nil), d.AllowedUses...), d.EmbargoUntil, d.ConsentEvidenceDigest}
	fields := authorizationChangedFields(before, in)
	if len(fields) == 0 {
		return nil, Invalid("authorization", "没有发生变更")
	}
	rec := AuthorizationRevision{RevisionID: fmt.Sprintf("authrev-%d", d.Revision+1), DossierID: d.DossierID, ChangedFields: fields, BeforeSHA256: map[string]string{}, AfterSHA256: map[string]string{}, ActorID: Clean(actor), CreatedAt: now.UTC(), Revision: d.Revision + 1}
	for _, field := range fields {
		rec.BeforeSHA256[field] = authorizationFieldDigest(before, field)
		rec.AfterSHA256[field] = authorizationFieldDigest(in, field)
	}
	d.SubjectCode, d.AudioRef, d.AudioSHA256, d.InterviewedAt = in.SubjectCode, in.AudioRef, in.AudioSHA256, in.InterviewedAt
	d.AllowedUses, d.EmbargoUntil, d.ConsentEvidenceDigest = in.AllowedUses, in.EmbargoUntil, in.ConsentEvidenceDigest
	d.AuthorizationRevisions = append(d.AuthorizationRevisions, rec)
	d.Advance(now)
	return &rec, nil
}

func authorizationChangedFields(a, b AuthorizationInput) []string {
	fields := []string{}
	if a.SubjectCode != b.SubjectCode {
		fields = append(fields, "subject_code")
	}
	if a.AudioRef != b.AudioRef {
		fields = append(fields, "audio_ref")
	}
	if a.AudioSHA256 != b.AudioSHA256 {
		fields = append(fields, "audio_sha256")
	}
	if a.InterviewedAt != b.InterviewedAt {
		fields = append(fields, "interviewed_at")
	}
	if strings.Join(a.AllowedUses, "\x00") != strings.Join(b.AllowedUses, "\x00") {
		fields = append(fields, "allowed_uses")
	}
	if a.EmbargoUntil != b.EmbargoUntil {
		fields = append(fields, "embargo_until")
	}
	if a.ConsentEvidenceDigest != b.ConsentEvidenceDigest {
		fields = append(fields, "consent_evidence_digest")
	}
	return fields
}

func authorizationFieldDigest(in AuthorizationInput, field string) string {
	var value any
	switch field {
	case "subject_code":
		value = in.SubjectCode
	case "audio_ref":
		value = in.AudioRef
	case "audio_sha256":
		value = in.AudioSHA256
	case "interviewed_at":
		value = in.InterviewedAt
	case "allowed_uses":
		value = in.AllowedUses
	case "embargo_until":
		value = in.EmbargoUntil
	case "consent_evidence_digest":
		value = in.ConsentEvidenceDigest
	}
	b, _ := json.Marshal(value)
	return Digest(string(b))
}

type TranscriptOperation struct {
	Type      string             `json:"type"`
	SegmentID string             `json:"segment_id"`
	Segment   *TranscriptSegment `json:"segment,omitempty"`
}

type TranscriptPreflight struct {
	Valid              bool           `json:"valid"`
	CoverageStartMS    int64          `json:"coverage_start_ms"`
	CoverageEndMS      int64          `json:"coverage_end_ms"`
	TotalDurationMS    int64          `json:"total_duration_ms"`
	Gaps               []TimelineGap  `json:"gaps"`
	UndeclaredSpeakers []string       `json:"undeclared_speakers"`
	Errors             []LocatedError `json:"errors"`
}
type TimelineGap struct {
	AfterSegmentID  string `json:"after_segment_id"`
	BeforeSegmentID string `json:"before_segment_id"`
	StartMS         int64  `json:"start_ms"`
	EndMS           int64  `json:"end_ms"`
}

func (d *InterviewDossier) ApplyTranscriptOperations(ops []TranscriptOperation, now time.Time) error {
	if d.Status != StatusConsentLocked {
		return ErrInvalidState
	}
	merged, errors := d.MergeTranscriptOperations(ops)
	if len(errors) > 0 {
		return ValidationErrors{Items: errors}
	}
	d.Segments = merged
	d.Advance(now)
	return nil
}

func (d *InterviewDossier) MergeTranscriptOperations(ops []TranscriptOperation) ([]TranscriptSegment, []LocatedError) {
	if len(ops) == 0 {
		return nil, []LocatedError{{Field: "operations", Code: "required", Message: "至少需要一项操作"}}
	}
	segments := make(map[string]TranscriptSegment, len(d.Segments))
	for _, segment := range d.Segments {
		segments[segment.SegmentID] = segment
	}
	seen := map[string]bool{}
	errs := []LocatedError{}
	for index, op := range ops {
		id := Clean(op.SegmentID)
		if id == "" && op.Segment != nil {
			id = Clean(op.Segment.SegmentID)
		}
		if id == "" {
			errs = append(errs, LocatedError{Field: "segment_id", Code: "required", Message: "片段标识不能为空", Index: index})
			continue
		}
		if seen[id] {
			errs = append(errs, LocatedError{Field: "segment_id", Code: "duplicate", Message: "同一批次中片段标识重复", SegmentID: id, Index: index})
			continue
		}
		seen[id] = true
		switch op.Type {
		case "add":
			if _, exists := segments[id]; exists {
				errs = append(errs, LocatedError{Field: "segment_id", Code: "duplicate", Message: "新增片段标识已存在", SegmentID: id, Index: index})
				continue
			}
			if op.Segment == nil {
				errs = append(errs, LocatedError{Field: "segment", Code: "required", Message: "新增操作缺少片段", SegmentID: id, Index: index})
				continue
			}
			next := *op.Segment
			next.SegmentID = id
			segments[id] = next
		case "update":
			old, exists := segments[id]
			if !exists {
				errs = append(errs, LocatedError{Field: "segment_id", Code: "not_found", Message: "修改目标不存在", SegmentID: id, Index: index})
				continue
			}
			if op.Segment == nil {
				errs = append(errs, LocatedError{Field: "segment", Code: "required", Message: "修改操作缺少片段", SegmentID: id, Index: index})
				continue
			}
			next := *op.Segment
			next.SegmentID = id
			if next.Text == old.Text {
				next.TextSHA256 = old.TextSHA256
				next.Revision = old.Revision
			}
			segments[id] = next
		case "delete":
			if _, exists := segments[id]; !exists {
				errs = append(errs, LocatedError{Field: "segment_id", Code: "not_found", Message: "删除目标不存在", SegmentID: id, Index: index})
				continue
			}
			delete(segments, id)
		default:
			errs = append(errs, LocatedError{Field: "type", Code: "invalid", Message: "操作类型必须为 add、update 或 delete", SegmentID: id, Index: index})
		}
	}
	result := make([]TranscriptSegment, 0, len(segments))
	for _, segment := range segments {
		result = append(result, segment)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].StartMS == result[j].StartMS {
			return result[i].SegmentID < result[j].SegmentID
		}
		return result[i].StartMS < result[j].StartMS
	})
	errs = append(errs, validateTimeline(result)...)
	if len(errs) > 0 {
		return nil, errs
	}
	for i := range result {
		s := &result[i]
		s.DossierID = d.DossierID
		s.Sequence = i + 1
		if s.TextSHA256 == "" || s.TextSHA256 != Digest(s.Text) {
			s.TextSHA256 = Digest(s.Text)
			s.Revision = d.Revision + 1
		}
	}
	return result, nil
}

func validateTimeline(segments []TranscriptSegment) []LocatedError {
	errs := []LocatedError{}
	if len(segments) == 0 {
		return []LocatedError{{Field: "segments", Code: "empty", Message: "删除后转写不能为空"}}
	}
	for i, s := range segments {
		if s.StartMS < 0 {
			errs = append(errs, LocatedError{Field: "start_ms", Code: "negative", Message: "起始时间不能为负数", SegmentID: s.SegmentID, Index: i})
		}
		if s.EndMS <= s.StartMS {
			errs = append(errs, LocatedError{Field: "end_ms", Code: "not_after_start", Message: "结束时间必须晚于起始时间", SegmentID: s.SegmentID, Index: i})
		}
		if Clean(s.Text) == "" {
			errs = append(errs, LocatedError{Field: "text", Code: "empty", Message: "正文不能为空", SegmentID: s.SegmentID, Index: i})
		}
		if Clean(s.SpeakerCode) == "" {
			errs = append(errs, LocatedError{Field: "speaker_code", Code: "empty", Message: "说话人不能为空", SegmentID: s.SegmentID, Index: i})
		}
		if i > 0 && segments[i-1].EndMS > s.StartMS {
			errs = append(errs, LocatedError{Field: "timeline", Code: "overlap", Message: "与前一片段时间重叠", SegmentID: s.SegmentID, Index: i})
		}
	}
	return errs
}

func TranscriptPrecheck(segments []TranscriptSegment) TranscriptPreflight {
	p := TranscriptPreflight{Gaps: []TimelineGap{}, UndeclaredSpeakers: []string{}, Errors: []LocatedError{}}
	ordered := append([]TranscriptSegment(nil), segments...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartMS < ordered[j].StartMS })
	p.Errors = validateTimeline(ordered)
	p.Valid = len(p.Errors) == 0
	if len(ordered) == 0 {
		return p
	}
	p.CoverageStartMS = ordered[0].StartMS
	p.CoverageEndMS = ordered[len(ordered)-1].EndMS
	for i, s := range ordered {
		if s.EndMS > s.StartMS {
			p.TotalDurationMS += s.EndMS - s.StartMS
		}
		if strings.EqualFold(Clean(s.SpeakerCode), "UNDECLARED") {
			p.UndeclaredSpeakers = append(p.UndeclaredSpeakers, s.SegmentID)
		}
		if i > 0 && ordered[i-1].EndMS < s.StartMS {
			p.Gaps = append(p.Gaps, TimelineGap{ordered[i-1].SegmentID, s.SegmentID, ordered[i-1].EndMS, s.StartMS})
		}
	}
	return p
}

type RedactionResolution struct {
	IssueID           string `json:"issue_id"`
	StartOffset       int    `json:"start_offset"`
	EndOffset         int    `json:"end_offset"`
	Reason            string `json:"reason"`
	ReplacementText   string `json:"replacement_text"`
	ActorID           string `json:"actor_id"`
	SegmentTextSHA256 string `json:"segment_text_sha256"`
}

func (d *InterviewDossier) ResolveIssues(items []RedactionResolution, now time.Time) error {
	if d.Status != StatusRemediation {
		return ErrInvalidState
	}
	if len(items) == 0 {
		return Invalid("items", "至少提交一个问题")
	}
	errs := []LocatedError{}
	seen := map[string]bool{}
	ranges := map[string][]RedactionResolution{}
	for idx, item := range items {
		issue := d.issue(item.IssueID)
		if issue == nil || issue.DossierID != d.DossierID {
			errs = append(errs, LocatedError{Field: "issue_id", Code: "not_found", Message: "问题不属于当前档案", IssueID: item.IssueID, Index: idx})
			continue
		}
		if seen[item.IssueID] {
			errs = append(errs, LocatedError{Field: "issue_id", Code: "duplicate", Message: "问题标识重复", IssueID: item.IssueID, Index: idx})
			continue
		}
		seen[item.IssueID] = true
		if issue.Status != IssueOpen {
			errs = append(errs, LocatedError{Field: "issue_id", Code: "not_open", Message: "问题已不再开放", IssueID: item.IssueID, Index: idx})
		}
		seg := d.segment(issue.SegmentID)
		if seg == nil {
			errs = append(errs, LocatedError{Field: "segment_id", Code: "not_found", Message: "问题片段不存在", IssueID: item.IssueID, Index: idx})
			continue
		}
		if item.SegmentTextSHA256 != seg.TextSHA256 {
			errs = append(errs, LocatedError{Field: "segment_text_sha256", Code: "stale", Message: "片段正文摘要已变化", IssueID: item.IssueID, Index: idx})
		}
		length := utf8.RuneCountInString(seg.Text)
		if item.StartOffset < 0 || item.EndOffset <= item.StartOffset || item.EndOffset > length {
			errs = append(errs, LocatedError{Field: "range", Code: "invalid", Message: "字符范围无效", IssueID: item.IssueID, SegmentID: seg.SegmentID, Index: idx})
		}
		if Clean(item.Reason) == "" || Clean(item.ReplacementText) == "" || Clean(item.ActorID) == "" {
			errs = append(errs, LocatedError{Field: "redaction", Code: "required", Message: "原因、替换文本和处理人不能为空", IssueID: item.IssueID, Index: idx})
		}
		ranges[seg.SegmentID] = append(ranges[seg.SegmentID], item)
	}
	for _, issue := range d.Issues {
		if issue.Status == IssueResolved && !seen[issue.IssueID] {
			ranges[issue.SegmentID] = append(ranges[issue.SegmentID], RedactionResolution{IssueID: issue.IssueID, StartOffset: issue.StartOffset, EndOffset: issue.EndOffset})
		}
	}
	for _, rs := range ranges {
		sort.Slice(rs, func(i, j int) bool { return rs[i].StartOffset < rs[j].StartOffset })
		for i := 1; i < len(rs); i++ {
			if rs[i-1].EndOffset > rs[i].StartOffset {
				errs = append(errs, LocatedError{Field: "range", Code: "overlap", Message: "同一片段的遮蔽范围相互重叠", IssueID: rs[i].IssueID})
			}
		}
	}
	if len(errs) > 0 {
		return ValidationErrors{Items: errs}
	}
	for _, item := range items {
		issue := d.issue(item.IssueID)
		seg := d.segment(issue.SegmentID)
		runes := []rune(seg.Text)
		issue.StartOffset = item.StartOffset
		issue.EndOffset = item.EndOffset
		issue.Reason = Clean(item.Reason)
		issue.ReplacementText = item.ReplacementText
		issue.OriginalSHA256 = Digest(string(runes[item.StartOffset:item.EndOffset]))
		issue.Status = IssueResolved
		issue.ResolvedBy = Clean(item.ActorID)
		t := now.UTC()
		issue.ResolvedAt = &t
		issue.History = append(issue.History, IssueResolutionSnapshot{RoundNumber: issue.CurrentRound, Reason: issue.Reason, ReplacementText: issue.ReplacementText, OriginalSHA256: issue.OriginalSHA256, ResolvedBy: issue.ResolvedBy, ResolvedAt: t})
	}
	if !d.HasOpenBlockers() {
		d.Status = StatusConfirmation
	}
	d.Advance(now)
	return nil
}

func (d *InterviewDossier) issue(id string) *RedactionIssue {
	for i := range d.Issues {
		if d.Issues[i].IssueID == id {
			return &d.Issues[i]
		}
	}
	return nil
}
