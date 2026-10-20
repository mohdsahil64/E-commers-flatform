package httpx

import (
	"errors"

	"github.com/gofiber/fiber/v2"
)

// AppError is a domain error that carries an HTTP status and a stable code.
// Services return these; the central error handler translates them to JSON.
type AppError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]any
}

func (e *AppError) Error() string { return e.Message }

// NewError constructs an AppError.
func NewError(status int, code, message string) *AppError {
	return &AppError{Status: status, Code: code, Message: message}
}

// Common constructors for frequent cases.
func BadRequest(code, msg string) *AppError   { return NewError(fiber.StatusBadRequest, code, msg) }
func Unauthorized(code, msg string) *AppError { return NewError(fiber.StatusUnauthorized, code, msg) }
func Forbidden(code, msg string) *AppError    { return NewError(fiber.StatusForbidden, code, msg) }
func NotFound(code, msg string) *AppError     { return NewError(fiber.StatusNotFound, code, msg) }
func Conflict(code, msg string) *AppError     { return NewError(fiber.StatusConflict, code, msg) }
func Internal(msg string) *AppError {
	return NewError(fiber.StatusInternalServerError, "internal_error", msg)
}

// WithFields attaches field-level validation details.
func (e *AppError) WithFields(f map[string]any) *AppError {
	e.Fields = f
	return e
}

// ErrorHandler is the Fiber central error handler. It understands AppError and
// falls back to a generic 500 for anything else.
func ErrorHandler(c *fiber.Ctx, err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return FailFields(c, appErr.Status, appErr.Code, appErr.Message, appErr.Fields)
	}

	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		return Fail(c, fiberErr.Code, "http_error", fiberErr.Message)
	}

	return Fail(c, fiber.StatusInternalServerError, "internal_error", "Something went wrong")
}
