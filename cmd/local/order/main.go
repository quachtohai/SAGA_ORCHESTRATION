package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"orchestration/cmd/local/order/adapters/repositores/order" // TODO: fix typo
	"orchestration/cmd/local/order/api"
	"orchestration/cmd/local/order/application"
	"orchestration/cmd/local/order/application/usecases"
	"orchestration/cmd/local/order/config/env"
	"orchestration/cmd/local/order/handlers"
	"orchestration/internal/config/logger"
	"orchestration/internal/streaming"

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

	lggr.Info("Starting Order Service")
	rabbitMQConfig := streaming.NewRabbitMQConfig("localhost", 5672, "guest", "guest", cfg.ServiceName, "topic")
	conn, err := streaming.NewRabbitMQConn(rabbitMQConfig, context.Background())

	if err != nil {
		lggr.With(zap.Error(err)).Fatal("Got error connecting with rabbit mq")
	}
	dbpool, err := pgxpool.New(context.Background(), cfg.DBConnectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer dbpool.Close()
	lggr.Info("Connected to database")

	redisConn := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	err = redisConn.Ping(ctx).Err()
	if err != nil {
		lggr.With(zap.Error(err)).Fatal("Got error connecting to Redis")
	}

	publisher := streaming.NewPublisher(lggr, rabbitMQConfig, conn, context.Background())

	var (
		ordersRepository = order.NewRepositoryAdapter(lggr, dbpool)
		usecasesMap      = map[string]application.MessageHandler{
			"create_order":  handlers.NewCreateOrderHandler(lggr, usecases.NewCreateOrder(lggr, ordersRepository)),
			"approve_order": handlers.NewApproveOrder(lggr, usecases.NewApproveOrder(lggr, ordersRepository)),
			"reject_order":  handlers.NewRejectOrder(lggr, usecases.NewRejectOrder(lggr, ordersRepository)),
		}
	)

	handler := NewOrderMessageHandler(lggr, *publisher, usecasesMap)

	consumer, err := streaming.NewConsumer(lggr, rabbitMQConfig, conn, handler, context.Background(), "service.orders.request")
	if err != nil {
		lggr.With(zap.Error(err)).Fatal("Got error creating consumer")
	}

	go func() {
		if err := consumer.Start(ctx); err != nil {
			lggr.With(zap.Error(err)).Fatal("Got in consumer")
			errCh <- err
		}
	}()

	var (
		listUseCase  = usecases.NewListOrders(lggr, ordersRepository)
		getOrderByID = usecases.NewGetOrderByID(lggr, ordersRepository)
		apiHandlers  = api.NewHandlers(lggr, listUseCase, getOrderByID)
		httpServer   = newApiServer(":3001", apiHandlers)
	)

	go func() {
		lggr.Info("Starting Orders Service API go routine")
		if err := httpServer.ListenAndServe(); err != nil {
			lggr.With(zap.Error(err)).Fatal("Got error in Orders Service API server")
			errCh <- err
		}
	}()

	lggr.Info("Running Order Service. Waiting for signal to stop...")
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

func newApiServer(addr string, handlers api.OrderHandlers) *http.Server {
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
