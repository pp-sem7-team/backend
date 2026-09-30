package dto

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
