package browserd

import (
	"errors"
	"fmt"
	"strings"
)

type Error struct {
	Code       string
	Message    string
	StatusCode int
	Operation  string
	Path       string
	Cause      error
}

func (e Error) Unwrap() error { return e.Cause }

func (e Error) Error() string {
	code := strings.TrimSpace(e.Code)
	message := strings.TrimSpace(e.Message)
	operation := strings.TrimSpace(strings.Join([]string{strings.TrimSpace(e.Operation), strings.TrimSpace(e.Path)}, " "))
	if operation != "" {
		if code != "" && e.StatusCode > 0 {
			if message != "" {
				return fmt.Sprintf("browserd %s failed with %s (status %d): %s", operation, code, e.StatusCode, message)
			}
			return fmt.Sprintf("browserd %s failed with %s (status %d)", operation, code, e.StatusCode)
		}
		if code != "" {
			if message != "" {
				return fmt.Sprintf("browserd %s failed with %s: %s", operation, code, message)
			}
			return fmt.Sprintf("browserd %s failed with %s", operation, code)
		}
		if e.StatusCode > 0 {
			if message != "" {
				return fmt.Sprintf("browserd %s failed with status %d: %s", operation, e.StatusCode, message)
			}
			return fmt.Sprintf("browserd %s failed with status %d", operation, e.StatusCode)
		}
		if message != "" {
			return fmt.Sprintf("browserd %s failed: %s", operation, message)
		}
		return fmt.Sprintf("browserd %s failed", operation)
	}
	if message != "" {
		return message
	}
	if code != "" {
		if e.StatusCode > 0 {
			return fmt.Sprintf("browserd failed with %s (status %d)", code, e.StatusCode)
		}
		return code
	}
	return "browserd error"
}

func AsError(err error, target *Error) bool {
	return errors.As(err, target)
}
