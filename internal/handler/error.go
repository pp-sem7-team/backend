package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/pp-sem7-team/backend/internal/dto"
)

func WriteError(
	c *gin.Context,
	status int,
	code dto.ErrorCode,
	message string,
) {
	c.JSON(status, dto.ErrorResponse{
		Error: dto.Error{
			Code:    code,
			Message: message,
		},
	})
}
