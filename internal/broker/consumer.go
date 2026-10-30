package broker

import (
	"context"
	"errors"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (c *Client) Consume(
	ctx context.Context,
	queue string,
	prefetch int,
	handler func(context.Context, amqp.Delivery) error,
) error {
	if prefetch < 1 {
		return errPrefetchInvalid
	}

	delay := reconnectMinDelay

	for {
		sessionStart := time.Now()

		err := c.consumeLoop(ctx, queue, prefetch, handler)
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}

		if time.Since(sessionStart) >= consumerStableSession {
			delay = reconnectMinDelay
		}

		c.logger.Warn("RabbitMQ consumer restarted",
			"queue", queue,
			"error", err,
			"next_delay", delay,
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		if delay *= 2; delay > reconnectMaxDelay {
			delay = reconnectMaxDelay
		}
	}
}

func (c *Client) consumeLoop(
	ctx context.Context,
	queue string,
	prefetch int,
	handler func(context.Context, amqp.Delivery) error,
) error {
	c.stateMu.Lock()
	if c.conn == nil || c.conn.IsClosed() {
		if err := c.connectLocked(); err != nil {
			c.stateMu.Unlock()
			return err
		}
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
				if errors.Is(err, ErrDiscardMessage) {
					c.logger.Info("RabbitMQ message discarded",
						"queue", queue,
						"message_id", delivery.MessageId,
					)

					if ackErr := delivery.Ack(false); ackErr != nil {
						c.logger.Error("failed to ack RabbitMQ message",
							"queue", queue,
							"message_id", delivery.MessageId,
							"error", ackErr,
						)
						return ackErr
					}

					continue
				}

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
