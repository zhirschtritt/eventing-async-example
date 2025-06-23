package domain

import (
	"context"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required"`
}

type UserRepository interface {
	DoInTx(ctx context.Context, fn func(ctx context.Context) error) error
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
}
