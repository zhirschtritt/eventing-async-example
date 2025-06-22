package events

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/destel/rill"
)

type EventConsumer interface {
	Consume(ctx context.Context, event Event) error
	Start(ctx context.Context)
	Stop()
}

type EventRepository interface {
	Insert(event Event) error
	BulkInsert(events []Event) error
}

type ConsumerOptions struct {
	BufferSize   int
	BatchSize    int
	BatchTimeout time.Duration
}

type Consumer struct {
	eventRepository EventRepository
	eventCh         chan Event
	opts            ConsumerOptions
}

func (o *ConsumerOptions) defaults() {
	if o.BufferSize == 0 {
		o.BufferSize = 1000
	}
	if o.BatchSize == 0 {
		o.BatchSize = 100
	}
	if o.BatchTimeout == 0 {
		o.BatchTimeout = 100 * time.Millisecond
	}
}

func NewConsumer(eventRepository EventRepository, opts ConsumerOptions) *Consumer {
	opts.defaults()

	c := &Consumer{
		eventRepository: eventRepository,
		eventCh:         make(chan Event, opts.BufferSize),
		opts:            opts,
	}
	return c
}

func (c *Consumer) Start(ctx context.Context) {
	go func() {
		batches := rill.Batch(rill.FromChan(c.eventCh, nil), c.opts.BatchSize, c.opts.BatchTimeout)
		for batch := range batches {
			if len(batch.Value) > 0 {
				if err := c.eventRepository.BulkInsert(batch.Value); err != nil {
					log.Printf("error bulk inserting events: %v", err)
				}
			}
		}
	}()
}

func (c *Consumer) Stop() {
	close(c.eventCh)
}

var EventConsumerErrorFull = errors.New("event consumer is full")

func (c *Consumer) Consume(ctx context.Context, event Event) error {
	select {
	case c.eventCh <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return EventConsumerErrorFull
	}
}
