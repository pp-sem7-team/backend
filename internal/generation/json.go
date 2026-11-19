package generation

import (
	"encoding/json"
	"strings"
)

func HasField(fields map[string]json.RawMessage, name string) bool {
	for key := range fields {
		if strings.EqualFold(key, name) {
			return true
		}
	}
	return false
}
