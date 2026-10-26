package broker

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	commandsExchange   = "generation.commands"
	resultsExchange    = "generation.results"
	deadLetterExchange = "generation.dlx"

	analysisRequestsQueue    = "generation.analysis.requests"
	commitRequestsQueue      = "generation.commit.requests"
	pullRequestRequestsQueue = "generation.pull_request.requests"
	analysisResultsQueue     = "generation.analysis.results"
	commitResultsQueue       = "generation.commit.results"
	pullRequestResultsQueue  = "generation.pull_request.results"
)

func (c *Client) declareTopology() error {
	exchanges := []string{
		commandsExchange,
		resultsExchange,
		deadLetterExchange,
	}

	for _, exchange := range exchanges {
		if err := c.channel.ExchangeDeclare(
			exchange,
			"direct",
			true,
			false,
			false,
			false,
			nil,
		); err != nil {
			return err
		}
	}

	queues := []topologyItem{
		{analysisRequestsQueue, commandsExchange, "analysis.request"},
		{commitRequestsQueue, commandsExchange, "commit.request"},
		{pullRequestRequestsQueue, commandsExchange, "pull_request.request"},
		{analysisResultsQueue, resultsExchange, "analysis.result"},
		{commitResultsQueue, resultsExchange, "commit.result"},
		{pullRequestResultsQueue, resultsExchange, "pull_request.result"},
	}

	for _, q := range queues {
		args := amqp.Table{
			"x-queue-type":              "quorum",
			"x-dead-letter-exchange":    deadLetterExchange,
			"x-dead-letter-routing-key": q.queue + ".dlq",
			"x-delivery-limit":          int32(5),
		}

		if _, err := c.channel.QueueDeclare(
			q.queue,
			true,
			false,
			false,
			false,
			args,
		); err != nil {
			return err
		}

		if err := c.channel.QueueBind(
			q.queue,
			q.routingKey,
			q.exchange,
			false,
			nil,
		); err != nil {
			return err
		}

		dlqName := q.queue + ".dlq"

		if _, err := c.channel.QueueDeclare(
			dlqName,
			true,
			false,
			false,
			false,
			amqp.Table{"x-queue-type": "quorum"},
		); err != nil {
			return err
		}

		if err := c.channel.QueueBind(
			dlqName,
			dlqName,
			deadLetterExchange,
			false,
			nil,
		); err != nil {
			return err
		}
	}

	c.logger.Info("RabbitMQ topology declared")
	return nil
}
