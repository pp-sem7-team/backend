package change

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/pp-sem7-team/backend/internal/generation"
)

type analysisRequestPayload struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
	RawDiff    string    `json:"raw_diff"`
}

type analysisResultMessage struct {
	AnalysisID        uuid.UUID         `json:"analysis_id"`
	Status            string            `json:"status"`
	StructuredChanges json.RawMessage   `json:"structured_changes"`
	Error             *generation.Error `json:"error"`
}
