package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/joho/godotenv"
	"github.com/pp-sem7-team/backend/internal/logger"
)

const (
	analysisRequestsQueue = "generation.analysis.requests"
	resultsExchange       = "generation.results"
	analysisResultKey     = "analysis.result"
	prefetch              = 1
	retryDelay            = 2 * time.Second
)

var errPermanent = errors.New("permanent message error")

type analysisRequest struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
	RawDiff    string    `json:"raw_diff"`
}

type analysisResult struct {
	AnalysisID        uuid.UUID        `json:"analysis_id"`
	Status            string           `json:"status"`
	StructuredChanges json.RawMessage  `json:"structured_changes,omitempty"`
	Error             *generationError `json:"error,omitempty"`
}

type generationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type structuredChanges struct {
	Summary string   `json:"summary"`
	Files   []string `json:"files"`
}

type stubOptions struct {
	mode        string
	successRate float64
	delay       time.Duration
	seed        int64
}

type rabbitMQConfig struct {
	host     string
	port     string
	user     string
	password string
	vhost    string
}

type stub struct {
	options stubOptions
	rng     *rand.Rand
	log     *slog.Logger
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return err
	}

	result := flag.String("result", "random", "result mode: random, done or failed")
	successRate := flag.Float64("success-rate", 0.8, "probability of done in random mode (0.0-1.0)")
	delay := flag.Duration("delay", time.Second, "delay before publishing result")
	seed := flag.Int64("seed", time.Now().UnixNano(), "random seed for reproducible decisions")

	flag.Parse()

	rmqCfg := rabbitMQConfig{
		host:     "localhost",
		port:     getEnv("RABBITMQ_PORT", "5672"),
		user:     getEnv("RABBITMQ_USER", "gitflow"),
		password: getEnv("RABBITMQ_PASSWORD", ""),
		vhost:    getEnv("RABBITMQ_VHOST", "/"),
	}

	if rmqCfg.password == "" {
		return fmt.Errorf("RABBITMQ_PASSWORD is required (set in .env or environment)")
	}

	mode := strings.ToLower(strings.TrimSpace(*result))
	switch mode {
	case "random", "done", "failed":
	default:
		return fmt.Errorf("invalid -result %q: expected random, done or failed", mode)
	}

	if *successRate < 0 || *successRate > 1 {
		return fmt.Errorf("invalid -success-rate %v: expected value between 0 and 1", *successRate)
	}

	if *delay < 0 {
		return fmt.Errorf("invalid -delay %v: duration must not be negative", *delay)
	}

	cfg := stubOptions{
		mode:        mode,
		successRate: *successRate,
		delay:       *delay,
		seed:        *seed,
	}

	log := logger.New(slog.LevelInfo)

	st := &stub{
		options: cfg,
		rng:     rand.New(rand.NewSource(cfg.seed)),
		log:     log,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info("ai stub starting",
		"mode", st.options.mode,
		"success_rate", st.options.successRate,
		"delay", st.options.delay,
		"seed", st.options.seed,
		"rmq_host", rmqCfg.host,
		"rmq_port", rmqCfg.port,
	)

	for {
		err := st.run(ctx, rmqCfg)
		if ctx.Err() != nil {
			return nil
		}

		log.Error("ai stub stopped", "error", err)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(retryDelay):
		}
	}
}

func (s *stub) run(ctx context.Context, cfg rabbitMQConfig) error {
	conn, err := amqp.Dial(rabbitURL(cfg))
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	defer conn.Close()

	consumeCh, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open consumer channel: %w", err)
	}
	defer consumeCh.Close()

	publishCh, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open publisher channel: %w", err)
	}
	defer publishCh.Close()

	if err := publishCh.Confirm(false); err != nil {
		return fmt.Errorf("enable publisher confirms: %w", err)
	}

	if err := consumeCh.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("configure consumer QoS: %w", err)
	}

	deliveries, err := consumeCh.Consume(
		analysisRequestsQueue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume %s: %w", analysisRequestsQueue, err)
	}

	s.log.Info("ai stub connected", "queue", analysisRequestsQueue)

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return errors.New("RabbitMQ deliveries channel closed")
			}

			s.handleDelivery(ctx, publishCh, delivery)
		}
	}
}

