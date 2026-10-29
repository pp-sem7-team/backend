package broker

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/pp-sem7-team/backend/internal/config"
)

var (
	errPublisherUnavailable   = errors.New("RabbitMQ publisher channel is unavailable")
	errPublisherReturnClosed  = errors.New("RabbitMQ publisher return channel closed")
	errPublisherConfirmClosed = errors.New("RabbitMQ publisher confirmation channel closed")
	errPublisherNack          = errors.New("RabbitMQ negatively acknowledged message")
	errMessageUnroutable      = errors.New("RabbitMQ could not route message")

	errConnectionUnavailable = errors.New("RabbitMQ connection is unavailable")
	errPrefetchInvalid       = errors.New("prefetch must be greater than zero")
	errConsumerChannelClosed = errors.New("RabbitMQ consumer channel closed")
)

const publishTimeout = 2 * time.Second

type Client struct {
	conn           *amqp.Connection
	channel        *amqp.Channel
	publishChannel *amqp.Channel
	confirms       chan amqp.Confirmation
	returns        chan amqp.Return
	publishMu      sync.Mutex
	logger         *slog.Logger
}

func NewClient(cfg config.RabbitMQConfig, logger *slog.Logger) (*Client, error) {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   "/" + cfg.Vhost,
	}

	logger.Info("creating RabbitMQ connection")

	conn, err := amqp.Dial(u.String())
	if err != nil {
		logger.Error("failed to create RabbitMQ connection", "error", err)
		return nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		logger.Error("failed to create RabbitMQ channel", "error", err)
		return nil, err
	}

	c := &Client{
		conn:    conn,
		channel: channel,
		logger:  logger,
	}

	if err := c.declareTopology(); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		logger.Error("failed to declare RabbitMQ topology", "error", err)
		return nil, err
	}

	publishChannel, err := conn.Channel()
	if err != nil {
		_ = channel.Close()
		_ = conn.Close()
		logger.Error("failed to create RabbitMQ publisher channel", "error", err)
		return nil, err
	}

	if err := publishChannel.Confirm(false); err != nil {
		_ = publishChannel.Close()
		_ = channel.Close()
		_ = conn.Close()
		logger.Error("failed to enable RabbitMQ publisher confirms", "error", err)
		return nil, err
	}

	c.publishChannel = publishChannel
	c.confirms = publishChannel.NotifyPublish(make(chan amqp.Confirmation, 32))
	c.returns = publishChannel.NotifyReturn(make(chan amqp.Return, 32))

	logger.Info("RabbitMQ connection established")

	return c, nil
}

func (c *Client) Close() {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()

	if c.publishChannel != nil {
		if err := c.publishChannel.Close(); err != nil {
			c.logger.Error("failed to close RabbitMQ publisher channel", "error", err)
		}
	}

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			c.logger.Error("failed to close RabbitMQ channel", "error", err)
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			c.logger.Error("failed to close RabbitMQ connection", "error", err)
			return
		}
	}

	c.logger.Info("RabbitMQ connection closed")
}

