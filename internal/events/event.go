package events

import "time"

type Event interface {
	ID() string
	Type() string
	AggregateType() string
	AggregateID() string
	Version() int64
	Timestamp() time.Time
	Data() map[string]interface{}
}
