package order

import (
	"context"
	"fmt"

	"orchestration/cmd/local/order/adapters/repositores/order/generated"
	"orchestration/cmd/local/order/application/repositories"
	"orchestration/cmd/local/order/domain/entities"
	"orchestration/cmd/local/order/presentation"
	"orchestration/pkg/utc"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type RepositoryAdapter struct {
	lggr *zap.SugaredLogger
	pool *pgxpool.Pool
}

func NewRepositoryAdapter(lggr *zap.SugaredLogger, pool *pgxpool.Pool) *RepositoryAdapter {
	return &RepositoryAdapter{lggr: lggr, pool: pool}
}

var (
	_ repositories.Orders = (*RepositoryAdapter)(nil)
)

func (r *RepositoryAdapter) List(ctx context.Context) ([]presentation.Order, error) {
	lggr := r.lggr
	lggr.Info("RepositoryAdapter.List")
	db, err := r.pool.Acquire(ctx)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error acquiring connection")
		return nil, err
	}
	defer db.Release()
	queries := generated.New(db)
	rows, err := queries.ListOrders(ctx)
	if err != nil {
		if err == pgx.ErrNoRows {
			return []presentation.Order{}, nil
		}
		lggr.With(zap.Error(err)).Error("Got error querying database")
		return nil, err
	}

	ordersPresentation := make([]presentation.Order, len(rows))
	for i, row := range rows {
		ordersPresentation[i] = presentation.Order{
			ID:           row.Uuid.String(),
			CustomerID:   row.CustomerID.String(),
			Amount:       row.Amount,
			CurrencyCode: row.CurrencyCode,
			Status:       row.Status,
			CreatedAt:    utc.NewFromTime(row.CreatedAt.Time),
			UpdatedAt:    utc.NewFromTime(row.UpdatedAt.Time),
		}
	}
	return ordersPresentation, nil
}

func (r *RepositoryAdapter) Insert(ctx context.Context, order entities.Order) error {
	lggr := r.lggr
	lggr.Info("RepositoryAdapter.Insert")

	db, err := r.pool.Acquire(ctx)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error acquiring connection")
		return err
	}
	defer db.Release()
	queries := generated.New(db)
	idString := order.ID.String()
	fmt.Println("idString", idString)
	err = queries.InsertOrder(ctx, generated.InsertOrderParams{
		Uuid:         order.ID,
		CustomerID:   order.CustomerID,
		Status:       order.Status.String(),
		Amount:       order.Amount,
		CurrencyCode: order.CurrencyCode,
		CreatedAt: pgtype.Timestamptz{
			Time:  order.CreatedAt.Time(),
			Valid: true,
		},
		UpdatedAt: pgtype.Timestamptz{
			Time:  order.UpdatedAt.Time(),
			Valid: true,
		},
	})
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error inserting order")
		return err
	}
	return nil
}

func (r *RepositoryAdapter) FindByID(ctx context.Context, id uuid.UUID) (*presentation.OrderById, error) {
	lggr := r.lggr
	lggr.Info("RepositoryAdapter.FindByID")
	db, err := r.pool.Acquire(ctx)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error acquiring connection")
		return nil, err
	}
	defer db.Release()
	queries := generated.New(db)
	order, err := queries.GetOrder(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &presentation.OrderById{}, nil
		}
		lggr.With(zap.Error(err)).Error("Got error querying get order by id")
		return nil, err
	}
	orderPresentation := presentation.OrderById{
		ID:           order.Uuid.String(),
		CustomerID:   order.CustomerID.String(),
		Amount:       order.Amount,
		CurrencyCode: order.CurrencyCode,
		Status:       order.Status,
		CreatedAt:    utc.NewFromTime(order.CreatedAt.Time),
		UpdatedAt:    utc.NewFromTime(order.UpdatedAt.Time),
	}

	return &orderPresentation, nil
}

func (r *RepositoryAdapter) Find(ctx context.Context, id uuid.UUID) (*entities.Order, error) {
	lggr := r.lggr
	lggr.Info("RepositoryAdapter.Find")
	db, err := r.pool.Acquire(ctx)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error acquiring connection")
		return nil, err
	}
	defer db.Release()
	queries := generated.New(db)
	order, err := queries.GetOrder(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return &entities.Order{}, nil
		}
		lggr.With(zap.Error(err)).Error("Got error querying get order by id")
		return nil, err
	}
	return &entities.Order{
		ID:           order.Uuid,
		CustomerID:   order.CustomerID,
		Amount:       order.Amount,
		CurrencyCode: order.CurrencyCode,
		Status:       entities.OrderStatus(order.Status),
		CreatedAt:    utc.NewFromTime(order.CreatedAt.Time),
		UpdatedAt:    utc.NewFromTime(order.UpdatedAt.Time),
	}, nil
}

func (r *RepositoryAdapter) UpdateStatus(ctx context.Context, order *entities.Order) error {
	lggr := r.lggr
	lggr.Info("RepositoryAdapter.UpdateStatus")
	db, err := r.pool.Acquire(ctx)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error acquiring connection")
		return err
	}
	defer db.Release()
	queries := generated.New(db)

	err = queries.UpdateOrder(ctx, generated.UpdateOrderParams{
		Uuid:   order.ID,
		Status: order.Status.String(),
		UpdatedAt: pgtype.Timestamptz{
			Time:  order.UpdatedAt.Time(),
			Valid: true,
		},
	})
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error updating order status")
		return err
	}

	lggr.Infof("Updated order status to [%s]", order.ID)
	return nil
}
