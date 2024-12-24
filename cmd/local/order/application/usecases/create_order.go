package usecases

import (
	"context"

	"orchestration/cmd/local/order/application/repositories"
	"orchestration/cmd/local/order/domain/entities"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CreateOrderRequest struct {
	GlobalID     uuid.UUID
	CustomerID   uuid.UUID
	Amount       int64
	CurrencyCode string
}

type CreateOrderResponse struct {
	ID uuid.UUID
}

type CreateOrderUseCasePort interface {
	Execute(ctx context.Context, request CreateOrderRequest) (CreateOrderResponse, error)
}

type CreateOrder struct {
	logger *zap.SugaredLogger
	repo   repositories.Orders
}

func NewCreateOrder(logger *zap.SugaredLogger, repo repositories.Orders) CreateOrder {
	return CreateOrder{logger: logger, repo: repo}
}

func (co CreateOrder) Execute(ctx context.Context, request CreateOrderRequest) (CreateOrderResponse, error) {
	lggr := co.logger
	lggr.Info("Creating order")
	order := entities.NewOrder(
		request.CustomerID,
		request.GlobalID,
		request.Amount,
		request.CurrencyCode,
	)
	err := co.repo.Insert(ctx, order)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error creating order")
		return CreateOrderResponse{}, err
	}
	return CreateOrderResponse{ID: order.ID}, nil
}
