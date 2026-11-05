package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/carlitos/backend/internal/httpx"
)

// ── timeline events ────────────────────────────────────────────────────────

// execer is satisfied by both *pgxpool.Pool and pgx.Tx.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// addEvent appends a tracking-timeline entry stamped "now".
func (s *Service) addEvent(ctx context.Context, q execer, shipmentID, status, location, description string) {
	s.addEventAt(ctx, q, shipmentID, status, location, description, time.Now())
}

// addEventAt appends a timeline entry at a given time. An identical entry
// (same status / place / text) is never stored twice, so courier retries,
// repeated webhooks and repeated "refresh" clicks cannot fill the timeline with
// duplicates. Best-effort: a timeline write must never fail the real action.
func (s *Service) addEventAt(ctx context.Context, q execer, shipmentID, status, location, description string, at time.Time) {
	if at.IsZero() {
		at = time.Now()
	}
	_, _ = q.Exec(ctx, `
		INSERT INTO shipment_tracking_events (shipment_id, status, location, description, event_time)
		SELECT $1::uuid, $2::text, $3::text, $4::text, $5::timestamptz
		WHERE NOT EXISTS (
			SELECT 1 FROM shipment_tracking_events
			WHERE shipment_id = $1::uuid AND status = $2::text AND location = $3::text AND description = $4::text
		)`, shipmentID, status, location, description, at)
}

// ── "needs attention" ──────────────────────────────────────────────────────

// attentionSQL is the SQL twin of attentionReason: a shipment is flagged when it
// failed / is returning to the seller, has sat un-picked-up for too long, or has
// been in transit for too long. Keep the two in sync.
const attentionSQL = `(
	s.status IN ('failed','rto')
	OR (s.status IN ('created','label_generated','pickup_scheduled') AND s.ship_type = 'courier'
	    AND s.created_at < now() - interval '2 days')
	OR (s.status IN ('created','label_generated','pickup_scheduled') AND s.ship_type = 'self'
	    AND s.created_at < now() - interval '3 days')
	OR (s.status IN ('picked_up','in_transit','out_for_delivery')
	    AND coalesce(s.shipped_at, s.created_at) < now() - interval '10 days')
)`

func daysSince(t, now time.Time) int { return int(now.Sub(t).Hours() / 24) }

// attentionReason explains why a shipment looks stuck, or "" when it is fine.
func attentionReason(status, shipType string, createdAt time.Time, shippedAt *time.Time, now time.Time) string {
	switch status {
	case "rto":
		return "Returning to the seller (RTO)"
	case "failed":
		return "Delivery failed"
	case "created", "label_generated", "pickup_scheduled":
		d := daysSince(createdAt, now)
		if shipType == "self" && d >= 3 {
			return fmt.Sprintf("Not dispatched for %d days", d)
		}
		if shipType != "self" && d >= 2 {
			return fmt.Sprintf("Waiting for courier pickup for %d days", d)
		}
	case "picked_up", "in_transit", "out_for_delivery":
		base := createdAt
		if shippedAt != nil {
			base = *shippedAt
		}
		if d := daysSince(base, now); d >= 10 {
			return fmt.Sprintf("In transit for %d days", d)
		}
	}
	return ""
}

// ── detail ─────────────────────────────────────────────────────────────────

// ShipmentLine is one product line inside a shipment.
type ShipmentLine struct {
	Title    string  `json:"title"`
	Variant  string  `json:"variant"`
	SKU      string  `json:"sku"`
	Quantity int     `json:"quantity"`
	Amount   float64 `json:"amount"`
}

// ShipmentDetail is a shipment plus everything needed to act on it.
type ShipmentDetail struct {
	Shipment
	ContactPhone string         `json:"contact_phone"`
	ShipLine1    string         `json:"ship_line1"`
	ShipLine2    string         `json:"ship_line2"`
	ShipCountry  string         `json:"ship_country"`
	OrderStatus  string         `json:"order_status"`
	Items        []ShipmentLine `json:"items"`
	Events       []TrackEvent   `json:"events"`
}

