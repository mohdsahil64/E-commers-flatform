// Package audit records important administrative actions to an immutable log
// and exposes them for admin review.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is an audit log record.
type Entry struct {
	ID         string    `json:"id"`
	ActorID    *string   `json:"actor_id"`
	ActorRole  string    `json:"actor_role"`
	Action     string    `json:"action"`
	EntityType string    `json:"entity_type"`
	EntityID   string    `json:"entity_id"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
}

// Logger writes audit entries.
type Logger struct{ pool *pgxpool.Pool }

// New builds an audit Logger.
func New(pool *pgxpool.Pool) *Logger { return &Logger{pool: pool} }

// Action is a convenience wrapper for the common case of recording an actor
// performing an action on an entity, without before/after snapshots or request
// metadata. Used by service-layer hooks that don't have the fiber context.
func (l *Logger) Action(ctx context.Context, actorID, actorRole, action, entityType, entityID string) {
	l.Record(ctx, actorID, actorRole, action, entityType, entityID, nil, nil, "", "")
}

// Record writes an audit entry. Errors are swallowed intentionally so auditing
// never blocks the primary operation, but they are best-effort persisted.
func (l *Logger) Record(ctx context.Context, actorID, actorRole, action, entityType, entityID string, prev, next any, ip, ua string) {
	var prevJSON, nextJSON []byte
	if prev != nil {
		prevJSON, _ = json.Marshal(prev)
	}
	if next != nil {
		nextJSON, _ = json.Marshal(next)
	}
	var actor *string
	if actorID != "" {
		actor = &actorID
	}
	_, _ = l.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, entity_type, entity_id,
			previous_value, new_value, ip, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		actor, actorRole, action, entityType, entityID, nullableJSON(prevJSON), nullableJSON(nextJSON), ip, ua)
}

// List returns recent audit entries (admin).
func (l *Logger) List(ctx context.Context, limit, offset int) ([]Entry, error) {
	rows, err := l.pool.Query(ctx, `
		SELECT id, actor_id, actor_role, action, entity_type, entity_id, ip, created_at
		FROM audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.ActorID, &e.ActorRole, &e.Action, &e.EntityType,
			&e.EntityID, &e.IP, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
