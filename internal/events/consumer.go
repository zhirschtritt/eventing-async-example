package events

import "context"

type EventConsumer interface {
	Consume(ctx context.Context, event Event) error
	Start(ctx context.Context)
	Stop()
}
