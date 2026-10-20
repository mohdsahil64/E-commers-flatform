// Package httpx contains shared HTTP helpers: a consistent JSON response
// envelope and error types used by every handler.
package httpx

import (
	"github.com/gofiber/fiber/v2"
)

// Envelope is the standard success response shape.
type Envelope struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Meta    *Meta  `json:"meta,omitempty"`
	Message string `json:"message,omitempty"`
}

// Meta carries pagination and related metadata.
type Meta struct {
	Page       int   `json:"page,omitempty"`
	PageSize   int   `json:"page_size,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

// ErrorBody is the standard error response shape.
type ErrorBody struct {
	Success bool           `json:"success"`
	Error   ErrorDetail    `json:"error"`
	Fields  map[string]any `json:"fields,omitempty"`
}

// ErrorDetail describes an error to the client.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// OK writes a 200 success envelope.
func OK(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(Envelope{Success: true, Data: data})
}

// Created writes a 201 success envelope.
func Created(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Envelope{Success: true, Data: data})
}

// Paginated writes a 200 envelope with pagination meta.
func Paginated(c *fiber.Ctx, data any, meta Meta) error {
	return c.Status(fiber.StatusOK).JSON(Envelope{Success: true, Data: data, Meta: &meta})
}

// Message writes a 200 envelope carrying only a message.
func Message(c *fiber.Ctx, msg string) error {
	return c.Status(fiber.StatusOK).JSON(Envelope{Success: true, Message: msg})
}

// Fail writes an error response with the given status, code and message.
func Fail(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(ErrorBody{
		Success: false,
		Error:   ErrorDetail{Code: code, Message: message},
	})
}

// FailFields writes a validation error with per-field details.
func FailFields(c *fiber.Ctx, status int, code, message string, fields map[string]any) error {
	return c.Status(status).JSON(ErrorBody{
		Success: false,
		Error:   ErrorDetail{Code: code, Message: message},
		Fields:  fields,
	})
}
