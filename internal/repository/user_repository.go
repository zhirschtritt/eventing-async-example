package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zhirschtritt/eventing/internal/domain"
)

type DBUserRepository struct {
	db *pgxpool.Pool
}

func NewDBUserRepository(db *pgxpool.Pool) *DBUserRepository {
	return &DBUserRepository{
		db: db,
	}
}

func (r *DBUserRepository) Create(user *domain.User) error {
	query := `
		INSERT INTO users (id, email, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := r.db.Exec(context.Background(), query,
		user.ID, user.Email, user.Name, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *DBUserRepository) GetByID(id string) (*domain.User, error) {
	query := `
		SELECT id, email, name, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user domain.User
	err := r.db.QueryRow(context.Background(), query, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

func (r *DBUserRepository) GetByEmail(email string) (*domain.User, error) {
	query := `
		SELECT id, email, name, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user domain.User
	err := r.db.QueryRow(context.Background(), query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}
