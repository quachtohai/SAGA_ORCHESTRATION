package createorder

import (
	"context"

	"orchestration/internal/saga"
	"orchestration/pkg/events"
	"orchestration/pkg/structs"

	"go.uber.org/zap"
)

type CreateOrderStepPayloadBuilder struct {
	logger *zap.SugaredLogger
}

func NewCreateOrderStepPayloadBuilder(logger *zap.SugaredLogger) *CreateOrderStepPayloadBuilder {
	return &CreateOrderStepPayloadBuilder{logger: logger}
}

func (b *CreateOrderStepPayloadBuilder) Build(ctx context.Context, exec *saga.Execution, action saga.ActionType) (map[string]interface{}, error) {
	lggr := b.logger
	if action.IsRequest() {
		return b.buildRequestPayload(ctx, exec)
	}
	if action.IsCompensationRequest() {
		return b.buildCompensationRequestPayload(ctx, exec)
	}
	lggr.Infof("No payload to build for action: %s", action.String())
	return nil, nil
}

func (b *CreateOrderStepPayloadBuilder) buildRequestPayload(_ context.Context, exec *saga.Execution) (map[string]interface{}, error) {
	lggr := b.logger
	lggr.Info("Building request payload for create order step")

	var input Input
	err := exec.Read("input", &input)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error decoding input data from execution in create order step builder")
		return nil, err
	}

	items := make([]CreateOrderRequestItemPayload, len(input.Items))
	for idx, item := range input.Items {
		items[idx] = CreateOrderRequestItemPayload(item)
	}

	payload := CreateOrderRequestPayload{
		CustomerID:   input.CustomerID,
		Amount:       input.Amount,
		CurrencyCode: input.CurrencyCode,
		Items:        items,
	}

	lggr.Infof("Built request payload: %+v", payload)
	payloadMap, err := structs.ToMap(payload)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error converting payload to map")
		return nil, err
	}

	evt := events.NewEvent("create_order", "orchestrator", payloadMap).WithCorrelationID(exec.ID.String())
	eventMap, err := structs.ToMap(evt)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error converting event to map")
		return nil, err
	}

	return eventMap, nil
}

func (b *CreateOrderStepPayloadBuilder) buildCompensationRequestPayload(_ context.Context, exec *saga.Execution) (map[string]interface{}, error) {
	lggr := b.logger
	lggr.Info("Building request payload to compensate create order step")
	evt := events.NewEvent("reject_order", "orchestrator", map[string]interface{}{}).WithCorrelationID(exec.ID.String())
	eventMap, err := structs.ToMap(evt)
	if err != nil {
		lggr.With(zap.Error(err)).Error("Got error converting event to map")
		return nil, err
	}
	return eventMap, nil
}
