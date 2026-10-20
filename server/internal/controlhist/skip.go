package controlhist

import (
	"errors"

	"fireproxy/server/internal/fwapp"
)

// ErrModuleDisabled is passed in Outcome.Skip when a control module is off.
var ErrModuleDisabled = errors.New("control module disabled")

// ShouldSkip reports whether an outcome should not produce a history row.
func ShouldSkip(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, fwapp.ErrNotPaired) || errors.Is(err, ErrModuleDisabled)
}
