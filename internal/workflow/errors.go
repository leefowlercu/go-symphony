package workflow

import "fmt"

type ErrorCode string

const (
	ErrMissingWorkflowFile     ErrorCode = "missing_workflow_file"
	ErrWorkflowParse           ErrorCode = "workflow_parse_error"
	ErrWorkflowFrontMatterType ErrorCode = "workflow_front_matter_not_a_map"
	ErrTemplateParse           ErrorCode = "template_parse_error"
	ErrTemplateRender          ErrorCode = "template_render_error"
)

type Error struct {
	Code ErrorCode
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }
