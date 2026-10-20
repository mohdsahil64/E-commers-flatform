// Package returns implements the return / exchange / refund lifecycle:
//
//	customer requests  ->  seller approves or rejects (with reason)
//	                   ->  seller marks goods received
//	                   ->  admin pays the refund out-of-band and records the UTR
//
// Approve / reject / received are SELLER actions. Admin is read-only over the
// return itself and only acts at the refund (UTR) stage, which unlocks once the
// seller has approved + received. Every transition is recorded in return_events
// so all three portals can render the same timeline (who, when, why).
package returns

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
)

// ReturnWindowDays is how long after delivery a customer may raise a
// return/exchange. Enforced server-side; the UI mirrors it.
const ReturnWindowDays = 7

// MaxReturnAttempts is how many times a customer may request a return for the
// same order item. After this many rejections, the item is permanently blocked
// from further return requests. Enforced server-side; the UI mirrors it.
const MaxReturnAttempts = 2

// Event is one entry in a return's status history.
type Event struct {
	Status    string    `json:"status"`
	ActorRole string    `json:"actor_role"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

// Photo is a customer-uploaded proof image.
type Photo struct {
	FileKey string `json:"file_key"`
}

// Item is a line within a return, with product context.
type Item struct {
	OrderItemID  string  `json:"order_item_id"`
	ProductTitle string  `json:"product_title"`
	Image        string  `json:"image"`
	VariantLabel string  `json:"variant_label"`
	SKU          string  `json:"sku"`
	Quantity     int     `json:"quantity"`
	RefundAmount float64 `json:"refund_amount"`
}

// Return view model (rich — used by all three portals).
type Return struct {
	ID           string    `json:"id"`
	ReturnNumber string    `json:"return_number"`
	OrderID      string    `json:"order_id"`
	OrderNumber  string    `json:"order_number"`
	CustomerID   string    `json:"customer_id"`
	CustomerName string    `json:"customer_name"`
	SellerID     string    `json:"seller_id"`
	SellerName   string    `json:"seller_name"`
	Type         string    `json:"type"`           // return | exchange
	ReasonCode   string    `json:"reason_code"`
	Reason       string    `json:"reason"`
	RejectReason string    `json:"reject_reason"`
	Status       string    `json:"status"`
	// Refund destination (captured at request time).
	RefundMethod  string `json:"refund_method"` // upi | bank
	RefundUPI     string `json:"refund_upi"`
	RefundAccName string `json:"refund_acc_name"`
	RefundAccNo   string `json:"refund_acc_no"`
	RefundIFSC    string `json:"refund_ifsc"`
	// Refund payout (filled by admin).
	RefundAmount float64 `json:"refund_amount"`
	RefundStatus string  `json:"refund_status"`
	RefundUTR    string  `json:"refund_utr"`
	CreatedAt    time.Time `json:"created_at"`
	Items        []Item    `json:"items"`
	Photos       []Photo   `json:"photos"`
	Events       []Event   `json:"events"`
}

// Repository provides return persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a returns Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool exposes the underlying pool (used by the uploads-linked photo insert).
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

// NotifyHook is called after a return/exchange is requested so other modules
// (email/notifications) can react without this package importing them.
type NotifyHook func(ctx context.Context, returnID string)

// Service holds return business logic.
type Service struct {
	repo   *Repository
	notify NotifyHook
}

// NewService builds a returns Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo, notify: func(context.Context, string) {}}
}

// WithNotifier attaches the post-request notification hook.
func (s *Service) WithNotifier(h NotifyHook) *Service {
	if h != nil {
		s.notify = h
	}
	return s
}

// RequestInput is the customer return request.
type RequestInput struct {
	UserID       string
	OrderItemID  string
	Type         string // return | exchange
	ReasonCode   string
	Reason       string
	RefundMethod string   // upi | bank  (required for 'return')
	RefundUPI    string
	RefundAccName string
	RefundAccNo  string
	RefundIFSC   string
	PhotoKeys    []string
}

// Request creates a return/exchange for a delivered order item owned by the
// requesting customer, within the return window.
func (s *Service) Request(ctx context.Context, in RequestInput) (string, error) {
	if in.Type == "" {
		in.Type = "return"
	}
	// Refund destination is required for a refund-type return (not exchange).
	if in.Type == "return" {
		switch in.RefundMethod {
		case "upi":
			if in.RefundUPI == "" {
				return "", httpx.BadRequest("validation_failed", "UPI ID is required for a UPI refund")
			}
		case "bank":
			if in.RefundAccName == "" || in.RefundAccNo == "" || in.RefundIFSC == "" {
				return "", httpx.BadRequest("validation_failed", "Account name, number and IFSC are required for a bank refund")
			}
		default:
			return "", httpx.BadRequest("validation_failed", "Choose a refund method (UPI or bank)")
		}
	}

	var returnID string
	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var orderID, sellerID, customerID string
		var qty int
		var itemStatus string
		var deliveredAt *time.Time
		err := tx.QueryRow(ctx, `
			SELECT oi.order_id, oi.seller_id, o.customer_id, oi.quantity,
			       oi.item_status, oi.delivered_at
			FROM order_items oi
			JOIN orders o ON o.id = oi.order_id
			JOIN customers cu ON cu.id = o.customer_id
			WHERE oi.id = $1 AND cu.user_id = $2`, in.OrderItemID, in.UserID).
			Scan(&orderID, &sellerID, &customerID, &qty, &itemStatus, &deliveredAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.Forbidden("not_eligible", "You can only return items you purchased")
		}
		if err != nil {
			return err
		}
		if itemStatus != "delivered" {
			return httpx.BadRequest("not_delivered", "Only delivered items can be returned")
		}
		// Enforce the return window (delivery + N days).
		if deliveredAt != nil && time.Since(*deliveredAt) > ReturnWindowDays*24*time.Hour {
			return httpx.BadRequest("window_closed",
				fmt.Sprintf("The %d-day return window for this item has closed", ReturnWindowDays))
		}
		// Block a request if there's already an open (non-rejected) return for
		// this item, or if the attempt cap (rejections) has been reached.
		var openCount, rejectedCount int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FILTER (WHERE r.status <> 'rejected'),
			       count(*) FILTER (WHERE r.status = 'rejected')
			FROM returns r
			JOIN return_items ri ON ri.return_id = r.id
			WHERE ri.order_item_id = $1`, in.OrderItemID).Scan(&openCount, &rejectedCount); err != nil {
			return err
		}
		if openCount > 0 {
			return httpx.Conflict("already_requested", "A return for this item is already in progress")
		}
		if rejectedCount >= MaxReturnAttempts {
			return httpx.BadRequest("attempts_exhausted",
				fmt.Sprintf("This item's return was rejected %d times and can no longer be returned", MaxReturnAttempts))
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO returns (return_number, order_id, customer_id, seller_id, type,
			    reason_code, reason, status, refund_method, refund_upi, refund_acc_name, refund_acc_no, refund_ifsc)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'requested',$8,$9,$10,$11,$12) RETURNING id`,
			returnNumber(), orderID, customerID, sellerID, in.Type,
			in.ReasonCode, in.Reason, in.RefundMethod, in.RefundUPI, in.RefundAccName, in.RefundAccNo, in.RefundIFSC).
			Scan(&returnID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO return_items (return_id, order_item_id, quantity, reason)
			VALUES ($1,$2,$3,$4)`, returnID, in.OrderItemID, qty, in.Reason); err != nil {
			return err
		}
		for i, key := range in.PhotoKeys {
			if key == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO return_photos (return_id, file_key, sort_order) VALUES ($1,$2,$3)`,
				returnID, key, i); err != nil {
				return err
			}
		}
		if err := addEvent(ctx, tx, returnID, "requested", "customer", in.UserID, in.Reason); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE order_items SET item_status='return_requested' WHERE id=$1`, in.OrderItemID)
		return err
	})
	if err == nil && returnID != "" {
		s.notify(context.Background(), returnID)
	}
	return returnID, err
}

