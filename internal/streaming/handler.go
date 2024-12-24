package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"orchestration/internal/saga"
	"orchestration/pkg/events"
	"time"

	"go.uber.org/zap"
)

type IdempotenceService interface {
	Has(ctx context.Context, key string) (bool, error)

	Set(ctx context.Context, key string, ttl time.Duration) error
}

type MessageHandler struct {
	logger *zap.SugaredLogger

	executionRepository saga.ExecutionRepository
	sagaService         saga.ServicePort
	idempotenceService  IdempotenceService
}

func NewMessageHandler(
	logger *zap.SugaredLogger,
	executionRepository saga.ExecutionRepository,
	workflowService saga.ServicePort,
	idempotenceService IdempotenceService,
) *MessageHandler {
	return &MessageHandler{
		logger:              logger,
		executionRepository: executionRepository,
		sagaService:         workflowService,
		idempotenceService:  idempotenceService,
	}
}

func (h *MessageHandler) Handle(ctx context.Context, msg []byte, commitFn func() error) error {
	l := h.logger
	l.Info("Handling message")
	var event events.Event

	err := json.Unmarshal(msg, &event)
	if err != nil {
		l.With(zap.Error(err)).Error("Got error unmarshalling message")
		return err
	}
	l.With("message", event).Info("Got message")
	msgHash, err := event.Hash()
	if err != nil {
		l.With(zap.Error(err)).Error("Got error creating message hash")
		return err
	}
	key := fmt.Sprintf("%s:%s:%s", event.ID, event.CorrelationID, msgHash)
	idempotent, err := h.idempotenceService.Has(ctx, key)
	if err != nil {
		l.With(zap.Error(err)).Error("Got error checking idempotence")
	}

	if idempotent {
		l.Info("Message was already processed")
		err = commitFn()
		if err != nil {
			l.With(zap.Error(err)).Error("Got error committing message")
			return err
		}
		return nil
	}

	// Get execution
	execution, err := h.executionRepository.Find(ctx, event.CorrelationID)
	if err != nil {
		l.With(zap.Error(err)).Error("Got error getting workflow")
		return err // TODO: handle error
	}

	if execution.IsEmpty() {
		l.Info("execution not found. Message will be ignored")
		err = commitFn()
		if err != nil {
			l.With(zap.Error(err)).Error("Got error committing message")
			return err
		}
		return nil
	}

	err = h.sagaService.ProcessMessage(ctx, &event, execution)
	if err != nil {
		l.With(zap.Error(err)).Error("Got error processing workflow message")
		return err
	}

	err = h.idempotenceService.Set(ctx, key, time.Hour*24*30)
	if err != nil {
		l.With(zap.Error(err)).Error("Got error setting idempotence")
	}
	err = commitFn()
	if err != nil {
		l.With(zap.Error(err)).Error("Got error committing message")
		return err
	}

	return nil
}
