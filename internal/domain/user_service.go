package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zhirschtritt/eventing/internal/events"
)

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrEventConsumerFull = errors.New("event consumer is full")

type UserService struct {
	userRepo      UserRepository
	eventConsumer events.EventConsumer
}

func NewUserService(userRepo UserRepository, eventConsumer events.EventConsumer) *UserService {
	return &UserService{
		userRepo:      userRepo,
		eventConsumer: eventConsumer,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	existingUser, err := s.userRepo.GetByEmail(req.Email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	now := time.Now()
	user := &User{
		ID:        uuid.New().String(),
		Email:     req.Email,
		Name:      req.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	event := &UserCreatedEvent{
		eventID:     uuid.New().String(),
		aggregateID: user.ID,
		timestamp:   now,
		userID:      user.ID,
		email:       user.Email,
		name:        user.Name,
	}

	if err := s.eventConsumer.Consume(ctx, event); err != nil {
		if errors.Is(err, events.EventConsumerErrorFull) {
			return nil, ErrEventConsumerFull
		}

		return nil, fmt.Errorf("user created but failed to publish event: %w", err)
	}

	return user, nil
}
