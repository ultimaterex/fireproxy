package controlhist

import (
	"errors"
	"strings"

	"fireproxy/server/internal/fwapp"
)

// MapError maps a control error to the History result vocabulary.
func MapError(err error) string {
	if err == nil {
		return ResultOK
	}
	if errors.Is(err, fwapp.ErrLocalUnreach) {
		return Result502
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "conflict"):
		return Result409
	case strings.Contains(msg, "busy"), strings.Contains(msg, "rate limit"), strings.Contains(msg, "in progress"):
		return ResultBusy
	case strings.Contains(msg, "invalid"), strings.Contains(msg, "validation"),
		strings.Contains(msg, "required"), strings.HasPrefix(msg, "bad "):
		return Result400
	default:
		return ResultError
	}
}
