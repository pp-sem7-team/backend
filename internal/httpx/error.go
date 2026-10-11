package httpx

import "github.com/gin-gonic/gin"

type ErrorCode string

const (
	ErrorCodeInvalidRequest  ErrorCode = "invalid_request"
	ErrorCodeUnauthorized    ErrorCode = "unauthorized"
	ErrorCodeNotFound        ErrorCode = "not_found"
	ErrorCodeConflict        ErrorCode = "conflict"
	ErrorCodePayloadTooLarge ErrorCode = "payload_too_large"
	ErrorCodeInternalError   ErrorCode = "internal_error"
	ErrorCodeUnavailable     ErrorCode = "unavailable"
)

type ErrorResponse struct {
	Error Error `json:"error"`
}

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func WriteError(
	c *gin.Context,
	status int,
	code ErrorCode,
	message string,
) {
	c.JSON(status, ErrorResponse{
		Error: Error{
			Code:    code,
			Message: message,
		},
	})
}
