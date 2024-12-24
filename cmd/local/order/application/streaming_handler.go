package application

import (
	"context"

	"orchestration/pkg/events"
)

type MessageHandler interface {
	Handle(ctx context.Context, msg *events.Event) (*events.Event, error)
}
