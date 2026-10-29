package changes

import (
	"encoding/json"

	"github.com/google/uuid"
)

type analysisRequestPayload struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
	RawDiff    string    `json:"raw_diff"`
}

type analysisResultMessage struct {
	AnalysisID        uuid.UUID        `json:"analysis_id"`
	Status            string           `json:"status"`
	StructuredChanges json.RawMessage  `json:"structured_changes"`
	Error             *generationError `json:"error"`
}

type generationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
