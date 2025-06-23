package events

import "time"

type Event struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	AggregateType string                 `json:"aggregate_type"`
	AggregateID   string                 `json:"aggregate_id"`
	Version       int64                  `json:"version"`
	Timestamp     time.Time              `json:"timestamp"`
	Data          map[string]interface{} `json:"data"`
}
