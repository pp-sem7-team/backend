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
	errPublisherReturnClosed  = errors.New("RabbitMQ publisher return channel closed")
	errPublisherConfirmClosed = errors.New("RabbitMQ publisher confirmation channel closed")
	errPublisherNack          = errors.New("RabbitMQ negatively acknowledged message")
	errMessageUnroutable      = errors.New("RabbitMQ could not route message")

	errConnectionUnavailable = errors.New("RabbitMQ connection is unavailable")
	errReconnectPending      = errors.New("RabbitMQ reconnect is pending")
	errPrefetchInvalid       = errors.New("prefetch must be greater than zero")
	errConsumerChannelClosed = errors.New("RabbitMQ consumer channel closed")
)

const (
	publishTimeout = 2 * time.Second

	notifyBufferSize = 32

	reconnectMinDelay = time.Second
	reconnectMaxDelay = 30 * time.Second
)

type Client struct {
	amqpURL string
	logger  *slog.Logger

	conn           *amqp.Connection
	channel        *amqp.Channel
	publishChannel *amqp.Channel
	confirms       chan amqp.Confirmation
	returns        chan amqp.Return

	stateMu sync.Mutex

	reconnectDelay time.Duration
	reconnectAfter time.Time
}

func NewClient(cfg config.RabbitMQConfig, logger *slog.Logger) (*Client, error) {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   "/" + cfg.Vhost,
	}

	c := &Client{
		amqpURL: u.String(),
		logger:  logger,
	}

	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	logger.Info("creating RabbitMQ connection")

	if err := c.connectLocked(); err != nil {
		logger.Error("failed to create RabbitMQ connection", "error", err)
		return nil, err
	}

	if err := c.openPublisherLocked(); err != nil {
		c.resetLocked()
		logger.Error("failed to create RabbitMQ publisher channel", "error", err)
		return nil, err
	}

	logger.Info("RabbitMQ connection established")

	return c, nil
}

func (c *Client) connectLocked() error {
	if c.reconnectDelay > 0 && time.Now().Before(c.reconnectAfter) {
		return errReconnectPending
	}

	if c.conn != nil && !c.conn.IsClosed() {
		return nil
	}

	c.resetLocked()

	conn, err := amqp.Dial(c.amqpURL)
	if err != nil {
		c.logger.Error("failed to dial RabbitMQ", "error", err)
		c.backoffLocked()
		return err
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		c.logger.Error("failed to create RabbitMQ channel", "error", err)
		c.backoffLocked()
		return err
	}

	c.conn = conn
	c.channel = channel

	if err := c.declareTopology(); err != nil {
		c.logger.Error("failed to declare RabbitMQ topology", "error", err)
		c.resetLocked()
		c.backoffLocked()
		return err
	}

	c.reconnectDelay = 0
	c.reconnectAfter = time.Time{}

	c.watchConnectionLocked(conn)

	return nil
}

func (c *Client) openPublisherLocked() error {
	if c.publishChannel != nil {
		return nil
	}

	if c.conn == nil || c.conn.IsClosed() {
		return errConnectionUnavailable
	}

	channel, err := c.conn.Channel()
	if err != nil {
		return err
	}

	if err := channel.Confirm(false); err != nil {
		_ = channel.Close()
		return err
	}

	c.publishChannel = channel
	c.confirms = channel.NotifyPublish(make(chan amqp.Confirmation, notifyBufferSize))
	c.returns = channel.NotifyReturn(make(chan amqp.Return, notifyBufferSize))

	c.watchPublishChannelLocked(channel)

	return nil
}

func (c *Client) resetLocked() {
	if c.publishChannel != nil {
		_ = c.publishChannel.Close()
	}

	if c.channel != nil {
		_ = c.channel.Close()
	}

	if c.conn != nil {
		_ = c.conn.Close()
	}

	c.conn = nil
	c.channel = nil
	c.publishChannel = nil
	c.confirms = nil
	c.returns = nil
}

func (c *Client) backoffLocked() {
	if c.reconnectDelay < reconnectMinDelay {
		c.reconnectDelay = reconnectMinDelay
	} else if c.reconnectDelay *= 2; c.reconnectDelay > reconnectMaxDelay {
		c.reconnectDelay = reconnectMaxDelay
	}

	c.reconnectAfter = time.Now().Add(c.reconnectDelay)

	c.logger.Warn("RabbitMQ reconnect scheduled", "delay", c.reconnectDelay)
}

func (c *Client) watchConnectionLocked(conn *amqp.Connection) {
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))

	go func() {
		reason, received := <-closeCh

		c.stateMu.Lock()
		defer c.stateMu.Unlock()

		if c.conn != conn {
			return
		}

		if received && reason != nil {
			c.logger.Error("RabbitMQ connection closed", "error", reason)
		}

		c.conn = nil
		c.channel = nil
		c.publishChannel = nil
		c.confirms = nil
		c.returns = nil
	}()
}

func (c *Client) watchPublishChannelLocked(channel *amqp.Channel) {
	closeCh := channel.NotifyClose(make(chan *amqp.Error, 1))

	go func() {
		reason, received := <-closeCh

		c.stateMu.Lock()
		defer c.stateMu.Unlock()

		if c.publishChannel != channel {
			return
		}

		if received && reason != nil {
			c.logger.Error("RabbitMQ publisher channel closed", "error", reason)
		}

		c.publishChannel = nil
		c.confirms = nil
		c.returns = nil
	}()
}

func (c *Client) Close() {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

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
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := c.ensurePublisherLocked(); err != nil {
		c.logger.Error("RabbitMQ publisher unavailable",
			"routing_key", routingKey,
			"message_id", messageID,
			"error", err,
		)
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
				c.invalidatePublisherLocked()
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
				c.invalidatePublisherLocked()
				c.logger.Error("RabbitMQ publisher confirmation channel closed",
					"routing_key", routingKey,
					"message_id", messageID,
				)
				return errPublisherConfirmClosed
			}

			pendingReturn, closed := c.drainReturns(messageID)
			if closed {
				c.invalidatePublisherLocked()
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
			c.invalidatePublisherLocked()
			return ctx.Err()

		case <-timer.C:
			c.invalidatePublisherLocked()
			c.logger.Error("RabbitMQ command publish timed out",
				"routing_key", routingKey,
				"message_id", messageID,
			)
			return context.DeadlineExceeded
		}
	}
}

func (c *Client) ensurePublisherLocked() error {
	if c.conn == nil || c.conn.IsClosed() {
		if err := c.connectLocked(); err != nil {
			return err
		}
	}

	return c.openPublisherLocked()
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

	c.stateMu.Lock()
	if c.conn == nil || c.conn.IsClosed() {
		c.stateMu.Unlock()
		return errConnectionUnavailable
	}

	channel, err := c.conn.Channel()
	c.stateMu.Unlock()

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

func (c *Client) invalidatePublisherLocked() {
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
