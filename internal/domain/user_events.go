package domain

import "time"

type UserCreatedEvent struct {
	eventID     string
	aggregateID string
	timestamp   time.Time
	userID      string
	email       string
	name        string
}

func (e *UserCreatedEvent) ID() string {
	return e.eventID
}

func (e *UserCreatedEvent) Type() string {
	return "user.created"
}

func (e *UserCreatedEvent) AggregateType() string {
	return "user"
}

func (e *UserCreatedEvent) AggregateID() string {
	return e.aggregateID
}

func (e *UserCreatedEvent) Version() int64 {
	return 1
}

func (e *UserCreatedEvent) Timestamp() time.Time {
	return e.timestamp
}

func (e *UserCreatedEvent) Data() map[string]interface{} {
	return map[string]interface{}{
		"user_id": e.userID,
		"email":   e.email,
		"name":    e.name,
	}
}
