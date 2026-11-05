// Package settlements implements the marketplace financial model: it turns
// delivered order items into immutable settlement records for sellers. Each
// settlement_item settles exactly once (enforced by a unique constraint).
package settlements

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/returns"
)

// HoldDays is how long after delivery an item is held before it can be paid
// out. It equals the customer return window so a payout never races a return.
const HoldDays = returns.ReturnWindowDays

// eligibleWhere filters order_items (alias oi, LEFT JOINed settlement_items
// alias si) down to items that can be settled now. $2 is HoldDays.
//   - delivered (return/exchange requests move the item out of 'delivered')
//   - delivery is at least HoldDays old
//   - not already refunded
//   - no open (unresolved) return/exchange request on it
//   - not already in a settlement
const eligibleWhere = `oi.item_status = 'delivered' AND si.id IS NULL
	AND oi.delivered_at IS NOT NULL
	AND oi.delivered_at <= now() - make_interval(days => $2::int)
	AND oi.refund_amount = 0
	AND NOT EXISTS (
		SELECT 1 FROM return_items ri JOIN returns r ON r.id = ri.return_id
		WHERE ri.order_item_id = oi.id
		  AND r.status NOT IN ('rejected', 'refunded', 'completed'))`

// GenerateResult is one row of a generate-all run.
type GenerateResult struct {
	SellerID     string  `json:"seller_id"`
	StoreName    string  `json:"store_name"`
	SettlementID string  `json:"settlement_id,omitempty"`
	NetPayable   float64 `json:"net_payable,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// GenerateAll creates one settlement per seller that has eligible items. Each
// seller is processed in its own transaction so one failure does not block
// the others.
func (s *Service) GenerateAll(ctx context.Context, adminID string) ([]GenerateResult, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT DISTINCT oi.seller_id, COALESCE(NULLIF(se.store_name,''), oi.seller_id::text)
		FROM order_items oi
		JOIN sellers se ON se.id = oi.seller_id
		LEFT JOIN settlement_items si ON si.order_item_id = oi.id
		WHERE ($1::uuid IS NULL OR oi.seller_id = $1) AND `+eligibleWhere, nil, HoldDays)
	if err != nil {
		return nil, err
	}
	var out []GenerateResult
	for rows.Next() {
		var r GenerateResult
		if err := rows.Scan(&r.SellerID, &r.StoreName); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		id, err := s.Generate(ctx, out[i].SellerID, adminID)
		if err != nil {
			out[i].Error = err.Error()
			continue
		}
		out[i].SettlementID = id
		_ = s.repo.pool.QueryRow(ctx, `SELECT net_payable FROM seller_settlements WHERE id=$1`, id).Scan(&out[i].NetPayable)
	}
	return out, nil
}

// Summary is the seller "when will I get paid" overview. All amounts are net
// of commission and delivery charge (seller_payable - shipping_charge).
type Summary struct {
	ProcessingAmount float64    `json:"processing_amount"`
	ProcessingCount  int        `json:"processing_count"`
	UpcomingAmount   float64    `json:"upcoming_amount"`
	UpcomingCount    int        `json:"upcoming_count"`
	NextPayoutDate   *time.Time `json:"next_payout_date"`
	ReadyAmount      float64    `json:"ready_amount"`
	ReadyCount       int        `json:"ready_count"`
	PaidAmount       float64    `json:"paid_amount"`
	PaidCount        int        `json:"paid_count"`
	LastPaidAt       *time.Time `json:"last_paid_at"`
	LastUTR          string     `json:"last_utr"`
	HoldDays         int        `json:"hold_days"`
}