// Approve approves a return request (SELLER only).
func (s *Service) Approve(ctx context.Context, returnID, userID string) error {
	return s.transition(ctx, returnID, userID, "requested", "approved", "return_approved", "seller", "")
}

// Reject rejects a return request with a reason (SELLER only).
func (s *Service) Reject(ctx context.Context, returnID, userID, reason string) error {
	if reason == "" {
		return httpx.BadRequest("validation_failed", "A reason is required to reject a return")
	}
	return s.transitionReject(ctx, returnID, userID, reason)
}

// MarkReceived marks returned goods received, books stock back and creates a
// pending refund (SELLER only). The refund then awaits admin payout.
func (s *Service) MarkReceived(ctx context.Context, returnID, userID string) error {
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var orderID, status, refundMethod string
		err := tx.QueryRow(ctx,
			`SELECT order_id, status, refund_method FROM returns WHERE id=$1 FOR UPDATE`, returnID).
			Scan(&orderID, &status, &refundMethod)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("return_not_found", "Return not found")
		}
		if err != nil {
			return err
		}
		if status != "approved" {
			return httpx.Conflict("invalid_state", "Return must be approved before it can be received")
		}

		rows, err := tx.Query(ctx, `
			SELECT ri.order_item_id, ri.quantity, oi.seller_listing_id, oi.gross_amount
			FROM return_items ri JOIN order_items oi ON oi.id = ri.order_item_id
			WHERE ri.return_id = $1`, returnID)
		if err != nil {
			return err
		}
		type ri struct {
			itemID, listingID string
			qty               int
			gross             float64
		}
		var items []ri
		var refundTotal float64
		for rows.Next() {
			var x ri
			if err := rows.Scan(&x.itemID, &x.qty, &x.listingID, &x.gross); err != nil {
				rows.Close()
				return err
			}
			items = append(items, x)
			refundTotal += x.gross
		}
		rows.Close()

		for _, x := range items {
			var avail int
			if err := tx.QueryRow(ctx, `
				UPDATE inventory SET available_quantity = available_quantity + $2,
				       returned_quantity = returned_quantity + $2
				WHERE seller_listing_id = $1 RETURNING available_quantity`,
				x.listingID, x.qty).Scan(&avail); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO inventory_transactions (seller_listing_id, change_type, quantity_delta, balance_after, reference_type, reference_id)
				VALUES ($1,'return_in',$2,$3,'return',$4)`, x.listingID, x.qty, avail, returnID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE order_items SET item_status='return_received', refund_amount=$2 WHERE id=$1`,
				x.itemID, x.gross); err != nil {
				return err
			}
		}

		if _, err := tx.Exec(ctx,
			`UPDATE returns SET status='refund_processing' WHERE id=$1`, returnID); err != nil {
			return err
		}
		// Create a pending refund linked to this return; admin pays it out.
		if _, err := tx.Exec(ctx, `
			INSERT INTO refunds (order_id, return_id, amount, method, reason, status, initiated_by)
			VALUES ($1,$2,$3,$4,'Return received',$5,$6)`,
			orderID, returnID, refundTotal, refundMethod, "pending", userID); err != nil {
			return err
		}
		return addEvent(ctx, tx, returnID, "refund_processing", "seller", userID, "Goods received; refund pending")
	})
}