// Detail returns a shipment with its items and timeline. sellerID scopes the
// lookup to one seller; pass "" for the admin view.
func (s *Service) Detail(ctx context.Context, shipmentID, sellerID string) (*ShipmentDetail, error) {
	if _, err := uuid.Parse(shipmentID); err != nil {
		return nil, httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	row := s.repo.pool.QueryRow(ctx, shipmentSelect+
		` WHERE s.id = $1 AND ($2::text = '' OR s.seller_id::text = $2::text)`, shipmentID, sellerID)
	sh, err := scanShipment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	if err != nil {
		return nil, err
	}
	d := &ShipmentDetail{Shipment: sh, Items: []ShipmentLine{}, Events: []TrackEvent{}}

	_ = s.repo.pool.QueryRow(ctx, `
		SELECT ship_contact_phone, ship_line1, ship_line2, ship_country, status
		FROM orders WHERE id = $1`, sh.OrderID).
		Scan(&d.ContactPhone, &d.ShipLine1, &d.ShipLine2, &d.ShipCountry, &d.OrderStatus)

	rows, err := s.repo.pool.Query(ctx, `
		SELECT oi.product_title, coalesce(oi.variant_label,''), coalesce(oi.sku,''),
		       si.quantity, oi.gross_amount::float8
		FROM shipment_items si JOIN order_items oi ON oi.id = si.order_item_id
		WHERE si.shipment_id = $1 ORDER BY oi.created_at`, shipmentID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var l ShipmentLine
		if err := rows.Scan(&l.Title, &l.Variant, &l.SKU, &l.Quantity, &l.Amount); err != nil {
			rows.Close()
			return nil, err
		}
		d.Items = append(d.Items, l)
	}
	rows.Close()

	if ev, err := s.timeline(ctx, shipmentID); err == nil && ev != nil {
		d.Events = ev
	}
	return d, nil
}

// ── seller: orders that are ready to ship ──────────────────────────────────

// ReadyOrder is an order whose items are packed and still need a shipment.
type ReadyOrder struct {
	OrderID       string    `json:"order_id"`
	OrderNumber   string    `json:"order_number"`
	CustomerName  string    `json:"customer_name"`
	ShipCity      string    `json:"ship_city"`
	ShipState     string    `json:"ship_state"`
	ShipPincode   string    `json:"ship_pincode"`
	PaymentMethod string    `json:"payment_method"`
	PlacedAt      time.Time `json:"placed_at"`
	ItemCount     int       `json:"item_count"`
	ItemTitle     string    `json:"item_title"`
	Amount        float64   `json:"amount"`
	CODAmount     float64   `json:"cod_amount"`
	Status        string    `json:"status"` // packed | ready_to_ship
}

// ReadyToShip lists the seller's orders that are fully packed but have no
// (non-cancelled) shipment yet — i.e. what the seller should ship next.
func (s *Service) ReadyToShip(ctx context.Context, sellerID string) ([]ReadyOrder, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT o.id, o.order_number, o.ship_contact_name, o.ship_city, o.ship_state, o.ship_pincode,
		       o.payment_method, coalesce(o.placed_at, o.created_at),
		       sum(oi.quantity)::int, coalesce(min(oi.product_title),''), sum(oi.gross_amount)::float8,
		       CASE WHEN bool_and(oi.item_status = 'ready_to_ship') THEN 'ready_to_ship' ELSE 'packed' END
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id
		WHERE oi.seller_id = $1
		  AND o.status NOT IN ('cancelled','refunded','pending_payment')
		  AND NOT EXISTS (
		        SELECT 1 FROM shipments sh
		        WHERE sh.order_id = o.id AND sh.seller_id = $1
		          AND sh.direction = 'forward' AND sh.status <> 'cancelled')
		GROUP BY o.id
		HAVING count(*) FILTER (WHERE oi.item_status IN ('packed','ready_to_ship')) > 0
		   AND count(*) FILTER (WHERE oi.item_status IN ('confirmed','processing')) = 0
		ORDER BY coalesce(o.placed_at, o.created_at)`, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReadyOrder{}
	for rows.Next() {
		var r ReadyOrder
		if err := rows.Scan(&r.OrderID, &r.OrderNumber, &r.CustomerName, &r.ShipCity, &r.ShipState,
			&r.ShipPincode, &r.PaymentMethod, &r.PlacedAt, &r.ItemCount, &r.ItemTitle,
			&r.Amount, &r.Status); err != nil {
			return nil, err
		}
		if r.PaymentMethod == "cod" {
			r.CODAmount = r.Amount
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── seller: cancel a booked shipment ───────────────────────────────────────

// Cancel withdraws a shipment that has not left yet (not picked up). The order
// goes back to "ready to ship" so the seller can book it again, and the freight
// that had been charged to the seller for it is released.
func (s *Service) Cancel(ctx context.Context, shipmentID, sellerID string) error {
	var status, shipType, awb string
	err := s.repo.pool.QueryRow(ctx, `
		SELECT status, coalesce(ship_type,'courier'), coalesce(awb_number,'')
		FROM shipments WHERE id = $1 AND seller_id = $2`, shipmentID, sellerID).Scan(&status, &shipType, &awb)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	if err != nil {
		return err
	}
	switch status {
	case "created", "label_generated", "pickup_scheduled":
	default:
		return httpx.Conflict("invalid_state",
			"Only a shipment that hasn't been picked up yet can be cancelled.")
	}
	if shipType == "courier" && awb != "" {
		if err := s.courier.Cancel(ctx, awb); err != nil {
			return httpx.Conflict("courier_cancel_failed",
				"The courier couldn't cancel this shipment. If it has already been picked up it can no longer be cancelled.")
		}
	}
	err = pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE shipments SET status = 'cancelled'
			WHERE id = $1 AND seller_id = $2 AND status IN ('created','label_generated','pickup_scheduled')`,
			shipmentID, sellerID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.Conflict("invalid_state", "This shipment can no longer be cancelled.")
		}
		_, err = tx.Exec(ctx, `
			UPDATE order_items SET shipping_charge = 0
			WHERE id IN (SELECT order_item_id FROM shipment_items WHERE shipment_id = $1)`, shipmentID)
		return err
	})
	if err != nil {
		return err
	}
	s.addEvent(ctx, s.repo.pool, shipmentID, "cancelled", "", "Shipment cancelled by the seller")
	return nil
}

// ── refresh from the courier ───────────────────────────────────────────────

// Refresh pulls the latest tracking from the courier, stores new timeline
// entries and moves the shipment forward if the courier is ahead of us. sellerID
// scopes the shipment to one seller ("" = admin).
func (s *Service) Refresh(ctx context.Context, shipmentID, sellerID string) error {
	if _, err := uuid.Parse(shipmentID); err != nil {
		return httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	var orderID, shipType, awb, status, courierName string
	err := s.repo.pool.QueryRow(ctx, `
		SELECT order_id, coalesce(ship_type,'courier'), coalesce(awb_number,''), status, courier_name
		FROM shipments
		WHERE id = $1 AND ($2::text = '' OR seller_id::text = $2::text)`, shipmentID, sellerID).
		Scan(&orderID, &shipType, &awb, &status, &courierName)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	if err != nil {
		return err
	}
	if shipType == "self" || awb == "" {
		return httpx.BadRequest("not_trackable", "Self-shipped orders have no courier tracking to refresh.")
	}
	events, err := s.courier.Track(ctx, awb)
	if err != nil {
		return httpx.NewError(502, "courier_unavailable", "Couldn't reach the courier right now. Please try again in a moment.")
	}
	var latest string
	var latestAt time.Time
	for _, e := range events {
		st := courierStatusToShipment(e.Status)
		s.addEventAt(ctx, s.repo.pool, shipmentID, st, e.Location, e.Description, e.EventTime)
		if latest == "" || e.EventTime.After(latestAt) {
			latest, latestAt = st, e.EventTime
		}
	}
	if latest != "" {
		s.applyStatus(ctx, shipmentID, orderID, courierName, awb, status, latest)
	}
	return nil
}

// ── admin: list + summary ──────────────────────────────────────────────────

// AdminFilter narrows the admin shipment list.
type AdminFilter struct {
	Status    string // "", a raw status, or a group: pending | in_transit | delivered | problem | cancelled
	ShipType  string // "", courier | self
	Query     string
	Attention bool
	Limit     int
	Offset    int
}

func statusGroup(v string) []string {
	switch v {
	case "":
		return nil
	case "pending":
		return []string{"created", "label_generated", "pickup_scheduled"}
	case "in_transit":
		return []string{"picked_up", "in_transit", "out_for_delivery"}
	case "problem":
		return []string{"failed", "rto"}
	}
	return []string{v}
}

func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(strings.TrimSpace(q)) + "%"
}

