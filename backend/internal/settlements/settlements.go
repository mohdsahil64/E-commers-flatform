// Package settlements implements the marketplace financial model: it turns
// delivered order items into immutable settlement records for sellers. Each
// settlement_item settles exactly once (enforced by a unique constraint).
package settlements

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// Settlement is a payout record view model.
type Settlement struct {
	ID                string    `json:"id"`
	SettlementNumber  string    `json:"settlement_number"`
	SellerID          string    `json:"seller_id"`
	PeriodStart       string    `json:"period_start"`
	PeriodEnd         string    `json:"period_end"`
	GrossAmount       float64   `json:"gross_amount"`
	CommissionAmount  float64   `json:"commission_amount"`
	ShippingDeduction float64   `json:"shipping_deduction"`
	OtherFees         float64   `json:"other_fees"`
	RefundAmount      float64   `json:"refund_amount"`
	NetPayable        float64   `json:"net_payable"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

// Repository provides settlement persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a settlements Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Service holds settlement business logic.
// AuditHook records settlement audit events without importing the audit package.
type AuditHook func(ctx context.Context, actorID, action, settlementID string)

// PayoutHook is called after a settlement is marked settled (paid) so other
// modules (email) can notify the seller.
type PayoutHook func(ctx context.Context, settlementID string)

type Service struct {
	repo   *Repository
	audit  AuditHook
	payout PayoutHook
}

// NewService builds a settlements Service.
func NewService(repo *Repository) *Service {
	return &Service{
		repo:   repo,
		audit:  func(context.Context, string, string, string) {},
		payout: func(context.Context, string) {},
	}
}

// WithAuditHook attaches an audit hook.
func (s *Service) WithAuditHook(h AuditHook) *Service {
	if h != nil {
		s.audit = h
	}
	return s
}

// WithPayoutHook attaches the hook fired when a settlement is paid.
func (s *Service) WithPayoutHook(h PayoutHook) *Service {
	if h != nil {
		s.payout = h
	}
	return s
}

// Generate creates a settlement for a seller from all delivered, unsettled
// order items. It is atomic: settlement + line items are inserted together, and
// each order_item can only be included once (unique constraint on
// settlement_items.order_item_id). Returns the new settlement id, or an error
// if there is nothing to settle.
func (s *Service) Generate(ctx context.Context, sellerID, adminID string) (string, error) {
	var settlementID string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		// Collect delivered, unsettled items for this seller.
		rows, err := tx.Query(ctx, `
			SELECT oi.id, oi.order_id, oi.gross_amount, oi.commission_amount,
			       oi.shipping_charge, oi.payment_fee, oi.refund_amount, oi.seller_payable
			FROM order_items oi
			LEFT JOIN settlement_items si ON si.order_item_id = oi.id
			WHERE oi.seller_id = $1 AND oi.item_status = 'delivered' AND si.id IS NULL
			FOR UPDATE OF oi`, sellerID)
		if err != nil {
			return err
		}
		type li struct {
			id, orderID                                              string
			gross, commission, shipping, fee, refund, payable float64
		}
		var items []li
		var gross, commission, shipping, fees, refund, net float64
		for rows.Next() {
			var x li
			if err := rows.Scan(&x.id, &x.orderID, &x.gross, &x.commission, &x.shipping,
				&x.fee, &x.refund, &x.payable); err != nil {
				rows.Close()
				return err
			}
			items = append(items, x)
			gross += x.gross
			commission += x.commission
			shipping += x.shipping
			fees += x.fee
			refund += x.refund
			// Recover the actual courier freight (fronted by the platform) from
			// the seller's payout. seller_payable was computed at checkout before
			// the real freight was known, so deduct shipping here.
			net += x.payable - x.shipping
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(items) == 0 {
			return httpx.BadRequest("nothing_to_settle", "No delivered, unsettled items for this seller")
		}

		now := time.Now()
		periodStart := now.AddDate(0, 0, -30)
		if err := tx.QueryRow(ctx, `
			INSERT INTO seller_settlements (settlement_number, seller_id, period_start, period_end,
				gross_amount, commission_amount, shipping_deduction, other_fees, refund_amount,
				net_payable, status, processed_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending',$11)
			RETURNING id`,
			settlementNumber(), sellerID, periodStart, now,
			round2(gross), round2(commission), round2(shipping), round2(fees), round2(refund),
			round2(net), adminID).Scan(&settlementID); err != nil {
			return err
		}
		for _, x := range items {
			if _, err := tx.Exec(ctx, `
				INSERT INTO settlement_items (settlement_id, order_id, order_item_id,
					gross_amount, commission, shipping, payment_fee, refund, net_payable)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
				settlementID, x.orderID, x.id, x.gross, x.commission, x.shipping, x.fee, x.refund, x.payable); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	s.audit(ctx, adminID, "settlement.generated", settlementID)
	return settlementID, nil
}

// MarkStatus transitions a settlement status (processing/settled/failed/reversed).
func (s *Service) MarkStatus(ctx context.Context, settlementID, status, utr string) error {
	valid := map[string]bool{"processing": true, "settled": true, "failed": true, "reversed": true}
	if !valid[status] {
		return httpx.BadRequest("validation_failed", "Invalid settlement status")
	}
	ct, err := s.repo.pool.Exec(ctx, `
		UPDATE seller_settlements
		SET status = $2, utr_reference = COALESCE(NULLIF($3,''), utr_reference),
		    processed_at = CASE WHEN $2 = 'settled' THEN now() ELSE processed_at END
		WHERE id = $1`, settlementID, status, utr)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return httpx.NotFound("settlement_not_found", "Settlement not found")
	}
	s.audit(ctx, "", "settlement."+status, settlementID)
	if status == "settled" {
		s.payout(context.Background(), settlementID)
	}
	return nil
}

// ListForSeller returns a seller's settlements.
func (s *Service) ListForSeller(ctx context.Context, sellerID string) ([]Settlement, error) {
	return s.list(ctx, `WHERE seller_id = $1`, sellerID)
}

// ListAll returns settlements for admin (optional status filter).
func (s *Service) ListAll(ctx context.Context, status string) ([]Settlement, error) {
	if status == "" {
		return s.list(ctx, ``)
	}
	return s.list(ctx, `WHERE status = $1`, status)
}

func (s *Service) list(ctx context.Context, where string, args ...any) ([]Settlement, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT id, settlement_number, seller_id, period_start::text, period_end::text,
		       gross_amount, commission_amount, shipping_deduction, other_fees, refund_amount,
		       net_payable, status, created_at
		FROM seller_settlements `+where+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Settlement
	for rows.Next() {
		var st Settlement
		if err := rows.Scan(&st.ID, &st.SettlementNumber, &st.SellerID, &st.PeriodStart,
			&st.PeriodEnd, &st.GrossAmount, &st.CommissionAmount, &st.ShippingDeduction,
			&st.OtherFees, &st.RefundAmount, &st.NetPayable, &st.Status, &st.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func settlementNumber() string {
	return fmt.Sprintf("STL-%d-%d", time.Now().Year(), time.Now().UnixNano()%1_000_000)
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100.0 }
