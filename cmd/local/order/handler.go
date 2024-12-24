package main

import (
	"context"
	"encoding/json"

	"orchestration/cmd/local/order/application"
	"orchestration/internal/streaming"
	"orchestration/pkg/events"

	"go.uber.org/zap"
)

type OrderMessageHandler struct {
	logger      *zap.SugaredLogger
	publisher   streaming.Publisher
	useCasesMap map[string]application.MessageHandler
}

var (
	_ streaming.Handler = (*OrderMessageHandler)(nil)
)

func NewOrderMessageHandler(logger *zap.SugaredLogger, publisher streaming.Publisher, useCasesMap map[string]application.MessageHandler) *OrderMessageHandler {
	return &OrderMessageHandler{
		logger:      logger,
		publisher:   publisher,
		useCasesMap: useCasesMap,
	}
}

func (h *OrderMessageHandler) Handle(ctx context.Context, msg []byte, commitFn func() error) error {
	l := h.logger
	l.Infof("Order service received message [%s]", string(msg))
	// TODO: add idempotence check

	var message events.Event
	if err := json.Unmarshal(msg, &message); err != nil {
		h.logger.With("error", err).Error("Got error unmarshalling message")
		return err
	}
	l.Infof("Successfully unmarshalled message")

	useCase, ok := h.useCasesMap[message.Type]
	if !ok {
		l.Infof("Ignoring message")
		err := commitFn()
		if err != nil {
			l.With(zap.Error(err)).Error("Got error committing message")
			return err
		}
		return nil
	}

	replyMessage, err := useCase.Handle(ctx, &message)
	if err != nil {
		l.With(zap.Error(err)).Error("Got error handling message")
		return err
	}
	if replyMessage == nil {
		l.Infof("Nil reply message received. Should send message for retry by any means")
		err = commitFn()
		if err != nil {
			l.With(zap.Error(err)).Error("Got error committing message")
			return err
		}
		return nil
	}

	data, err := json.Marshal(replyMessage)
	if err != nil {
		l.With(zap.Error(err)).Error("Got error marshalling reply message")
		return err
	}

	l.Infof("Successfully marshalled reply message")
	err = h.publisher.Publish(ctx, "service.orders.events", data) // TODO: add outbox pattern
	if err != nil {
		l.With(zap.Error(err)).Error("Got error publishing message")
		return err
	}

	err = commitFn()
	if err != nil {
		l.With(zap.Error(err)).Error("Got error committing message")
		return err
	}

	return nil
}