// RefundPayout is the admin action that pays a refund and records the UTR. It
// is only allowed once the return is in refund_processing (i.e. the seller has
// approved + received), which is what unlocks the admin's refund screen.
type RefundPayoutInput struct {
	ReturnID string
	AdminID  string
	UTR      string
}

func (s *Service) RefundPayout(ctx context.Context, in RefundPayoutInput) error {
	if in.UTR == "" {
		return httpx.BadRequest("validation_failed", "UTR / reference number is required")
	}
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx,
			`SELECT status FROM returns WHERE id=$1 FOR UPDATE`, in.ReturnID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.NotFound("return_not_found", "Return not found")
			}
			return err
		}
		if status != "refund_processing" {
			return httpx.Conflict("invalid_state",
				"Refund can only be paid after the seller has approved and received the return")
		}
		ct, err := tx.Exec(ctx, `
			UPDATE refunds SET status='completed', utr=$2, provider_ref=$2, processed_at=now()
			WHERE return_id=$1 AND status='pending'`, in.ReturnID, in.UTR)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.Conflict("invalid_state", "No pending refund for this return")
		}
		if _, err := tx.Exec(ctx,
			`UPDATE returns SET status='refunded' WHERE id=$1`, in.ReturnID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE order_items SET item_status='refunded'
			WHERE id IN (SELECT order_item_id FROM return_items WHERE return_id=$1)`, in.ReturnID); err != nil {
			return err
		}
		return addEvent(ctx, tx, in.ReturnID, "refunded", "admin", in.AdminID, "Refund paid · UTR "+in.UTR)
	})
}

// ---- transitions ----

func (s *Service) transition(ctx context.Context, returnID, userID, from, to, itemStatus, role, note string) error {
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE returns SET status=$3, approved_by=$4 WHERE id=$1 AND status=$2`,
			returnID, from, to, userID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.Conflict("invalid_state", "Return is not in the expected state")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE order_items SET item_status=$2
			WHERE id IN (SELECT order_item_id FROM return_items WHERE return_id=$1)`,
			returnID, itemStatus); err != nil {
			return err
		}
		return addEvent(ctx, tx, returnID, to, role, userID, note)
	})
}

func (s *Service) transitionReject(ctx context.Context, returnID, userID, reason string) error {
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE returns SET status='rejected', reject_reason=$3, approved_by=$2
			 WHERE id=$1 AND status='requested'`, returnID, userID, reason)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.Conflict("invalid_state", "Return is not in the expected state")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE order_items SET item_status='delivered'
			WHERE id IN (SELECT order_item_id FROM return_items WHERE return_id=$1)`, returnID); err != nil {
			return err
		}
		return addEvent(ctx, tx, returnID, "rejected", "seller", userID, reason)
	})
}

func addEvent(ctx context.Context, tx pgx.Tx, returnID, status, role, userID, note string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO return_events (return_id, status, actor_role, actor_id, note)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5)`, returnID, status, role, userID, note)
	return err
}

