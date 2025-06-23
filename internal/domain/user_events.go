package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/zhirschtritt/eventing/internal/events"
)

type UserCreatedEventData struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type UserCreatedEvent struct {
	events.Event
	Data UserCreatedEventData `json:"data"`
}

func NewUserCreatedEvent(user *User) *UserCreatedEvent {
	return &UserCreatedEvent{
		Event: events.Event{
			ID:            uuid.New().String(),
			Type:          "user.created",
			AggregateType: "user",
			AggregateID:   user.ID,
			Version:       1,
			Timestamp:     time.Now(),
		},
		Data: UserCreatedEventData{
			Email: user.Email,
			Name:  user.Name,
		},
	}
}
