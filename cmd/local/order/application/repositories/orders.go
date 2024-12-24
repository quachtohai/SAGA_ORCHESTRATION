package repositories

import (
	"context"

	"orchestration/cmd/local/order/domain/entities"
	"orchestration/cmd/local/order/presentation"

	"github.com/google/uuid"
)

type Orders interface {
	List(ctx context.Context) ([]presentation.Order, error) // TODO: add pagination filters
	Insert(ctx context.Context, order entities.Order) error
	Find(ctx context.Context, id uuid.UUID) (*entities.Order, error)
	FindByID(ctx context.Context, id uuid.UUID) (*presentation.OrderById, error)
	UpdateStatus(ctx context.Context, order *entities.Order) error
}
