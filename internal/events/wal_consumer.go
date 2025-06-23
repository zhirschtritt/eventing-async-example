package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/destel/rill"
	"github.com/vadiminshakov/gowal"
)

var _ EventConsumer = new(WALConsumer)

type WALConsumer struct {
	eventRepository    EventRepository
	wal                *gowal.Wal
	workCh             chan WorkItem
	doneCh             chan uint64
	opts               WALConsumerOptions
	stateFile          string
	lastProcessedIndex atomic.Uint64
	coordinatorDone    chan struct{}
	workerDone         chan struct{}
	// In-memory state tracking for performance
	lastFlushedIndex atomic.Uint64
	flushThreshold   int
	flushTicker      *time.Ticker
	// Mutex to synchronize WAL access
	walMutex sync.Mutex
}

type WALConsumerOptions struct {
	BufferSize       int
	BatchSize        int
	BatchTimeout     time.Duration
	WALDir           string
	WALPrefix        string
	SegmentThreshold int
	MaxSegments      int
	IsInSyncDiskMode bool
	WorkerCount      int
	FlushThreshold   int
}

type WorkItem struct {
	Index uint64
	Event Event
}

func (o *WALConsumerOptions) defaults() {
	if o.BufferSize == 0 {
		o.BufferSize = 1000
	}
	if o.BatchSize == 0 {
		o.BatchSize = 100
	}
	if o.BatchTimeout == 0 {
		o.BatchTimeout = 100 * time.Millisecond
	}
	if o.WALDir == "" {
		o.WALDir = "./wal"
	}
	if o.WALPrefix == "" {
		o.WALPrefix = "event_"
	}
	if o.SegmentThreshold == 0 {
		o.SegmentThreshold = 1000
	}
	if o.MaxSegments == 0 {
		o.MaxSegments = 10
	}
	if o.WorkerCount == 0 {
		o.WorkerCount = 4
	}
	if o.FlushThreshold == 0 {
		o.FlushThreshold = 1000
	}
}

func NewWALConsumer(eventRepository EventRepository, opts WALConsumerOptions) (*WALConsumer, error) {
	opts.defaults()

	if err := os.MkdirAll(opts.WALDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	walConfig := gowal.Config{
		Dir:              opts.WALDir,
		Prefix:           opts.WALPrefix,
		SegmentThreshold: opts.SegmentThreshold,
		MaxSegments:      opts.MaxSegments,
		IsInSyncDiskMode: opts.IsInSyncDiskMode,
	}

	wal, err := gowal.NewWAL(walConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}

	stateFile := filepath.Join(opts.WALDir, "processor.state")

	c := &WALConsumer{
		eventRepository: eventRepository,
		wal:             wal,
		workCh:          make(chan WorkItem, opts.BufferSize),
		doneCh:          make(chan uint64, opts.BufferSize),
		opts:            opts,
		stateFile:       stateFile,
		coordinatorDone: make(chan struct{}),
		workerDone:      make(chan struct{}),
		flushThreshold:  opts.FlushThreshold,
	}

	return c, nil
}

func (c *WALConsumer) Start(ctx context.Context) {
	// Initialize last flushed index from disk
	lastFlushedIndex, err := c.readLastProcessedIndex()
	if err != nil {
		log.Printf("failed to read last processed index, starting from 0: %v", err)
		lastFlushedIndex = 0
	}
	c.lastFlushedIndex.Store(lastFlushedIndex)
	c.lastProcessedIndex.Store(lastFlushedIndex)

	// Start state coordinator
	go c.stateCoordinator(ctx)

	// Start workers
	for range c.opts.WorkerCount {
		go c.worker(ctx)
	}

	// Start recovery process
	if err := c.recover(ctx); err != nil {
		log.Printf("error during recovery: %v", err)
	}
}

func (c *WALConsumer) Stop() {
	close(c.workCh)
	close(c.doneCh)

	// Wait for workers to finish
	<-c.workerDone

	// Wait for coordinator to finish
	<-c.coordinatorDone

	// Final flush to disk
	if err := c.writeLastProcessedIndex(c.lastProcessedIndex.Load()); err != nil {
		log.Printf("error during final flush: %v", err)
	}

	// Synchronize WAL access when closing
	c.walMutex.Lock()
	if err := c.wal.Close(); err != nil {
		log.Printf("error closing WAL: %v", err)
	}
	c.walMutex.Unlock()
}

func (c *WALConsumer) Consume(ctx context.Context, event Event) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Synchronize WAL access to prevent concurrent writes
	c.walMutex.Lock()
	// Get the next index from WAL
	index := c.wal.CurrentIndex() + 1

	// Write to WAL first (this is the critical durability step)
	if err := c.wal.Write(index, event.ID, eventJSON); err != nil {
		c.walMutex.Unlock()
		return fmt.Errorf("failed to write to WAL: %w", err)
	}
	c.walMutex.Unlock()

	// Create work item with the WAL index
	workItem := WorkItem{
		Index: index,
		Event: event,
	}

	// Push to work channel
	select {
	case c.workCh <- workItem:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return EventConsumerErrorFull
	}
}