// ListAdmin returns a page of shipments across all sellers.
func (s *Service) ListAdmin(ctx context.Context, f AdminFilter) ([]Shipment, int64, error) {
	conds := []string{"TRUE"}
	args := []any{}
	add := func(clause string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(clause, "?", fmt.Sprintf("$%d", len(args))))
	}
	if g := statusGroup(f.Status); len(g) > 0 {
		add("s.status = ANY(?)", g)
	}
	if f.ShipType == "courier" || f.ShipType == "self" {
		add("s.ship_type = ?", f.ShipType)
	}
	if strings.TrimSpace(f.Query) != "" {
		add(`(o.order_number ILIKE ? OR s.awb_number ILIKE ? OR o.ship_contact_name ILIKE ? OR sl.store_name ILIKE ?)`,
			likePattern(f.Query))
	}
	if f.Attention {
		conds = append(conds, attentionSQL)
	}
	where := strings.Join(conds, " AND ")

	var total int64
	if err := s.repo.pool.QueryRow(ctx, `
		SELECT count(*) FROM shipments s
		JOIN orders o ON o.id = s.order_id
		LEFT JOIN sellers sl ON sl.id = s.seller_id
		WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	args = append(args, limit, f.Offset)
	rows, err := s.repo.pool.Query(ctx, shipmentSelect+` WHERE `+where+
		fmt.Sprintf(` ORDER BY s.created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, err := collect(rows)
	return list, total, err
}

// Summary counts shipments by stage for the admin dashboard strip.
type Summary struct {
	Total     int `json:"total"`
	Pending   int `json:"pending"`
	InTransit int `json:"in_transit"`
	Delivered int `json:"delivered"`
	Problem   int `json:"problem"`
	Cancelled int `json:"cancelled"`
	Attention int `json:"attention"`
}

// SummaryAdmin returns the stage counts across every shipment.
func (s *Service) SummaryAdmin(ctx context.Context) (*Summary, error) {
	var sm Summary
	err := s.repo.pool.QueryRow(ctx, `
		SELECT count(*)::int,
		       count(*) FILTER (WHERE s.status IN ('created','label_generated','pickup_scheduled'))::int,
		       count(*) FILTER (WHERE s.status IN ('picked_up','in_transit','out_for_delivery'))::int,
		       count(*) FILTER (WHERE s.status = 'delivered')::int,
		       count(*) FILTER (WHERE s.status IN ('failed','rto'))::int,
		       count(*) FILTER (WHERE s.status = 'cancelled')::int,
		       count(*) FILTER (WHERE `+attentionSQL+`)::int
		FROM shipments s`).
		Scan(&sm.Total, &sm.Pending, &sm.InTransit, &sm.Delivered, &sm.Problem, &sm.Cancelled, &sm.Attention)
	if err != nil {
		return nil, err
	}
	return &sm, nil
}
