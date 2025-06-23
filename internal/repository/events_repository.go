package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zhirschtritt/eventing/internal/events"
)

type DBEventsRepository struct {
	db *pgxpool.Pool
}

func NewDBEventsRepository(db *pgxpool.Pool) *DBEventsRepository {
	return &DBEventsRepository{
		db: db,
	}
}

func (r *DBEventsRepository) Insert(ctx context.Context, event events.Event) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	query := `
		INSERT INTO events (type, aggregate_type, aggregate_id, version, timestamp, data)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err = r.db.Exec(ctx, query,
		event.Type, event.AggregateType, event.AggregateID, event.Version, event.Timestamp, data)

	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

func (r *DBEventsRepository) BulkInsert(ctx context.Context, events []events.Event) error {
	if len(events) == 0 {
		return nil
	}

	query := `
		INSERT INTO events (type, aggregate_type, aggregate_id, version, timestamp, data)
		VALUES 
	`

	values := make([]string, len(events))
	args := make([]interface{}, 0, len(events)*6)
	argIndex := 1

	for i, event := range events {
		data, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("failed to marshal event data: %w", err)
		}

		values[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
			argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5)

		args = append(args, event.Type, event.AggregateType, event.AggregateID, event.Version, event.Timestamp, data)
		argIndex += 6
	}

	query += strings.Join(values, ", ")

	_, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to bulk insert events: %w", err)
	}

	return nil
}
