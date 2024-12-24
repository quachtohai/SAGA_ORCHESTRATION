package main

import (
	"context"
	"fmt"
	"net/http"
	"orchestration/cmd/local/orchestrator/adapters/repositories/executions"
	"orchestration/cmd/local/orchestrator/workflows"
	"orchestration/pkg/validator"
	"strings"

	workflowrepo "orchestration/cmd/local/orchestrator/adapters/repositories/workflows"
	"orchestration/cmd/local/orchestrator/api"
	"orchestration/cmd/local/orchestrator/config/env"
	"orchestration/internal/adapters/infra/kv"
	"orchestration/internal/config/logger"
	"orchestration/internal/saga"
	"orchestration/internal/streaming"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	var (
		ctx, cancel = context.WithCancel(context.Background())
		sigCh       = make(chan os.Signal, 1)
		errCh       = make(chan error, 1)
	)

	defer cancel()
	defer close(sigCh)
	defer close(errCh)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	cfg, err := env.Load()
	if err != nil {
		panic(err)
	}
	lggr := logger.New(cfg.ServiceName)

	defer lggr.Sync()

	redisConn := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})

	err = redisConn.Ping(ctx).Err()
	if err != nil {
		lggr.With(zap.Error(err)).Fatal("Got error connecting to Redis")
	}

	dbpool, err := pgxpool.New(context.Background(), cfg.DBConnectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()
	lggr.Info("Connected to database")

	var (
		workflows = []saga.Workflow{
			*workflows.NewCreateOrderV1(lggr),
		}
		workflowRepository = workflowrepo.NewInmemRepository(workflows)
	)

	var (
		executionsRepository = executions.NewRepositoryAdapter(lggr, dbpool, workflowRepository)
		topics               = strings.Split(cfg.Topics, ",")
		publisher            = newPublisher(lggr)
		workflowService      = saga.NewService(lggr, executionsRepository, publisher)
		idempotenceService   = kv.NewAdapter(lggr, redisConn)
		messageHandler       = streaming.NewMessageHandler(lggr, executionsRepository, workflowService, idempotenceService)
	)
	fmt.Print(topics)

	var (
		val         = validator.New()
		apiHandlers = api.NewHandlers(lggr, workflowRepository, workflowService, val)
		httpServer  = newApiServer(":3000", apiHandlers)
	)

	go func() {
		for _, topicExchange := range topics {
			consumer, err := newConsumer(lggr, topicExchange, messageHandler)

			if err != nil {
				lggr.With(zap.Error(err)).Fatal("Got error creating consumer")
				lggr.Info("CONSUME MESSAGE")

			}
			lggr.Infof("Starting orchestrator consumer go routine" + topicExchange)
			go func() {
				if err := consumer.Start(ctx); err != nil {
					lggr.With(zap.Error(err)).Fatal("Got in consumer")
					errCh <- err
				}
			}()

		}

	}()

	go func() {
		lggr.Info("Starting API server go routine")
		if err := httpServer.ListenAndServe(); err != nil {
			lggr.With(zap.Error(err)).Fatal("Got error in API server")
			errCh <- err
		}
	}()

	lggr.Info("Running. Waiting for signal to stop...")
	select {
	case <-sigCh:
		lggr.Info("Got signal, stopping consumer")
		cancel()
	case err := <-errCh:
		lggr.With(zap.Error(err)).Fatal("Got error in consumer")
		cancel()
		os.Exit(1)
	}

	lggr.Info("Exiting")

}
func newApiServer(addr string, handlers api.HandlersPort) *http.Server {
	mux := api.NewRouter(handlers).Build()
	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       5 * time.Second,
	}
}

func newPublisher(lggr *zap.SugaredLogger) *streaming.Publisher {
	rabbitMQConfig := streaming.NewRabbitMQConfig("localhost", 5672, "guest", "guest", "", "topic")

	conn, err := streaming.NewRabbitMQConn(rabbitMQConfig, context.Background())

	if err != nil {
		lggr.With(zap.Error(err)).Fatal("Got error connecting with rabbit mq")
	}

	return streaming.NewPublisher(lggr, rabbitMQConfig, conn, context.Background())
}

func newConsumer(lggr *zap.SugaredLogger, topic string, handler streaming.Handler) (*streaming.Consumer, error) {
	rabbitMQConfig := streaming.NewRabbitMQConfig("localhost", 5672, "guest", "guest", topic, "topic")

	conn, err := streaming.NewRabbitMQConn(rabbitMQConfig, context.Background())

	if err != nil {
		lggr.With(zap.Error(err)).Fatal("Got error connecting with rabbit mq")
	}
	return streaming.NewConsumer(lggr, rabbitMQConfig, conn, handler, context.Background(), topic)
}
