// Package notifications stores in-app notifications and dispatches them through
// pluggable channels (email/SMS/etc via the notify provider). In-app records
// are always written; other channels are best-effort via the provider.
package notifications

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Notification is an in-app notification view model.
type Notification struct {
	ID        string         `json:"id"`
	EventType string         `json:"event_type"`
	Title     string         `json:"title"`
	Body      string         `json:"body"`
	Data      map[string]any `json:"data"`
	IsRead    bool           `json:"is_read"`
	CreatedAt time.Time      `json:"created_at"`
}

// Store persists notifications.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds a notification Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Create writes an in-app notification for a user.
func (s *Store) Create(ctx context.Context, userID, eventType, title, body string, data map[string]any) error {
	var payload []byte
	if data != nil {
		payload, _ = json.Marshal(data)
	} else {
		payload = []byte(`{}`)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notifications (user_id, channel, event_type, title, body, data, delivery_status)
		VALUES ($1,'in_app',$2,$3,$4,$5,'sent')`,
		userID, eventType, title, body, payload)
	return err
}

// ListForUser returns a user's notifications.
func (s *Store) ListForUser(ctx context.Context, userID string, limit, offset int) ([]Notification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, event_type, title, body, data, is_read, created_at
		FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.EventType, &n.Title, &n.Body, &n.Data, &n.IsRead, &n.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkRead marks a notification read.
func (s *Store) MarkRead(ctx context.Context, userID, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE notifications SET is_read = TRUE, read_at = now() WHERE id = $1 AND user_id = $2`,
		id, userID)
	return err
}

// UnreadCount returns the number of unread notifications.
func (s *Store) UnreadCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND is_read = FALSE`, userID).Scan(&n)
	return n, err
}
