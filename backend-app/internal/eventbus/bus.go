package eventbus

import (
	"context"
	"groupeak/internal/models"
	"log/slog"
	"sync"
)

// Транспортная структура
type SystemEvent struct {
	Type      models.EventType
	ProjectID int64
	ActorID   int64
	EntityID  int64
	Payload   map[string]interface{}
}

// Ядро системы асинхронных событий
type EventBus struct {
	ch chan SystemEvent
	wg sync.WaitGroup

	logger *slog.Logger

	handler func(ctx context.Context, event SystemEvent)
}

func NewEventBus(logger *slog.Logger) *EventBus {
	return &EventBus{
		ch:     make(chan SystemEvent, 5000),
		logger: logger,
	}
}

func (b *EventBus) SetHandler(h func(ctx context.Context, event SystemEvent)) {
	b.handler = h
}

// Pub
func (b *EventBus) Publish(event SystemEvent) {
	select {
	case b.ch <- event:
	default:
		b.logger.Warn("event bus full, dropped event", slog.Any("type", event.Type))
	}
}

func (b *EventBus) Start(ctx context.Context) {
	if b.handler == nil {
		b.logger.Error("event bus started without handler")
		return
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				b.logger.Info("shutting down event bus worker")
				return
			case event := <-b.ch:
				b.handler(context.Background(), event)
			}
		}
	}()
}

func (b *EventBus) Stop() {
	close(b.ch)
	b.wg.Wait()
}
