package pullrequest

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
	ErrPullRequestResultInvalidCorrelationID  = errors.New("invalid pull request result correlation_id")
	ErrPullRequestResultPullRequestIDRequired = errors.New("pull request result pull_request_id is required")
	ErrPullRequestResultUnexpectedError       = errors.New("successful pull request result must omit error")
	ErrPullRequestResultTitleRequired         = errors.New("pull request result title is required")
	ErrPullRequestResultDescriptionRequired   = errors.New("pull request result description is required")
	ErrPullRequestResultUnexpectedFields      = errors.New("failed pull request result must omit title and description")
	ErrPullRequestResultErrorDetailsRequired  = errors.New("pull request result error code and message are required")
	ErrPullRequestResultInvalidStatus         = errors.New("invalid pull request result status")
)

type PullRequestResultConsumer struct {
	service *Service
	log     *slog.Logger
}

func NewPullRequestResultConsumer(service *Service, log *slog.Logger) *PullRequestResultConsumer {
	return &PullRequestResultConsumer{
		service: service,
		log:     log,
	}
}

func (c *PullRequestResultConsumer) Handle(ctx context.Context, delivery amqp.Delivery) error {
	message, messageID, err := parsePullRequestResult(delivery)
	if err != nil {
		c.log.Warn("pull request result message rejected",
			"queue_message_id", delivery.MessageId,
			"correlation_id", delivery.CorrelationId,
			"error", err,
		)

		return broker.ErrRejectMessage
	}

	applied, err := c.service.ApplyPullRequestResult(
		ctx,
		message.PullRequestID,
		messageID,
		message.Status,
		message.Title,
		message.Description,
	)
	if err != nil {
		return err
	}

	if !applied {
		c.log.Info("pull request result ignored",
			"pull_request_id", message.PullRequestID,
			"message_id", messageID,
			"status", message.Status,
		)
		return nil
	}

	if message.Status == generation.Failed {
		c.log.Warn("pull request generation failed",
			"pull_request_id", message.PullRequestID,
			"message_id", messageID,
			"error_code", message.Error.Code,
			"error_message", message.Error.Message,
		)
	}

	return nil
}

func parsePullRequestResult(delivery amqp.Delivery) (pullRequestResultMessage, uuid.UUID, error) {
	var message pullRequestResultMessage

	messageID, err := uuid.Parse(delivery.CorrelationId)
	if err != nil || messageID == uuid.Nil {
		return message, uuid.Nil, ErrPullRequestResultInvalidCorrelationID
	}

	if err := json.Unmarshal(delivery.Body, &message); err != nil {
		return message, uuid.Nil, err
	}

	if message.PullRequestID == uuid.Nil {
		return message, uuid.Nil, ErrPullRequestResultPullRequestIDRequired
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(delivery.Body, &fields); err != nil {
		return message, uuid.Nil, err
	}

	switch message.Status {
	case generation.Done:
		if generation.HasField(fields, "error") {
			return message, uuid.Nil, ErrPullRequestResultUnexpectedError
		}
		if message.Title == nil || strings.TrimSpace(*message.Title) == "" {
			return message, uuid.Nil, ErrPullRequestResultTitleRequired
		}
		if message.Description == nil || strings.TrimSpace(*message.Description) == "" {
			return message, uuid.Nil, ErrPullRequestResultDescriptionRequired
		}
	case generation.Failed:
		if generation.HasField(fields, "title") || generation.HasField(fields, "description") {
			return message, uuid.Nil, ErrPullRequestResultUnexpectedFields
		}
		if message.Error == nil ||
			strings.TrimSpace(message.Error.Code) == "" ||
			strings.TrimSpace(message.Error.Message) == "" {
			return message, uuid.Nil, ErrPullRequestResultErrorDetailsRequired
		}
	default:
		return message, uuid.Nil, ErrPullRequestResultInvalidStatus
	}

	return message, messageID, nil
}
