package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

type txCtxKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

func GetTx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txCtxKey{}).(pgx.Tx)
	return tx, ok
}

func (r *DBUserRepository) DoInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(WithTx(ctx, tx)); err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return tx.Commit(ctx)
}

type executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (r *DBUserRepository) Create(ctx context.Context, user *domain.User) error {
	var executor executor = r.db

	tx, ok := GetTx(ctx)
	if ok {
		executor = tx
	}

	query := `
		INSERT INTO users (id, email, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := executor.Exec(context.Background(), query,
		user.ID, user.Email, user.Name, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *DBUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var executor queryer = r.db
	tx, ok := GetTx(ctx)
	if ok {
		executor = tx
	}

	query := `
		SELECT id, email, name, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user domain.User
	err := executor.QueryRow(context.Background(), query, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}

	return &user, nil
}

func (r *DBUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var executor queryer = r.db
	tx, ok := GetTx(ctx)
	if ok {
		executor = tx
	}

	query := `
		SELECT id, email, name, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	var user domain.User
	err := executor.QueryRow(context.Background(), query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &user, nil
}
