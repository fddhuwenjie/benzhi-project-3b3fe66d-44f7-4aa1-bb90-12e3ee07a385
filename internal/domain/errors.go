package domain

import "errors"

var (
	ErrNotFound         = errors.New("档案不存在")
	ErrConflict         = errors.New("档案版本冲突，请刷新后重试")
	ErrInvalidState     = errors.New("当前状态不允许此操作")
	ErrValidation       = errors.New("提交内容不符合业务规则")
	ErrTerminal         = errors.New("档案已封存，禁止修改")
	ErrRoleSeparation   = errors.New("复核员必须与整理员不同")
	ErrUnresolvedIssues = errors.New("仍有阻断问题未解决")
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type LocatedError struct {
	Field     string `json:"field"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	SegmentID string `json:"segment_id,omitempty"`
	IssueID   string `json:"issue_id,omitempty"`
	Index     int    `json:"index,omitempty"`
}

type ValidationErrors struct {
	Items []LocatedError `json:"errors"`
}

func (e ValidationErrors) Error() string { return "提交内容包含多项校验错误" }

func (e FieldError) Error() string { return e.Field + ": " + e.Message }

func Invalid(field, message string) error { return FieldError{Field: field, Message: message} }
