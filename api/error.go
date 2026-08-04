package dots

import (
	"errors"
	"fmt"
)

const (
	EINTERNAL       = "internal"
	ECONFLICT       = "conflict"
	EINVALID        = "invalid"
	ENOTFOUND       = "not_found"
	ENOTIMPLEMENTED = "not_implemented"
	EUNAUTHORIZED   = "unauthorized"
	ENOTAFFECTED    = "not_affected"
)

type Error struct {
	Code    string
	Message string
	Data    map[string]interface{}
	err     error
}

func (e *Error) Error() string {
	return fmt.Sprintf("dots error: Code: %s Message: %s", e.Code, e.Message)
}

func (e *Error) Wrap(err error) error {
	e.err = err
	return e
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) WithData(d map[string]interface{}) *Error {
	e.Data = d
	return e
}

func ErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return EINTERNAL
}

func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Message
	}
	return "internal"
}

func ErrorData(err error) map[string]interface{} {
	errdata := map[string]interface{}{}
	if err == nil {
		return errdata
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Data
	}

	return errdata
}

func Errorf(code string, format string, args ...interface{}) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}
