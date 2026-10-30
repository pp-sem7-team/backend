package changes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/pp-sem7-team/backend/internal/broker"
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
	message, messageID, err := parseAnalysisResult(delivery)
	if err != nil {
		c.log.Warn("analysis result message rejected",
			"queue_message_id", delivery.MessageId,
			"correlation_id", delivery.CorrelationId,
			"error", err,
		)

		return broker.ErrDiscardMessage
	}

	status := AnalysisStatus(message.Status)

	if status == AnalysisStatusFailed {
		c.log.Warn("analysis generation failed",
			"analysis_id", message.AnalysisID,
			"message_id", messageID,
			"error_code", message.Error.Code,
			"error_message", message.Error.Message,
		)

		message.StructuredChanges = nil
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

func parseAnalysisResult(delivery amqp.Delivery) (analysisResultMessage, uuid.UUID, error) {
	var message analysisResultMessage

	if delivery.CorrelationId == "" {
		return message, uuid.Nil, ErrAnalysisResultCorrelationIDEmpty
	}

	messageID, err := uuid.Parse(delivery.CorrelationId)
	if err != nil {
		return message, uuid.Nil, err
	}

	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		return message, uuid.Nil, err
	}

	if message.AnalysisID == uuid.Nil {
		return message, uuid.Nil, ErrAnalysisResultAnalysisIDRequired
	}

	switch status := AnalysisStatus(message.Status); status {
	case AnalysisStatusDone:
		if len(message.StructuredChanges) == 0 ||
			!json.Valid(message.StructuredChanges) ||
			bytes.Equal(bytes.TrimSpace(message.StructuredChanges), []byte("null")) {
			return message, uuid.Nil, ErrAnalysisResultStructuredChangesRequired
		}
	case AnalysisStatusFailed:
		if message.Error == nil {
			return message, uuid.Nil, ErrAnalysisResultErrorRequired
		}
	default:
		return message, uuid.Nil, ErrAnalysisResultInvalidStatus
	}

	return message, messageID, nil
}
