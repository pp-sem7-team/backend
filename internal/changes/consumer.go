package changes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	ErrAnalysisResultCorrelationIDEmpty        = errors.New("analysis result correlation id is empty")
	ErrAnalysisResultAnalysisIDRequired        = errors.New("analysis result analysis_id is required")
	ErrAnalysisResultStructuredChangesRequired = errors.New("analysis result structured_changes is required for done status")
	ErrAnalysisResultErrorRequired             = errors.New("analysis result error is required for failed status")
	ErrAnalysisResultInvalidStatus             = errors.New("invalid analysis result status")
)

type AnalysisResultConsumer struct {
	service *Service
	log     *slog.Logger
}

func NewAnalysisResultConsumer(service *Service, log *slog.Logger) *AnalysisResultConsumer {
	return &AnalysisResultConsumer{
		service: service,
		log:     log,
	}
}

func (c *AnalysisResultConsumer) Handle(ctx context.Context, delivery amqp.Delivery) error {
	if delivery.CorrelationId == "" {
		return ErrAnalysisResultCorrelationIDEmpty
	}

	messageID, err := uuid.Parse(delivery.CorrelationId)
	if err != nil {
		return err
	}

	var message analysisResultMessage
	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		return err
	}

	if message.AnalysisID == uuid.Nil {
		return ErrAnalysisResultAnalysisIDRequired
	}

	status := AnalysisStatus(message.Status)

	switch status {
	case AnalysisStatusDone:
		if len(message.StructuredChanges) == 0 ||
			!json.Valid(message.StructuredChanges) ||
			bytes.Equal(bytes.TrimSpace(message.StructuredChanges), []byte("null")) {
			return ErrAnalysisResultStructuredChangesRequired
		}
	case AnalysisStatusFailed:
		if message.Error == nil {
			return ErrAnalysisResultErrorRequired
		}

		c.log.Warn("analysis generation failed",
			"analysis_id", message.AnalysisID,
			"message_id", messageID,
			"error_code", message.Error.Code,
			"error_message", message.Error.Message,
		)

		message.StructuredChanges = nil
	default:
		return ErrAnalysisResultInvalidStatus
	}

	applied, err := c.service.ApplyAnalysisResult(
		ctx,
		message.AnalysisID,
		messageID,
		status,
		message.StructuredChanges,
	)
	if err != nil {
		return err
	}

	if !applied {
		c.log.Info("analysis result ignored",
			"analysis_id", message.AnalysisID,
			"message_id", messageID,
			"status", status,
		)
	}

	return nil
}