func (c *Client) PublishCommand(
	ctx context.Context,
	routingKey, messageID string,
	body []byte,
) error {
	c.publishMu.Lock()
	defer c.publishMu.Unlock()

	if c.publishChannel == nil {
		return errPublisherUnavailable
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	timer := time.NewTimer(publishTimeout)
	defer timer.Stop()

	err := c.publishChannel.Publish(
		commandsExchange,
		routingKey,
		true,
		false,
		amqp.Publishing{
			ContentType:     "application/json",
			ContentEncoding: "utf-8",
			DeliveryMode:    amqp.Persistent,
			MessageId:       messageID,
			CorrelationId:   messageID,
			Type:            routingKey,
			Timestamp:       time.Now(),
			AppId:           "backend",
			Body:            body,
		},
	)
	if err != nil {
		c.logger.Error("failed to publish RabbitMQ command",
			"routing_key", routingKey,
			"message_id", messageID,
			"error", err,
		)
		return err
	}

	var returned *amqp.Return

	for {
		select {
		case msg, ok := <-c.returns:
			if !ok {
				c.invalidatePublisher()
				c.logger.Error("RabbitMQ publisher return channel closed",
					"routing_key", routingKey,
					"message_id", messageID,
				)
				return errPublisherReturnClosed
			}

			if msg.MessageId == messageID {
				returned = &msg
			}

		case confirmation, ok := <-c.confirms:
			if !ok {
				c.invalidatePublisher()
				c.logger.Error("RabbitMQ publisher confirmation channel closed",
					"routing_key", routingKey,
					"message_id", messageID,
				)
				return errPublisherConfirmClosed
			}

			pendingReturn, closed := c.drainReturns(messageID)
			if closed {
				c.invalidatePublisher()
				c.logger.Error("RabbitMQ publisher return channel closed",
					"routing_key", routingKey,
					"message_id", messageID,
				)
				return errPublisherReturnClosed
			}
			if pendingReturn != nil {
				returned = pendingReturn
			}

			if returned != nil {
				c.logger.Error("RabbitMQ could not route message",
					"routing_key", routingKey,
					"message_id", messageID,
					"reply_code", returned.ReplyCode,
					"reply_text", returned.ReplyText,
				)
				return errMessageUnroutable
			}

			if !confirmation.Ack {
				c.logger.Error("RabbitMQ negatively acknowledged message",
					"routing_key", routingKey,
					"message_id", messageID,
				)
				return errPublisherNack
			}

			c.logger.Info("RabbitMQ command published",
				"routing_key", routingKey,
				"message_id", messageID,
			)
			return nil

		case <-ctx.Done():
			c.invalidatePublisher()
			return ctx.Err()

		case <-timer.C:
			c.invalidatePublisher()
			c.logger.Error("RabbitMQ command publish timed out",
				"routing_key", routingKey,
				"message_id", messageID,
			)
			return context.DeadlineExceeded
		}
	}
}

func (c *Client) Consume(
	ctx context.Context,
	queue string,
	prefetch int,
	handler func(context.Context, amqp.Delivery) error,
) error {
	if prefetch < 1 {
		return errPrefetchInvalid
	}

	c.publishMu.Lock()
	if c.conn == nil || c.conn.IsClosed() {
		c.publishMu.Unlock()
		return errConnectionUnavailable
	}

	channel, err := c.conn.Channel()
	c.publishMu.Unlock()

	if err != nil {
		c.logger.Error("failed to create RabbitMQ consumer channel",
			"queue", queue,
			"error", err,
		)
		return err
	}

	defer func() {
		if err := channel.Close(); err != nil {
			c.logger.Error("failed to close RabbitMQ consumer channel",
				"queue", queue,
				"error", err,
			)
		}
	}()

	if err := channel.Qos(prefetch, 0, false); err != nil {
		c.logger.Error("failed to configure RabbitMQ consumer QoS",
			"queue", queue,
			"prefetch", prefetch,
			"error", err,
		)
		return err
	}

	deliveries, err := channel.Consume(
		queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		c.logger.Error("failed to consume RabbitMQ queue",
			"queue", queue,
			"error", err,
		)
		return err
	}

	c.logger.Info("RabbitMQ consumer started", "queue", queue)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()

		case delivery, ok := <-deliveries:
			if !ok {
				if err := ctx.Err(); err != nil {
					return err
				}

				c.logger.Error("RabbitMQ consumer channel closed",
					"queue", queue,
				)
				return errConsumerChannelClosed
			}

			if err := handler(ctx, delivery); err != nil {
				c.logger.Error("failed to process RabbitMQ message",
					"queue", queue,
					"message_id", delivery.MessageId,
					"error", err,
				)

				if nackErr := delivery.Nack(false, true); nackErr != nil {
					c.logger.Error("failed to nack RabbitMQ message",
						"queue", queue,
						"message_id", delivery.MessageId,
						"error", nackErr,
					)
					return nackErr
				}

				continue
			}

			if err := delivery.Ack(false); err != nil {
				c.logger.Error("failed to ack RabbitMQ message",
					"queue", queue,
					"message_id", delivery.MessageId,
					"error", err,
				)
				return err
			}
		}
	}
}

func (c *Client) invalidatePublisher() {
	if c.publishChannel == nil {
		return
	}

	if err := c.publishChannel.Close(); err != nil {
		c.logger.Error("failed to close RabbitMQ publisher channel", "error", err)
	}

	c.publishChannel = nil
	c.confirms = nil
	c.returns = nil
}

func (c *Client) drainReturns(messageID string) (*amqp.Return, bool) {
	var returned *amqp.Return

	for {
		select {
		case msg, ok := <-c.returns:
			if !ok {
				return returned, true
			}

			if msg.MessageId == messageID {
				returned = &msg
			}

		default:
			return returned, false
		}
	}
}
