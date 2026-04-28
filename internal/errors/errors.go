package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Common application errors
var (
	ErrNotFound          = errors.New("resource not found")
	ErrAlreadyExists     = errors.New("resource already exists")
	ErrInvalidInput      = errors.New("invalid input")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrInternal          = errors.New("internal server error")
	ErrDatabaseOperation = errors.New("database operation failed")
	ErrCacheOperation    = errors.New("cache operation failed")
	ErrFileOperation     = errors.New("file operation failed")
	ErrValidation        = errors.New("validation error")
	ErrConflict          = errors.New("resource conflict")
	ErrTimeout           = errors.New("operation timeout")
)

type AppError struct {
	Err        error
	Message    string
	StatusCode int
	Context    map[string]interface{}
}

func (e *AppError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "unknown error"
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new application error
func NewAppError(err error, message string, statusCode int) *AppError {
	return &AppError{
		Err:        err,
		Message:    message,
		StatusCode: statusCode,
		Context:    make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Error type checkers
func IsNotFound(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return errors.Is(appErr.Err, ErrNotFound)
	}
	return errors.Is(err, ErrNotFound)
}

func IsAlreadyExists(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return errors.Is(appErr.Err, ErrAlreadyExists)
	}
	return errors.Is(err, ErrAlreadyExists)
}

func IsValidation(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return errors.Is(appErr.Err, ErrValidation)
	}
	return errors.Is(err, ErrValidation)
}

func IsUnauthorized(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return errors.Is(appErr.Err, ErrUnauthorized)
	}
	return errors.Is(err, ErrUnauthorized)
}

// Common error constructors
func NotFound(message string) *AppError {
	return NewAppError(ErrNotFound, message, http.StatusNotFound)
}

func BadRequest(message string) *AppError {
	return NewAppError(ErrInvalidInput, message, http.StatusBadRequest)
}

func Validation(message string) *AppError {
	return NewAppError(ErrValidation, message, http.StatusBadRequest)
}

func Unauthorized(message string) *AppError {
	return NewAppError(ErrUnauthorized, message, http.StatusUnauthorized)
}

func Forbidden(message string) *AppError {
	return NewAppError(ErrForbidden, message, http.StatusForbidden)
}

func Conflict(message string) *AppError {
	return NewAppError(ErrConflict, message, http.StatusConflict)
}

func Internal(message string) *AppError {
	return NewAppError(ErrInternal, message, http.StatusInternalServerError)
}

// GetStatusCode returns the appropriate HTTP status code for an error
func GetStatusCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrAlreadyExists), errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidInput), errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}