// SummaryForSeller computes the three payout boxes for a seller.
func (s *Service) SummaryForSeller(ctx context.Context, sellerID string) (*Summary, error) {
	sum := &Summary{HoldDays: HoldDays}
	if err := s.repo.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(seller_payable - shipping_charge),0), count(*)
		FROM order_items
		WHERE seller_id = $1 AND item_status IN
		  ('confirmed','processing','packed','ready_to_ship','shipped','in_transit','out_for_delivery')`,
		sellerID).Scan(&sum.ProcessingAmount, &sum.ProcessingCount); err != nil {
		return nil, err
	}
	// Upcoming: delivered, not yet in a settlement. Items with delivered_at
	// missing fall back to created_at for the payout date.
	if err := s.repo.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(oi.seller_payable - oi.shipping_charge),0), count(*),
		       MIN(COALESCE(oi.delivered_at, oi.created_at)) + make_interval(days => $2::int),
		       COALESCE(SUM(oi.seller_payable - oi.shipping_charge)
		         FILTER (WHERE oi.delivered_at <= now() - make_interval(days => $2::int)),0),
		       count(*) FILTER (WHERE oi.delivered_at <= now() - make_interval(days => $2::int))
		FROM order_items oi
		LEFT JOIN settlement_items si ON si.order_item_id = oi.id
		WHERE oi.seller_id = $1 AND oi.item_status = 'delivered' AND si.id IS NULL
		  AND oi.refund_amount = 0`,
		sellerID, HoldDays).Scan(&sum.UpcomingAmount, &sum.UpcomingCount, &sum.NextPayoutDate,
		&sum.ReadyAmount, &sum.ReadyCount); err != nil {
		return nil, err
	}
	if sum.UpcomingCount == 0 {
		sum.NextPayoutDate = nil
	}
	if err := s.repo.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(net_payable),0), count(*) FROM seller_settlements
		WHERE seller_id = $1 AND status = 'settled'`, sellerID).
		Scan(&sum.PaidAmount, &sum.PaidCount); err != nil {
		return nil, err
	}
	if sum.PaidCount > 0 {
		if err := s.repo.pool.QueryRow(ctx, `
			SELECT COALESCE(processed_at, created_at), utr_reference FROM seller_settlements
			WHERE seller_id = $1 AND status = 'settled'
			ORDER BY COALESCE(processed_at, created_at) DESC LIMIT 1`, sellerID).
			Scan(&sum.LastPaidAt, &sum.LastUTR); err != nil {
			return nil, err
		}
	}
	return sum, nil
}

// Settlement is a payout record view model.
type Settlement struct {
	ID                string     `json:"id"`
	SettlementNumber  string     `json:"settlement_number"`
	SellerID          string     `json:"seller_id"`
	SellerName        string     `json:"seller_name"`
	PeriodStart       string     `json:"period_start"`
	PeriodEnd         string     `json:"period_end"`
	GrossAmount       float64    `json:"gross_amount"`
	CommissionAmount  float64    `json:"commission_amount"`
	ShippingDeduction float64    `json:"shipping_deduction"`
	OtherFees         float64    `json:"other_fees"`
	RefundAmount      float64    `json:"refund_amount"`
	NetPayable        float64    `json:"net_payable"`
	Status            string     `json:"status"`
	UTRReference      string     `json:"utr_reference"`
	ProcessedAt       *time.Time `json:"processed_at"`
	ItemCount         int        `json:"item_count"`
	CreatedAt         time.Time  `json:"created_at"`
}

// Item is one order line inside a settlement.
type Item struct {
	OrderID      string     `json:"order_id"`
	OrderNumber  string     `json:"order_number"`
	ProductTitle string     `json:"product_title"`
	Quantity     int        `json:"quantity"`
	GrossAmount  float64    `json:"gross_amount"`
	Commission   float64    `json:"commission"`
	Shipping     float64    `json:"shipping"`
	PaymentFee   float64    `json:"payment_fee"`
	NetPayable   float64    `json:"net_payable"`
	DeliveredAt  *time.Time `json:"delivered_at"`
}

// BankAccount is where the admin sends the money. Only returned to admins.
type BankAccount struct {
	AccountHolder string `json:"account_holder"`
	AccountNumber string `json:"account_number"`
	IFSC          string `json:"ifsc"`
	BankName      string `json:"bank_name"`
	Branch        string `json:"branch"`
	Verified      bool   `json:"verified"`
}

// Detail is a settlement with its order lines (and, for admins, the bank to pay).
type Detail struct {
	Settlement
	Items []Item       `json:"items"`
	Bank  *BankAccount `json:"bank,omitempty"`
}

// allowedNext lists the legal status moves. Settled money can only be
// reversed, never silently moved back to pending, so history stays honest.
var allowedNext = map[string][]string{
	"pending":    {"processing", "settled", "failed"},
	"processing": {"settled", "failed"},
	"failed":     {"processing", "settled"},
	"settled":    {"reversed"},
	"reversed":   {},
}

func canMove(from, to string) bool {
	for _, n := range allowedNext[from] {
		if n == to {
			return true
		}
	}
	return false
}

// ReadySeller is one row of the "what would Generate pay now" preview.
type ReadySeller struct {
	SellerID   string  `json:"seller_id"`
	StoreName  string  `json:"store_name"`
	ItemCount  int     `json:"item_count"`
	Gross      float64 `json:"gross_amount"`
	Commission float64 `json:"commission_amount"`
	Shipping   float64 `json:"shipping_deduction"`
	NetPayable float64 `json:"net_payable"`
	HasBank    bool    `json:"has_bank"`
}

// Preview shows, per seller, what is eligible to be paid right now (delivered
// at least HoldDays ago, no open return, not yet in a settlement).
func (s *Service) Preview(ctx context.Context) ([]ReadySeller, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT oi.seller_id, COALESCE(NULLIF(se.store_name,''), oi.seller_id::text),
		       count(*), COALESCE(SUM(oi.gross_amount),0), COALESCE(SUM(oi.commission_amount),0),
		       COALESCE(SUM(oi.shipping_charge),0),
		       COALESCE(SUM(oi.seller_payable - oi.shipping_charge),0),
		       EXISTS (SELECT 1 FROM seller_bank_accounts b WHERE b.seller_id = oi.seller_id)
		FROM order_items oi
		JOIN sellers se ON se.id = oi.seller_id
		LEFT JOIN settlement_items si ON si.order_item_id = oi.id
		WHERE ($1::uuid IS NULL OR oi.seller_id = $1) AND `+eligibleWhere+`
		GROUP BY oi.seller_id, se.store_name
		ORDER BY 7 DESC`, nil, HoldDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ReadySeller{}
	for rows.Next() {
		var r ReadySeller
		if err := rows.Scan(&r.SellerID, &r.StoreName, &r.ItemCount, &r.Gross, &r.Commission,
			&r.Shipping, &r.NetPayable, &r.HasBank); err != nil {
			return nil, err
		}
		r.Gross, r.Commission, r.Shipping, r.NetPayable =
			round2(r.Gross), round2(r.Commission), round2(r.Shipping), round2(r.NetPayable)
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDetail loads one settlement with its lines. When sellerID is non-empty the
// settlement must belong to that seller (seller portal); otherwise it is the
// admin view, which also includes the seller's bank account to pay.
func (s *Service) GetDetail(ctx context.Context, id, sellerID string) (*Detail, error) {
	where := `WHERE ss.id = $1`
	args := []any{id}
	if sellerID != "" {
		where += ` AND ss.seller_id = $2`
		args = append(args, sellerID)
	}
	list, err := s.list(ctx, where, args...)
	if err != nil {
		// A malformed id is a "not found" to the caller, not a server error.
		return nil, httpx.NotFound("settlement_not_found", "Settlement not found")
	}
	if len(list) == 0 {
		return nil, httpx.NotFound("settlement_not_found", "Settlement not found")
	}
	d := &Detail{Settlement: list[0], Items: []Item{}}

	rows, err := s.repo.pool.Query(ctx, `
		SELECT si.order_id, o.order_number, oi.product_title, oi.quantity,
		       si.gross_amount, si.commission, si.shipping, si.payment_fee, si.net_payable, oi.delivered_at
		FROM settlement_items si
		JOIN order_items oi ON oi.id = si.order_item_id
		JOIN orders o ON o.id = si.order_id
		WHERE si.settlement_id = $1
		ORDER BY oi.delivered_at NULLS LAST, o.order_number`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.OrderID, &it.OrderNumber, &it.ProductTitle, &it.Quantity,
			&it.GrossAmount, &it.Commission, &it.Shipping, &it.PaymentFee, &it.NetPayable, &it.DeliveredAt); err != nil {
			return nil, err
		}
		d.Items = append(d.Items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if sellerID == "" { // admin only: where to send the money
		var b BankAccount
		err := s.repo.pool.QueryRow(ctx, `
			SELECT account_holder, account_number, ifsc, bank_name, branch, verified
			FROM seller_bank_accounts WHERE seller_id = $1
			ORDER BY is_primary DESC, created_at DESC LIMIT 1`, d.SellerID).
			Scan(&b.AccountHolder, &b.AccountNumber, &b.IFSC, &b.BankName, &b.Branch, &b.Verified)
		if err == nil {
			d.Bank = &b
		} else if err != pgx.ErrNoRows {
			return nil, err
		}
	}
	return d, nil
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
			WHERE oi.seller_id = $1 AND `+eligibleWhere+`
			FOR UPDATE OF oi`, sellerID, HoldDays)
		if err != nil {
			return err
		}
		type li struct {
			id, orderID                                       string
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
				settlementID, x.orderID, x.id, x.gross, x.commission, x.shipping, x.fee, x.refund, round2(x.payable-x.shipping)); err != nil {
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

// MarkStatus moves a settlement along its lifecycle
// (pending -> processing -> settled, failed can be retried, settled can only be
// reversed). Marking it settled needs the bank UTR so the seller has proof.
// The check and the update run in one transaction on a locked row, so two admins
// clicking at once cannot both win.
func (s *Service) MarkStatus(ctx context.Context, actorID, settlementID, status, utr string) error {
	if _, ok := allowedNext[status]; !ok || status == "pending" {
		return httpx.BadRequest("validation_failed", "Invalid settlement status")
	}
	utr = strings.TrimSpace(utr)
	if len(utr) > 64 {
		return httpx.BadRequest("validation_failed", "UTR / reference is too long (max 64 characters)")
	}
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var current, existingUTR string
		if err := tx.QueryRow(ctx,
			`SELECT status, utr_reference FROM seller_settlements WHERE id = $1 FOR UPDATE`, settlementID).
			Scan(&current, &existingUTR); err != nil {
			if err == pgx.ErrNoRows {
				return httpx.NotFound("settlement_not_found", "Settlement not found")
			}
			return err
		}
		if !canMove(current, status) {
			return httpx.Conflict("invalid_transition",
				fmt.Sprintf("A %s settlement cannot be changed to %s", current, status))
		}
		if status == "settled" && utr == "" && existingUTR == "" {
			return httpx.BadRequest("utr_required", "Enter the bank UTR / reference number to mark this as paid")
		}
		_, err := tx.Exec(ctx, `
			UPDATE seller_settlements
			SET status = $2, utr_reference = COALESCE(NULLIF($3,''), utr_reference),
			    processed_by = NULLIF($4,'')::uuid,
			    processed_at = CASE WHEN $2 = 'settled' THEN now() ELSE processed_at END
			WHERE id = $1`, settlementID, status, utr, actorID)
		return err
	})
	if err != nil {
		return err
	}
	s.audit(ctx, actorID, "settlement."+status, settlementID)
	if status == "settled" {
		s.payout(context.Background(), settlementID)
	}
	return nil
}

// ListForSeller returns a seller's settlements.
func (s *Service) ListForSeller(ctx context.Context, sellerID string) ([]Settlement, error) {
	return s.list(ctx, `WHERE ss.seller_id = $1`, sellerID)
}

// ListAll returns settlements for admin (optional status filter).
func (s *Service) ListAll(ctx context.Context, status string) ([]Settlement, error) {
	if status == "" {
		return s.list(ctx, ``)
	}
	return s.list(ctx, `WHERE ss.status = $1`, status)
}

func (s *Service) list(ctx context.Context, where string, args ...any) ([]Settlement, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT ss.id, ss.settlement_number, ss.seller_id, COALESCE(NULLIF(se.store_name,''), ''),
		       ss.period_start::text, ss.period_end::text,
		       ss.gross_amount, ss.commission_amount, ss.shipping_deduction, ss.other_fees, ss.refund_amount,
		       ss.net_payable, ss.status, ss.utr_reference, ss.processed_at,
		       (SELECT count(*) FROM settlement_items x WHERE x.settlement_id = ss.id),
		       ss.created_at
		FROM seller_settlements ss
		JOIN sellers se ON se.id = ss.seller_id `+where+` ORDER BY ss.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Settlement{}
	for rows.Next() {
		var st Settlement
		if err := rows.Scan(&st.ID, &st.SettlementNumber, &st.SellerID, &st.SellerName, &st.PeriodStart,
			&st.PeriodEnd, &st.GrossAmount, &st.CommissionAmount, &st.ShippingDeduction,
			&st.OtherFees, &st.RefundAmount, &st.NetPayable, &st.Status, &st.UTRReference,
			&st.ProcessedAt, &st.ItemCount, &st.CreatedAt); err != nil {
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
