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
	var user *User

	err := s.userRepo.DoInTx(ctx, func(ctx context.Context) error {

		existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
		if err == nil && existingUser != nil {
			return ErrUserAlreadyExists
		}

		now := time.Now()
		user = &User{
			ID:        uuid.New().String(),
			Email:     req.Email,
			Name:      req.Name,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}

		event := NewUserCreatedEvent(user)

		if err := s.eventConsumer.Consume(ctx, event.Event); err != nil {
			if errors.Is(err, events.EventConsumerErrorFull) {
				return ErrEventConsumerFull
			}

			return fmt.Errorf("user created but failed to publish event: %w", err)
		}

		return nil
	})

	return user, err
}
