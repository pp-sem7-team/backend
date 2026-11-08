package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

func BindJSON(c *gin.Context, dst any, maxBytes int64) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(dst); err != nil {
		writeJSONError(c, err)
		return false
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		writeJSONError(c, err)
		return false
	}

	if err := binding.Validator.ValidateStruct(dst); err != nil {
		writeJSONError(c, err)
		return false
	}
	return true
}

func writeJSONError(c *gin.Context, err error) {
	var sizeErr *http.MaxBytesError
	if errors.As(err, &sizeErr) {
		WriteError(c, http.StatusRequestEntityTooLarge, ErrorCodePayloadTooLarge, "Request body exceeds size limit")
		return
	}
	WriteError(c, http.StatusBadRequest, ErrorCodeInvalidRequest, "Invalid request body")
}
