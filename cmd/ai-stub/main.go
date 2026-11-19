package main

import (
	"bytes"
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
	"github.com/pp-sem7-team/backend/internal/generation"
	"github.com/pp-sem7-team/backend/internal/logger"
)

const (
	analysisRequestsQueue = "generation.analysis.requests"
	commitRequestsQueue   = "generation.commit.requests"
	resultsExchange       = "generation.results"
	analysisResultKey     = "analysis.result"
	commitResultKey       = "commit.result"
	prefetch              = 1
	retryDelay            = 2 * time.Second
)

var errPermanent = errors.New("permanent message error")

type analysisRequest struct {
	AnalysisID uuid.UUID `json:"analysis_id"`
	RawDiff    string    `json:"raw_diff"`
}

type analysisResult struct {
	AnalysisID        uuid.UUID         `json:"analysis_id"`
	Status            string            `json:"status"`
	StructuredChanges json.RawMessage   `json:"structured_changes,omitempty"`
	Error             *generation.Error `json:"error,omitempty"`
}

type commitRequest struct {
	CommitID          uuid.UUID       `json:"commit_id"`
	StructuredChanges json.RawMessage `json:"structured_changes"`
}

type commitResult struct {
	CommitID uuid.UUID         `json:"commit_id"`
	Status   string            `json:"status"`
	Type     string            `json:"type,omitempty"`
	Scope    json.RawMessage   `json:"scope,omitempty"`
	Message  string            `json:"message,omitempty"`
	Error    *generation.Error `json:"error,omitempty"`
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

	analysisDeliveries, err := consumeCh.Consume(analysisRequestsQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", analysisRequestsQueue, err)
	}

	commitDeliveries, err := consumeCh.Consume(commitRequestsQueue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", commitRequestsQueue, err)
	}

	returns := publishCh.NotifyReturn(make(chan amqp.Return, 1))
	s.log.Info("ai stub connected",
		"analysis_queue", analysisRequestsQueue,
		"commit_queue", commitRequestsQueue,
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-analysisDeliveries:
			if !ok {
				return errors.New("analysis deliveries channel closed")
			}
			s.handleDelivery(ctx, publishCh, returns, delivery, analysisResultKey)
		case delivery, ok := <-commitDeliveries:
			if !ok {
				return errors.New("commit deliveries channel closed")
			}
			s.handleDelivery(ctx, publishCh, returns, delivery, commitResultKey)
		}
	}
}

func (s *stub) handleDelivery(ctx context.Context, ch *amqp.Channel, returns <-chan amqp.Return, delivery amqp.Delivery, resultKey string) {
	if err := s.process(ctx, ch, returns, delivery, resultKey); err != nil {
		requeue := !errors.Is(err, errPermanent)

		s.log.Error("failed to process generation request",
			"error", err,
			"result_type", resultKey,
			"requeue", requeue,
			"message_id", delivery.MessageId,
			"correlation_id", delivery.CorrelationId,
		)

		if nackErr := delivery.Nack(false, requeue); nackErr != nil {
			s.log.Error("failed to nack generation request",
				"error", nackErr,
				"message_id", delivery.MessageId,
			)
		}
		return
	}

	if err := delivery.Ack(false); err != nil {
		s.log.Error("failed to ack generation request",
			"error", err,
			"message_id", delivery.MessageId,
		)
	}
}

func (s *stub) process(ctx context.Context, ch *amqp.Channel, returns <-chan amqp.Return, delivery amqp.Delivery, resultKey string) error {
	messageID, err := uuid.Parse(delivery.CorrelationId)
	if err != nil || messageID == uuid.Nil || delivery.CorrelationId != delivery.MessageId {
		return fmt.Errorf("%w: invalid request correlation id", errPermanent)
	}

	status := s.nextStatus()
	var body []byte
	switch resultKey {
	case analysisResultKey:
		body, err = makeAnalysisResult(delivery.Body, status)
	case commitResultKey:
		body, err = makeCommitResult(delivery.Body, status)
	}
	if err != nil {
		return err
	}

	if err := s.wait(ctx); err != nil {
		return err
	}

	confirmation, err := ch.PublishWithDeferredConfirmWithContext(
		ctx,
		resultsExchange,
		resultKey,
		true,
		false,
		amqp.Publishing{
			ContentType:     "application/json",
			ContentEncoding: "utf-8",
			DeliveryMode:    amqp.Persistent,
			MessageId:       uuid.NewString(),
			CorrelationId:   delivery.CorrelationId,
			Type:            resultKey,
			Timestamp:       time.Now(),
			AppId:           "ai-service",
			Body:            body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish generation result: %w", err)
	}

	if confirmation == nil {
		return errors.New("publisher confirms are disabled")
	}

	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("wait for publish confirmation: %w", err)
	}

	if !acked {
		return errors.New("generation result publish was nacked")
	}

	select {
	case returned, ok := <-returns:
		if !ok {
			return errors.New("publisher return channel closed")
		}
		return fmt.Errorf("generation result could not be routed: %s", returned.ReplyText)
	default:
	}

	s.log.Info("generation result published",
		"result_type", resultKey,
		"status", status,
		"correlation_id", delivery.CorrelationId,
	)

	return nil
}

func makeAnalysisResult(body []byte, status string) ([]byte, error) {
	var request analysisRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("%w: invalid payload: %v", errPermanent, err)
	}
	if request.AnalysisID == uuid.Nil || strings.TrimSpace(request.RawDiff) == "" {
		return nil, fmt.Errorf("%w: analysis_id and raw_diff are required", errPermanent)
	}

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
			return nil, err
		}
		result.StructuredChanges = structured
	} else {
		result.Error = &generation.Error{
			Code:    "internal_error",
			Message: "simulated analysis failure",
		}
	}
	return json.Marshal(result)
}

func makeCommitResult(body []byte, status string) ([]byte, error) {
	var request commitRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, fmt.Errorf("%w: invalid payload: %v", errPermanent, err)
	}
	if request.CommitID == uuid.Nil || len(request.StructuredChanges) == 0 ||
		bytes.Equal(bytes.TrimSpace(request.StructuredChanges), []byte("null")) {
		return nil, fmt.Errorf("%w: commit_id and structured_changes are required", errPermanent)
	}

	result := commitResult{
		CommitID: request.CommitID,
		Status:   status,
	}
	if status == "done" {
		result.Type = "chore"
		result.Scope = json.RawMessage("null")
		result.Message = "update project files"
	} else {
		result.Error = &generation.Error{
			Code:    "internal_error",
			Message: "simulated commit generation failure",
		}
	}
	return json.Marshal(result)
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
