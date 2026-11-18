package broker

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (c *Client) PublishCommand(
	ctx context.Context,
	routingKey, messageID string,
	body []byte,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.stateMu.Lock()

	err := c.ensurePublisherLocked()
	publishChannel := c.publishChannel
	returns := c.returns

	c.stateMu.Unlock()

	if err != nil {
		c.logger.Error("RabbitMQ publisher unavailable",
			"routing_key", routingKey,
			"message_id", messageID,
			"error", err,
		)
		return err
	}

	confirmation, err := publishChannel.PublishWithDeferredConfirmWithContext(
		ctx,
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

	waitCtx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()

	ack, err := confirmation.WaitContext(waitCtx)
	if err != nil {
		c.logger.Error("RabbitMQ command confirmation failed",
			"routing_key", routingKey,
			"message_id", messageID,
			"error", err,
		)
		return err
	}

	returned, closed := drainReturns(returns, messageID)

	switch {
	case closed:
		c.logger.Error("RabbitMQ publisher return channel closed",
			"routing_key", routingKey,
			"message_id", messageID,
		)
		return errPublisherReturnClosed

	case returned != nil:
		c.logger.Error("RabbitMQ could not route message",
			"routing_key", routingKey,
			"message_id", messageID,
			"reply_code", returned.ReplyCode,
			"reply_text", returned.ReplyText,
		)
		return errMessageUnroutable

	case !ack:
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
}

func (c *Client) ensurePublisherLocked() error {
	if c.conn == nil || c.conn.IsClosed() {
		if err := c.connectLocked(); err != nil {
			return err
		}
	}

	return c.openPublisherLocked()
}

func drainReturns(returns <-chan amqp.Return, messageID string) (*amqp.Return, bool) {
	var returned *amqp.Return

	for {
		select {
		case msg, ok := <-returns:
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
