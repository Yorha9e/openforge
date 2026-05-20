package errors

import "fmt"

// Code is a machine-readable error category.
type Code string

const (
	CodeNotFound         Code = "NOT_FOUND"
	CodeAlreadyExists    Code = "ALREADY_EXISTS"
	CodeInvalidArgument  Code = "INVALID_ARGUMENT"
	CodePermissionDenied Code = "PERMISSION_DENIED"
	CodeUnauthenticated  Code = "UNAUTHENTICATED"
	CodeInternal         Code = "INTERNAL"
	CodeUnavailable      Code = "UNAVAILABLE"
	CodeNotImplemented   Code = "NOT_IMPLEMENTED"
)

// Error is a structured error with a code, message, and optional cause.
type Error struct {
	Code    Code
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// New creates a new Error with the given code and message.
func New(code Code, msg string) *Error {
	return &Error{Code: code, Message: msg}
}

// Wrap creates a new Error with the given code, message, and cause.
func Wrap(code Code, msg string, cause error) *Error {
	return &Error{Code: code, Message: msg, Cause: cause}
}
