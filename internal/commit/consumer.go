package commit

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/pp-sem7-team/backend/internal/broker"
	"github.com/pp-sem7-team/backend/internal/generation"
)

var (
	ErrCommitResultInvalidCorrelationID = errors.New("invalid commit result correlation_id")
	ErrCommitResultCommitIDRequired     = errors.New("commit result commit_id is required")
	ErrCommitResultUnexpectedError      = errors.New("successful commit result must omit error")
	ErrCommitResultInvalidType          = errors.New("invalid commit result type")
	ErrCommitResultScopeRequired        = errors.New("commit result scope is required for done status")
	ErrCommitResultMessageRequired      = errors.New("commit result message is required")
	ErrCommitResultUnexpectedFields     = errors.New("failed commit result must omit type, scope and message")
	ErrCommitResultErrorDetailsRequired = errors.New("commit result error code and message are required")
	ErrCommitResultInvalidStatus        = errors.New("invalid commit result status")
)

type CommitResultConsumer struct {
	service *Service
	log     *slog.Logger
}

func NewCommitResultConsumer(service *Service, log *slog.Logger) *CommitResultConsumer {
	return &CommitResultConsumer{
		service: service,
		log:     log,
	}
}

func (c *CommitResultConsumer) Handle(ctx context.Context, delivery amqp.Delivery) error {
	message, messageID, err := parseCommitResult(delivery)
	if err != nil {
		c.log.Warn("commit result message rejected",
			"queue_message_id", delivery.MessageId,
			"correlation_id", delivery.CorrelationId,
			"error", err,
		)

		return broker.ErrRejectMessage
	}

	applied, err := c.service.ApplyCommitResult(
		ctx,
		message.CommitID,
		messageID,
		message.Status,
		message.Type,
		message.Scope,
		message.Message,
	)
	if err != nil {
		return err
	}

	if !applied {
		c.log.Info("commit result ignored",
			"commit_id", message.CommitID,
			"message_id", messageID,
			"status", message.Status,
		)
		return nil
	}

	if message.Status == generation.Failed {
		c.log.Warn("commit generation failed",
			"commit_id", message.CommitID,
			"message_id", messageID,
			"error_code", message.Error.Code,
			"error_message", message.Error.Message,
		)
	}

	return nil
}

func parseCommitResult(delivery amqp.Delivery) (commitResultMessage, uuid.UUID, error) {
	var message commitResultMessage

	messageID, err := uuid.Parse(delivery.CorrelationId)
	if err != nil || messageID == uuid.Nil {
		return message, uuid.Nil, ErrCommitResultInvalidCorrelationID
	}

	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		return message, uuid.Nil, err
	}

	if message.CommitID == uuid.Nil {
		return message, uuid.Nil, ErrCommitResultCommitIDRequired
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(delivery.Body, &fields); err != nil {
		return message, uuid.Nil, err
	}

	switch message.Status {
	case generation.Done:
		if generation.HasField(fields, "error") {
			return message, uuid.Nil, ErrCommitResultUnexpectedError
		}
		if message.Type == nil || !message.Type.IsValid() {
			return message, uuid.Nil, ErrCommitResultInvalidType
		}
		if !generation.HasField(fields, "scope") {
			return message, uuid.Nil, ErrCommitResultScopeRequired
		}
		if message.Message == nil || strings.TrimSpace(*message.Message) == "" {
			return message, uuid.Nil, ErrCommitResultMessageRequired
		}
	case generation.Failed:
		if generation.HasField(fields, "type") ||
			generation.HasField(fields, "scope") ||
			generation.HasField(fields, "message") {
			return message, uuid.Nil, ErrCommitResultUnexpectedFields
		}
		if message.Error == nil || strings.TrimSpace(message.Error.Code) == "" || strings.TrimSpace(message.Error.Message) == "" {
			return message, uuid.Nil, ErrCommitResultErrorDetailsRequired
		}
	default:
		return message, uuid.Nil, ErrCommitResultInvalidStatus
	}

	return message, messageID, nil
}
