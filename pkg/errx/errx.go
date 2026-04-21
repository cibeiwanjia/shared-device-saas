package errx

import (
	"github.com/go-kratos/kratos/v2/errors"
)

// 统一错误码
var (
	Unauthorized = func(reason string) *errors.Error {
		return errors.Unauthorized("UNAUTHORIZED", reason)
	}
	Forbidden = func(reason string) *errors.Error {
		return errors.Forbidden("FORBIDDEN", reason)
	}
	BadRequest = func(reason string) *errors.Error {
		return errors.BadRequest("BAD_REQUEST", reason)
	}
	NotFound = func(reason string) *errors.Error {
		return errors.NotFound("NOT_FOUND", reason)
	}
	Internal = func(reason string) *errors.Error {
		return errors.InternalServer("INTERNAL_ERROR", reason)
	}
)
