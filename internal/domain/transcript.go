package domain

func ValidateSegments(segments []TranscriptSegment) error {
	if len(segments) == 0 {
		return Invalid("segments", "至少需要一个转写片段")
	}
	for i, segment := range segments {
		if segment.StartMS < 0 || segment.EndMS <= segment.StartMS {
			return Invalid("segments", "时间范围必须递增")
		}
		if i > 0 && segments[i-1].StartMS > segment.StartMS {
			return Invalid("segments", "片段必须按起始时间排序")
		}
		if i > 0 && segments[i-1].EndMS > segment.StartMS {
			return Invalid("segments", "时间片段不能重叠")
		}
		if Clean(segment.Text) == "" {
			return Invalid("segments", "正文不能为空")
		}
	}
	return nil
}
