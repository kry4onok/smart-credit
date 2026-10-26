package requirements

import "strings"

type RuleError struct {
	Code   string
	Issues []Issue
}

func (e *RuleError) Error() string {
	if e == nil {
		return ""
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, issue.Path+": "+issue.Message)
	}
	if len(parts) == 0 {
		return e.Code
	}
	return e.Code + ": " + strings.Join(parts, "; ")
}