// ---- listing / detail ----

// ListForCustomer returns a customer's returns (rich).
func (s *Service) ListForCustomer(ctx context.Context, userID string) ([]Return, error) {
	return s.list(ctx, `WHERE r.customer_id = (SELECT id FROM customers WHERE user_id = $1)`, userID)
}

// ListForSeller returns a seller's returns (rich).
func (s *Service) ListForSeller(ctx context.Context, sellerID string) ([]Return, error) {
	return s.list(ctx, `WHERE r.seller_id = $1`, sellerID)
}

// ListAll returns all returns for admin (rich), optionally filtered by status.
func (s *Service) ListAll(ctx context.Context, status string) ([]Return, error) {
	if status == "" {
		return s.list(ctx, ``)
	}
	return s.list(ctx, `WHERE r.status = $1`, status)
}

func (s *Service) list(ctx context.Context, where string, args ...any) ([]Return, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT r.id, r.return_number, r.order_id, coalesce(o.order_number,''),
		       r.customer_id, coalesce(cu.display_name,''),
		       r.seller_id, coalesce(se.store_name,''),
		       r.type, r.reason_code, r.reason, r.reject_reason, r.status,
		       r.refund_method, r.refund_upi, r.refund_acc_name, r.refund_acc_no, r.refund_ifsc,
		       coalesce(rf.amount,0), coalesce(rf.status,''), coalesce(rf.utr,''),
		       r.created_at
		FROM returns r
		JOIN orders o     ON o.id = r.order_id
		LEFT JOIN customers cu ON cu.id = r.customer_id
		LEFT JOIN sellers se   ON se.id = r.seller_id
		LEFT JOIN LATERAL (
			SELECT amount, status, utr FROM refunds WHERE return_id = r.id ORDER BY created_at DESC LIMIT 1
		) rf ON TRUE
		`+where+` ORDER BY r.created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Return
	ids := []string{}
	index := map[string]int{}
	for rows.Next() {
		var r Return
		if err := rows.Scan(&r.ID, &r.ReturnNumber, &r.OrderID, &r.OrderNumber,
			&r.CustomerID, &r.CustomerName, &r.SellerID, &r.SellerName,
			&r.Type, &r.ReasonCode, &r.Reason, &r.RejectReason, &r.Status,
			&r.RefundMethod, &r.RefundUPI, &r.RefundAccName, &r.RefundAccNo, &r.RefundIFSC,
			&r.RefundAmount, &r.RefundStatus, &r.RefundUTR, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Items = []Item{}
		r.Photos = []Photo{}
		r.Events = []Event{}
		index[r.ID] = len(out)
		out = append(out, r)
		ids = append(ids, r.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return out, nil
	}
	if err := s.attachItems(ctx, out, index, ids); err != nil {
		return nil, err
	}
	if err := s.attachPhotos(ctx, out, index, ids); err != nil {
		return nil, err
	}
	if err := s.attachEvents(ctx, out, index, ids); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) attachItems(ctx context.Context, out []Return, index map[string]int, ids []string) error {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT ri.return_id, ri.order_item_id, oi.product_title,
		       coalesce((SELECT file_key FROM product_images pi
		                 JOIN seller_listings sl ON sl.product_id = pi.product_id
		                 WHERE sl.id = oi.seller_listing_id
		                 ORDER BY pi.is_primary DESC, pi.sort_order LIMIT 1),''),
		       oi.variant_label, oi.sku, ri.quantity, oi.gross_amount
		FROM return_items ri
		JOIN order_items oi ON oi.id = ri.order_item_id
		WHERE ri.return_id = ANY($1)`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rid string
		var it Item
		if err := rows.Scan(&rid, &it.OrderItemID, &it.ProductTitle, &it.Image,
			&it.VariantLabel, &it.SKU, &it.Quantity, &it.RefundAmount); err != nil {
			return err
		}
		if i, ok := index[rid]; ok {
			out[i].Items = append(out[i].Items, it)
		}
	}
	return rows.Err()
}

func (s *Service) attachPhotos(ctx context.Context, out []Return, index map[string]int, ids []string) error {
	rows, err := s.repo.pool.Query(ctx,
		`SELECT return_id, file_key FROM return_photos WHERE return_id = ANY($1) ORDER BY sort_order`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rid string
		var p Photo
		if err := rows.Scan(&rid, &p.FileKey); err != nil {
			return err
		}
		if i, ok := index[rid]; ok {
			out[i].Photos = append(out[i].Photos, p)
		}
	}
	return rows.Err()
}

func (s *Service) attachEvents(ctx context.Context, out []Return, index map[string]int, ids []string) error {
	rows, err := s.repo.pool.Query(ctx,
		`SELECT return_id, status, actor_role, note, created_at
		 FROM return_events WHERE return_id = ANY($1) ORDER BY created_at`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var rid string
		var e Event
		if err := rows.Scan(&rid, &e.Status, &e.ActorRole, &e.Note, &e.CreatedAt); err != nil {
			return err
		}
		if i, ok := index[rid]; ok {
			out[i].Events = append(out[i].Events, e)
		}
	}
	return rows.Err()
}

func returnNumber() string {
	return fmt.Sprintf("RET-%d", time.Now().UnixNano()%1_000_000_000)
}