func (c *WALConsumer) recover(ctx context.Context) error {
	// Use the last flushed index from memory (already loaded in Start)
	lastProcessedIndex := c.lastProcessedIndex.Load()

	log.Printf("recovering from index %d", lastProcessedIndex)

	// Synchronize WAL access during recovery
	c.walMutex.Lock()
	// Iterate through WAL from the last processed index + 1
	for msg := range c.wal.Iterator() {
		if msg.Index() <= lastProcessedIndex {
			continue // skip already processed events
		}

		var event Event
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("failed to unmarshal event at index %d: %v", msg.Index(), err)
			continue
		}

		workItem := WorkItem{
			Index: msg.Index(),
			Event: event,
		}

		select {
		case c.workCh <- workItem:
			log.Printf("recovered event at index %d: %s", msg.Index(), event.Type)
		case <-ctx.Done():
			c.walMutex.Unlock()
			return ctx.Err()
		}
	}
	c.walMutex.Unlock()

	log.Printf("recovery completed")
	return nil
}

func (c *WALConsumer) stateCoordinator(ctx context.Context) {
	defer func() {
		select {
		case c.coordinatorDone <- struct{}{}:
		default:
		}
	}()

	// Track completed indexes that arrived out of order
	completedOutOfOrder := make(map[uint64]bool)

	// Set up periodic flush ticker (every 5 seconds as backup)
	flushTicker := time.NewTicker(5 * time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case completedIndex, ok := <-c.doneCh:
			if !ok {
				return
			}

			lastProcessedIndex := c.lastProcessedIndex.Load()

			// If this is the next expected index
			if completedIndex == lastProcessedIndex+1 {
				c.lastProcessedIndex.Add(1)

				log.Printf("state advanced to %d", c.lastProcessedIndex.Load())

				// Check if we can advance further with out-of-order completions
				for {
					lastProcessedIndex = c.lastProcessedIndex.Load()

					if completedOutOfOrder[lastProcessedIndex+1] {
						c.lastProcessedIndex.Add(1)

						log.Printf("state advanced (catch-up) to %d", c.lastProcessedIndex.Load())
						delete(completedOutOfOrder, c.lastProcessedIndex.Load())
					} else {
						break
					}
				}

				// Check if we should flush to disk
				currentIndex := c.lastProcessedIndex.Load()
				lastFlushed := c.lastFlushedIndex.Load()
				if currentIndex-lastFlushed >= uint64(c.flushThreshold) {
					if err := c.writeLastProcessedIndex(currentIndex); err != nil {
						log.Printf("failed to write last processed index: %v", err)
					} else {
						c.lastFlushedIndex.Store(currentIndex)
						log.Printf("flushed state to disk at index %d", currentIndex)
					}
				}

			} else if completedIndex > lastProcessedIndex+1 {
				// Out-of-order completion
				log.Printf("received out-of-order completion for index %d, waiting for %d",
					completedIndex, lastProcessedIndex+1)
				completedOutOfOrder[completedIndex] = true
			}
			// Ignore duplicates (completedIndex <= lastProcessedIndex)

		case <-flushTicker.C:
			// Periodic flush to disk as backup
			currentIndex := c.lastProcessedIndex.Load()
			lastFlushed := c.lastFlushedIndex.Load()
			if currentIndex > lastFlushed {
				if err := c.writeLastProcessedIndex(currentIndex); err != nil {
					log.Printf("failed to write last processed index during periodic flush: %v", err)
				} else {
					c.lastFlushedIndex.Store(currentIndex)
					log.Printf("periodic flush: state to disk at index %d", currentIndex)
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

func (c *WALConsumer) worker(ctx context.Context) {
	defer close(c.workerDone)

	batches := rill.Batch(rill.FromChan(c.workCh, nil), c.opts.BatchSize, c.opts.BatchTimeout)

	for batch := range batches {
		if len(batch.Value) > 0 {
			events := make([]Event, len(batch.Value))
			for i, workItem := range batch.Value {
				events[i] = workItem.Event
			}

			if err := c.eventRepository.BulkInsert(ctx, events); err != nil {
				log.Printf("error bulk inserting events: %v", err)
			}
			for _, workItem := range batch.Value {
				c.doneCh <- workItem.Index
			}
		}
	}
}

func (c *WALConsumer) readLastProcessedIndex() (uint64, error) {
	data, err := os.ReadFile(c.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var index uint64
	if _, err := fmt.Sscanf(string(data), "%d", &index); err != nil {
		return 0, err
	}

	return index, nil
}

func (c *WALConsumer) writeLastProcessedIndex(index uint64) error {
	data := fmt.Sprintf("%d", index)
	return os.WriteFile(c.stateFile, []byte(data), 0644)
}
