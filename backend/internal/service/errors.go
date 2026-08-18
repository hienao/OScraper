package service

import "net/http"

type Error struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

func BadRequest(code, message string) *Error {
	return &Error{Status: http.StatusBadRequest, Code: code, Message: message}
}
func Unauthorized(code, message string) *Error {
	return &Error{Status: http.StatusUnauthorized, Code: code, Message: message}
}
func Forbidden(code, message string) *Error {
	return &Error{Status: http.StatusForbidden, Code: code, Message: message}
}
func NotFound(code, message string) *Error {
	return &Error{Status: http.StatusNotFound, Code: code, Message: message}
}
func Conflict(code, message string) *Error {
	return &Error{Status: http.StatusConflict, Code: code, Message: message}
}
func Internal(code, message string, cause error) *Error {
	return &Error{Status: http.StatusInternalServerError, Code: code, Message: message, Cause: cause}
}
