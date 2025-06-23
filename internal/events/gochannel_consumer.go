package events

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/destel/rill"
)

type EventRepository interface {
	Insert(ctx context.Context, event Event) error
	BulkInsert(ctx context.Context, events []Event) error
}

type ConsumerOptions struct {
	BufferSize   int
	BatchSize    int
	BatchTimeout time.Duration
	WorkerCount  int
}

// typecheck
var _ EventConsumer = new(GoChannelConsumer)

type GoChannelConsumer struct {
	eventRepository EventRepository
	workCh          chan Event
	opts            ConsumerOptions
	workerWg        sync.WaitGroup
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
	if o.WorkerCount == 0 {
		o.WorkerCount = 4
	}
}

func NewConsumer(eventRepository EventRepository, opts ConsumerOptions) *GoChannelConsumer {
	opts.defaults()

	c := &GoChannelConsumer{
		eventRepository: eventRepository,
		workCh:          make(chan Event, opts.BufferSize),
		opts:            opts,
	}
	return c
}

func (c *GoChannelConsumer) Start(ctx context.Context) {
	// Start workers
	for range c.opts.WorkerCount {
		c.workerWg.Add(1)
		go c.worker(ctx)
	}
}

func (c *GoChannelConsumer) Stop() {
	close(c.workCh)

	// Wait for all workers to finish
	c.workerWg.Wait()
}

func (c *GoChannelConsumer) worker(ctx context.Context) {
	defer c.workerWg.Done()

	batches := rill.Batch(rill.FromChan(c.workCh, nil), c.opts.BatchSize, c.opts.BatchTimeout)
	for batch := range batches {
		if len(batch.Value) > 0 {
			if err := c.eventRepository.BulkInsert(ctx, batch.Value); err != nil {
				// TODO: retry logic, dead-letter queue, etc.
				log.Printf("error bulk inserting events: %v", err)
			}
		}
	}
}

var EventConsumerErrorFull = errors.New("event consumer is full")

func (c *GoChannelConsumer) Consume(ctx context.Context, event Event) error {
	select {
	case c.workCh <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return EventConsumerErrorFull
	}
}