func (s *stub) handleDelivery(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery) {
	if err := s.process(ctx, ch, delivery); err != nil {
		requeue := !errors.Is(err, errPermanent)

		s.log.Error("failed to process analysis request",
			"error", err,
			"requeue", requeue,
			"message_id", delivery.MessageId,
			"correlation_id", delivery.CorrelationId,
		)

		if nackErr := delivery.Nack(false, requeue); nackErr != nil {
			s.log.Error("failed to nack analysis request",
				"error", nackErr,
				"message_id", delivery.MessageId,
			)
		}
		return
	}

	if err := delivery.Ack(false); err != nil {
		s.log.Error("failed to ack analysis request",
			"error", err,
			"message_id", delivery.MessageId,
		)
	}
}

func (s *stub) process(ctx context.Context, ch *amqp.Channel, delivery amqp.Delivery) error {
	if delivery.CorrelationId == "" {
		return fmt.Errorf("%w: correlation id is empty", errPermanent)
	}

	if _, err := uuid.Parse(delivery.CorrelationId); err != nil {
		return fmt.Errorf("%w: invalid correlation id: %v", errPermanent, err)
	}

	var request analysisRequest
	if err := json.Unmarshal(delivery.Body, &request); err != nil {
		return fmt.Errorf("%w: invalid payload: %v", errPermanent, err)
	}

	if request.AnalysisID == uuid.Nil {
		return fmt.Errorf("%w: analysis_id is required", errPermanent)
	}

	if err := s.wait(ctx); err != nil {
		return err
	}

	status := s.nextStatus()
	result := analysisResult{
		AnalysisID: request.AnalysisID,
		Status:     status,
	}

	if status == "done" {
		structured, err := json.Marshal(structuredChanges{
			Summary: "stub analysis result",
			Files:   []string{"main.go"},
		})
		if err != nil {
			return err
		}
		result.StructuredChanges = structured
	} else {
		result.Error = &generationError{
			Code:    "STUB_FAILURE",
			Message: "simulated analysis failure",
		}
	}

	body, err := json.Marshal(result)
	if err != nil {
		return err
	}

	confirmation, err := ch.PublishWithDeferredConfirm(
		resultsExchange,
		analysisResultKey,
		false,
		false,
		amqp.Publishing{
			ContentType:     "application/json",
			ContentEncoding: "utf-8",
			DeliveryMode:    amqp.Persistent,
			MessageId:       uuid.NewString(),
			CorrelationId:   delivery.CorrelationId,
			Type:            analysisResultKey,
			Timestamp:       time.Now(),
			AppId:           "ai-stub",
			Body:            body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish analysis result: %w", err)
	}

	if confirmation == nil {
		return errors.New("publisher confirms are disabled")
	}

	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for publish confirmation: %w", err)
	}

	if !acked {
		return errors.New("analysis result publish was nacked")
	}

	s.log.Info("analysis result published",
		"analysis_id", request.AnalysisID,
		"status", status,
		"correlation_id", delivery.CorrelationId,
		"raw_diff_bytes", len(request.RawDiff),
	)

	return nil
}

func (s *stub) wait(ctx context.Context) error {
	if s.options.delay <= 0 {
		return nil
	}

	timer := time.NewTimer(s.options.delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *stub) nextStatus() string {
	if s.options.mode == "done" || s.options.mode == "failed" {
		return s.options.mode
	}

	if s.rng.Float64() < s.options.successRate {
		return "done"
	}

	return "failed"
}

func rabbitURL(cfg rabbitMQConfig) string {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(cfg.user, cfg.password),
		Host:   net.JoinHostPort(cfg.host, cfg.port),
		Path:   "/" + cfg.vhost,
	}

	return u.String()
}

func getEnv(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}

	return value
}
