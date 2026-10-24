package orders

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/carlitos/backend/internal/cart"
	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/inventory"
)

// Service orchestrates checkout and order lifecycle operations.
type Service struct {
	repo              *Repository
	cart              *cart.Repository
	inv               *inventory.Repository
	defaultCommission float64
	hook              LifecycleHook
	coupons           CouponApplier
}

// WithCoupons attaches a coupon applier so checkout can apply discounts.
func (s *Service) WithCoupons(a CouponApplier) *Service {
	s.coupons = a
	return s
}

// LifecycleHook lets other modules (notifications/audit/settlements) react to
// order events without orders depending on them.
type LifecycleHook func(ctx context.Context, orderID, event, actorID string)

// NewService builds an orders Service.
func NewService(repo *Repository, cartRepo *cart.Repository, inv *inventory.Repository, defaultCommission float64, hook LifecycleHook) *Service {
	if hook == nil {
		hook = func(context.Context, string, string, string) {}
	}
	return &Service{repo: repo, cart: cartRepo, inv: inv, defaultCommission: defaultCommission, hook: hook}
}

// CheckoutInput is the checkout payload.
type CheckoutInput struct {
	UserID        string
	PaymentMethod string // online | cod
	AddressID     string
	CouponCode    string // optional discount coupon
	// SelectedListingIDs, when non-empty, restricts the order to just these
	// cart lines (seller_listing_id). The unselected lines stay in the cart.
	// Empty/nil means "order the whole cart" (backward compatible).
	SelectedListingIDs []string
}

// CouponApplier validates/redeems a coupon during checkout without the orders
// package depending on the coupons package directly (avoids an import cycle;
// wired at composition time).
type CouponApplier interface {
	ValidateForCheckout(ctx context.Context, code string, subtotal float64, customerID string) (couponID, normalizedCode string, discount float64, err error)
	RedeemTx(ctx context.Context, tx pgx.Tx, couponID, code, customerID, orderID string, discount float64) error
}

// Checkout converts the customer's cart into an order. It runs entirely inside
// a single transaction:
//  1. loads cart items with a row lock intent,
//  2. atomically reserves inventory for each item (prevents overselling),
//  3. computes the financial breakdown (commission, tax, totals),
//  4. inserts the order + order_items,
//  5. records the initial status history,
//  6. clears the cart.
//
// If any step fails the whole transaction rolls back, so no stock is leaked
// and no partial order is created.
func (s *Service) Checkout(ctx context.Context, in CheckoutInput) (*Order, error) {
	if in.PaymentMethod != "online" && in.PaymentMethod != "cod" {
		return nil, httpx.BadRequest("validation_failed", "payment_method must be 'online' or 'cod'")
	}

	customerID, err := s.cart.CustomerID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	cartID, err := s.cart.CartIDForCustomer(ctx, customerID)
	if err != nil {
		return nil, httpx.NotFound("cart_not_found", "Cart not found")
	}

	// Resolve shipping address snapshot.
	addr, err := s.loadAddress(ctx, customerID, in.AddressID)
	if err != nil {
		return nil, err
	}

	var order *Order
	err = pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		// Serialize concurrent checkouts of the same cart with a transaction-scoped
		// advisory lock. A double-submit (or racing requests) blocks here until the
		// first checkout commits and clears the cart; the loser then sees an empty
		// cart below and is rejected — preventing duplicate orders / double reserve.
		if _, err := tx.Exec(ctx,
			`SELECT pg_advisory_xact_lock(hashtext($1))`, cartID); err != nil {
			return err
		}
		// Load cart lines (authoritative price from listing, not the snapshot).
		// When SelectedListingIDs is provided, only those lines are ordered; the
		// rest remain in the cart. Empty => the whole cart (backward compatible).
		//
		// $2 is always passed (a text[]); an empty array means "no filter" via the
		// `cardinality($2)=0 OR ...` guard, so the same query serves both modes.
		selected := in.SelectedListingIDs
		if selected == nil {
			selected = []string{}
		}
		rows, err := tx.Query(ctx, `
			SELECT ci.seller_listing_id, ci.quantity, sl.seller_id, sl.selling_price,
			       p.title, p.tax_percent, sl.sku, p.category_id, s.commission_percent,
			       coalesce(s.cod_enabled, TRUE)
			FROM cart_items ci
			JOIN seller_listings sl ON sl.id = ci.seller_listing_id
			JOIN products p ON p.id = sl.product_id
			JOIN sellers s ON s.id = sl.seller_id
			WHERE ci.cart_id = $1
			  AND (cardinality($2::text[]) = 0 OR ci.seller_listing_id = ANY($2::text[]))`,
			cartID, selected)
		if err != nil {
			return err
		}
		type line struct {
			listingID  string
			qty        int
			sellerID   string
			price      float64
			title      string
			taxPct     float64
			sku        string
			categoryID *string
			sellerComm *float64
			codEnabled bool
		}
		var lines []line
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.listingID, &l.qty, &l.sellerID, &l.price, &l.title,
				&l.taxPct, &l.sku, &l.categoryID, &l.sellerComm, &l.codEnabled); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(lines) == 0 {
			return httpx.BadRequest("empty_cart", "Your cart is empty")
		}

		// Cash on Delivery guard: COD is only allowed when EVERY seller in the
		// cart offers it. If any item is from a seller who turned COD off, the
		// customer must pay online. This mirrors the storefront, which disables
		// the COD option in the same situation — this is the authoritative check.
		if in.PaymentMethod == "cod" {
			for _, l := range lines {
				if !l.codEnabled {
					return httpx.BadRequest("cod_unavailable",
						"Cash on Delivery isn't available for one or more items in your cart. Please pay online.")
				}
			}
		}

		// Create the order shell. Order number is a gap-free-ish sequential
		// human-friendly id (CRL-00001, CRL-00002 …) drawn from a DB sequence.
		orderNumber, err := nextOrderNumber(ctx, tx)
		if err != nil {
			return err
		}
		var orderID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO orders (order_number, customer_id, status, payment_method, payment_status,
				ship_contact_name, ship_contact_phone, ship_line1, ship_line2, ship_city,
				ship_state, ship_pincode, ship_country, placed_at)
			VALUES ($1,$2,'pending_payment',$3,'pending',$4,$5,$6,$7,$8,$9,$10,$11, now())
			RETURNING id`,
			orderNumber, customerID, in.PaymentMethod,
			addr.name, addr.phone, addr.line1, addr.line2, addr.city, addr.state, addr.pincode, addr.country,
		).Scan(&orderID); err != nil {
			return err
		}

		var itemsSubtotal, taxTotal, shippingTotal, grandTotal float64
		for _, l := range lines {
			// Reserve stock atomically — fails the tx if insufficient.
			if err := s.inv.ReserveTx(ctx, tx, l.listingID, l.qty); err != nil {
				if err == inventory.ErrInsufficientStock {
					return httpx.Conflict("insufficient_stock",
						fmt.Sprintf("Insufficient stock for %s", l.title))
				}
				return err
			}

			gross := l.price * float64(l.qty)
			tax := round2(gross * l.taxPct / 100.0)
			commPct := s.defaultCommission
			if l.sellerComm != nil {
				commPct = *l.sellerComm
			}
			commission := round2(gross * commPct / 100.0)
			shipping := 0.0 // free shipping in v1; courier layer can override later
			payable := round2(gross - commission)

			itemsSubtotal += gross
			taxTotal += tax
			shippingTotal += shipping
			grandTotal += gross + tax + shipping

			if _, err := tx.Exec(ctx, `
				INSERT INTO order_items (order_id, seller_id, seller_listing_id, product_title,
					variant_label, sku, item_status, quantity, unit_price, gross_amount,
					discount_amount, tax_amount, shipping_charge, commission_percent,
					commission_amount, seller_payable)
				VALUES ($1,$2,$3,$4,'',$5,'confirmed',$6,$7,$8,0,$9,$10,$11,$12,$13)`,
				orderID, l.sellerID, l.listingID, l.title, l.sku, l.qty, l.price, gross,
				tax, shipping, commPct, commission, payable); err != nil {
				return err
			}
		}

		// Apply a coupon discount if one was supplied and is valid. The discount
		// is computed on the server (never trusted from the client) against the
		// items subtotal, redeemed inside this same transaction, and recorded on
		// the order. If the coupon is invalid the whole checkout fails so the
		// customer sees the reason rather than a silently-ignored coupon.
		var discountTotal float64
		var appliedCode string
		if strings.TrimSpace(in.CouponCode) != "" && s.coupons != nil {
			couponID, normCode, disc, cerr := s.coupons.ValidateForCheckout(
				ctx, in.CouponCode, round2(itemsSubtotal), customerID)
			if cerr != nil {
				return cerr
			}
			discountTotal = disc
			appliedCode = normCode
			if err := s.coupons.RedeemTx(ctx, tx, couponID, normCode, customerID, orderID, disc); err != nil {
				return err
			}
		}
		grandTotal = grandTotal - discountTotal
		if grandTotal < 0 {
			grandTotal = 0
		}

		// Finalize totals.
		if _, err := tx.Exec(ctx, `
			UPDATE orders SET items_subtotal=$2, discount_total=$3, tax_total=$4, shipping_total=$5,
				grand_total=$6, coupon_code=NULLIF($7,'')
			WHERE id=$1`,
			orderID, round2(itemsSubtotal), round2(discountTotal), round2(taxTotal),
			round2(shippingTotal), round2(grandTotal), appliedCode); err != nil {
			return err
		}

		// Initial status history entry.
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, actor_role, note)
			VALUES ($1,'','pending_payment','customer','Order placed')`, orderID); err != nil {
			return err
		}

		// Clear only the lines that were actually ordered. With a partial
		// selection this leaves the unselected items in the cart; with no
		// selection (whole-cart checkout) it empties the cart as before.
		if err := s.cart.ClearSelectedTx(ctx, tx, cartID, in.SelectedListingIDs); err != nil {
			return err
		}

		// For COD there is no online payment step, so confirm the order within
		// the same transaction (atomic with order creation). Online orders stay
		// pending_payment until the payments module confirms capture.
		if in.PaymentMethod == "cod" {
			if _, err := tx.Exec(ctx,
				`UPDATE orders SET status='confirmed' WHERE id=$1`, orderID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO order_status_history (order_id, from_status, to_status, actor_role, note)
				VALUES ($1,'pending_payment','confirmed','system','COD confirmed')`, orderID); err != nil {
				return err
			}
		}

		var oErr error
		order, oErr = scanOrderTx(ctx, tx, orderID)
		return oErr
	})
	if err != nil {
		return nil, err
	}
	s.hook(ctx, order.ID, "order.created", in.UserID)
	if order.PaymentMethod == "cod" {
		s.hook(ctx, order.ID, "order.confirmed", in.UserID)
	}
	return order, nil
}

// ConfirmPayment marks an order paid/confirmed (called by payments module on
// successful capture, or immediately for COD).
func (s *Service) ConfirmPayment(ctx context.Context, orderID string) error {
	if _, err := s.repo.Pool().Exec(ctx,
		`UPDATE orders SET payment_status='paid' WHERE id=$1 AND payment_method='online'`, orderID); err != nil {
		return err
	}
	// Idempotent: both the gateway webhook and the client-side verify call race
	// to confirm the same payment. Whichever lands first moves the order to
	// "confirmed"; the second must be a no-op, not an error. Only transition
	// when the order is still in its initial state — if it's already confirmed
	// (or further along) we silently succeed so the other caller doesn't 500.
	var current Status
	if err := s.repo.Pool().QueryRow(ctx,
		`SELECT status FROM orders WHERE id=$1`, orderID).Scan(&current); err != nil {
		return err
	}
	if current != StatusConfirmed && CanTransition(current, StatusConfirmed) {
		if err := s.repo.UpdateOrderStatus(ctx, orderID, StatusConfirmed, "", "system", "Payment confirmed"); err != nil {
			// If a concurrent caller confirmed it between our read and write,
			// treat an invalid-transition as already-done rather than failing.
			var ite *InvalidTransitionError
			if asInvalid(err, &ite) {
				return nil
			}
			return err
		}
		s.hook(ctx, orderID, "order.confirmed", "")
	}
	return nil
}

// TransitionOrder applies a validated order-level status change.
func (s *Service) TransitionOrder(ctx context.Context, orderID string, to Status, actorID, actorRole, note string) error {
	err := s.repo.UpdateOrderStatus(ctx, orderID, to, actorID, actorRole, note)
	var ite *InvalidTransitionError
	if err == ErrNotFound {
		return httpx.NotFound("order_not_found", "Order not found")
	}
	if err != nil {
		if asInvalid(err, &ite) {
			return httpx.BadRequest("invalid_transition", ite.Error())
		}
		return err
	}
	s.hook(ctx, orderID, "order."+string(to), actorID)
	return nil
}

// Cancel cancels an order (customer or admin) if the state allows it, releasing
// reserved inventory.
func (s *Service) Cancel(ctx context.Context, orderID, actorID, actorRole, reason string) error {
	note := "Order cancelled"
	if r := strings.TrimSpace(reason); r != "" {
		note = "Order cancelled: " + r
	}
	return pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		var current Status
		if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id=$1 FOR UPDATE`, orderID).
			Scan(&current); err != nil {
			return httpx.NotFound("order_not_found", "Order not found")
		}
		if !CanTransition(current, StatusCancelled) {
			return httpx.BadRequest("not_cancellable", "This order can no longer be cancelled")
		}
		// Release reserved stock for each item.
		irows, err := tx.Query(ctx, `SELECT seller_listing_id, quantity FROM order_items WHERE order_id=$1`, orderID)
		if err != nil {
			return err
		}
		type ri struct {
			listing string
			qty     int
		}
		var items []ri
		for irows.Next() {
			var x ri
			if err := irows.Scan(&x.listing, &x.qty); err != nil {
				irows.Close()
				return err
			}
			items = append(items, x)
		}
		irows.Close()
		for _, x := range items {
			if err := s.inv.ReleaseTx(ctx, tx, x.listing, x.qty); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE orders SET status='cancelled' WHERE id=$1`, orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE order_items SET item_status='cancelled' WHERE order_id=$1`, orderID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, actor_role, note)
			VALUES ($1,$2,'cancelled',$3,$4)`, orderID, string(current), actorRole, note)
		return err
	})
}

// AbandonOnlineOrder is called when an online payment did not complete (the
// customer closed the Razorpay popup, or signature verification failed). It
// undoes the just-created order so the customer is never left with a stranded
// "pending payment" order and never loses their cart:
//
//  1. verifies the order belongs to the customer, is an online order, and has
//     not been paid yet (a paid order must never be abandoned),
//  2. cancels the order (status -> cancelled) and releases the reserved stock,
//  3. restores the order's items back into the customer's cart so they can
//     retry checkout immediately.
//
// Everything runs in one transaction, so either the order is fully unwound and
// the cart restored, or nothing changes.
func (s *Service) AbandonOnlineOrder(ctx context.Context, orderID, userID string) error {
	customerID, err := s.cart.CustomerID(ctx, userID)
	if err != nil {
		return err
	}
	cartID, err := s.cart.CartIDForCustomer(ctx, customerID)
	if err != nil {
		return httpx.NotFound("cart_not_found", "Cart not found")
	}

	return pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		// Lock and validate the order, scoped to this customer.
		var status Status
		var method, payStatus string
		err := tx.QueryRow(ctx, `
			SELECT status, payment_method, payment_status
			FROM orders WHERE id=$1 AND customer_id=$2 FOR UPDATE`, orderID, customerID).
			Scan(&status, &method, &payStatus)
		if err == pgx.ErrNoRows {
			return httpx.NotFound("order_not_found", "Order not found")
		}
		if err != nil {
			return err
		}
		if method != "online" {
			return httpx.BadRequest("not_online", "Only online orders can be abandoned")
		}
		if payStatus == "paid" {
			return httpx.Conflict("already_paid", "This order is already paid")
		}
		if !CanTransition(status, StatusCancelled) {
			// Already cancelled (or beyond) — nothing to undo, treat as success so
			// the client flow is idempotent.
			return nil
		}

		// Load the order's items (listing + quantity + snapshot price) to both
		// release stock and rebuild the cart.
		rows, err := tx.Query(ctx,
			`SELECT seller_listing_id, quantity, unit_price FROM order_items WHERE order_id=$1`, orderID)
		if err != nil {
			return err
		}
		type oi struct {
			listing string
			qty     int
			price   float64
		}
		var items []oi
		for rows.Next() {
			var x oi
			if err := rows.Scan(&x.listing, &x.qty, &x.price); err != nil {
				rows.Close()
				return err
			}
			items = append(items, x)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		// Release reserved stock and restore each line into the cart.
		for _, x := range items {
			if err := s.inv.ReleaseTx(ctx, tx, x.listing, x.qty); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO cart_items (cart_id, seller_listing_id, quantity, unit_price_snapshot)
				VALUES ($1,$2,$3,$4)
				ON CONFLICT (cart_id, seller_listing_id)
				DO UPDATE SET quantity = cart_items.quantity + EXCLUDED.quantity,
				              unit_price_snapshot = EXCLUDED.unit_price_snapshot`,
				cartID, x.listing, x.qty, x.price); err != nil {
				return err
			}
		}

		// Cancel the order + its items so it never shows as a stranded
		// "pending payment" order.
		if _, err := tx.Exec(ctx, `UPDATE orders SET status='cancelled' WHERE id=$1`, orderID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE order_items SET item_status='cancelled' WHERE order_id=$1`, orderID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, actor_role, note)
			VALUES ($1,$2,'cancelled','customer','Online payment not completed — order abandoned, cart restored')`,
			orderID, string(status))
		return err
	})
}

// ---- reads ----

// Get returns an order by ID without scoping — reserved for admin callers only.
func (s *Service) Get(ctx context.Context, id string) (*Order, error) {
	o, err := s.repo.GetByID(ctx, id)
	if err == ErrNotFound {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	return o, err
}

// GetForCustomerUser returns an order only if it belongs to the authenticated
// customer (resolved from the user ID). Prevents cross-customer IDOR.
func (s *Service) GetForCustomerUser(ctx context.Context, id, userID string) (*Order, error) {
	customerID, err := s.cart.CustomerID(ctx, userID)
	if err != nil {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	o, err := s.repo.GetByIDForCustomer(ctx, id, customerID)
	if err == ErrNotFound {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	return o, err
}

// GetForSeller returns an order limited to the seller's own items. Prevents a
// seller reading another seller's items/financials or unrelated orders.
func (s *Service) GetForSeller(ctx context.Context, id, sellerID string) (*Order, error) {
	o, err := s.repo.GetByIDForSeller(ctx, id, sellerID)
	if err == ErrNotFound {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	return o, err
}

// HistoryForCustomerUser returns status history only for the customer's order.
func (s *Service) HistoryForCustomerUser(ctx context.Context, id, userID string) ([]StatusEvent, error) {
	customerID, err := s.cart.CustomerID(ctx, userID)
	if err != nil {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	ok, err := s.repo.OrderOwnedByCustomer(ctx, id, customerID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	return s.repo.History(ctx, id)
}

// History returns status history unscoped — admin only.
func (s *Service) History(ctx context.Context, id string) ([]StatusEvent, error) {
	return s.repo.History(ctx, id)
}

// HistoryForSeller returns status history only if the order contains the
// seller's items (ownership-scoped).
func (s *Service) HistoryForSeller(ctx context.Context, id, sellerID string) ([]StatusEvent, error) {
	if _, err := s.repo.GetByIDForSeller(ctx, id, sellerID); err == ErrNotFound {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	} else if err != nil {
		return nil, err
	}
	return s.repo.History(ctx, id)
}

func (s *Service) ListForCustomerUser(ctx context.Context, userID string, limit, offset int) ([]Order, int64, error) {
	customerID, err := s.cart.CustomerID(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return s.repo.ListForCustomer(ctx, customerID, limit, offset)
}

func (s *Service) ListForSeller(ctx context.Context, sellerID, itemStatus string, limit, offset int) ([]Order, int64, error) {
	return s.repo.ListForSeller(ctx, sellerID, itemStatus, limit, offset)
}

// ListForSellerFiltered returns a seller's orders with status + date-range filters.
func (s *Service) ListForSellerFiltered(ctx context.Context, sellerID, itemStatus, from, to string, limit, offset int) ([]Order, int64, error) {
	return s.repo.ListForSellerFiltered(ctx, SellerOrderFilter{
		SellerID: sellerID, ItemStatus: itemStatus, DateFrom: from, DateTo: to, Limit: limit, Offset: offset,
	})
}

func (s *Service) ListAll(ctx context.Context, status string, limit, offset int) ([]Order, int64, error) {
	return s.repo.ListAll(ctx, status, limit, offset)
}

// UpdateItemStatus lets a seller advance one of their items.
func (s *Service) UpdateItemStatus(ctx context.Context, orderItemID, sellerID string, to Status) error {
	if err := s.repo.UpdateItemStatus(ctx, orderItemID, sellerID, to); err == ErrNotFound {
		return httpx.NotFound("item_not_found", "Order item not found")
	} else if err != nil {
		return err
	}
	return nil
}

// AcceptForSeller accepts a new order: the seller confirms they'll fulfil it, so
// their items move confirmed -> processing and the order status advances to
// 'processing'. Fires an "order confirmed / being prepared" email to the buyer.
func (s *Service) AcceptForSeller(ctx context.Context, orderID, sellerID string) error {
	err := pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx, `
			UPDATE order_items SET item_status='processing'
			WHERE order_id=$1 AND seller_id=$2 AND item_status IN ('confirmed','pending_payment')`,
			orderID, sellerID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.BadRequest("not_acceptable", "This order has no new items to accept")
		}
		// Advance the order shell too (best-effort; ignore if already ahead).
		_, _ = tx.Exec(ctx, `UPDATE orders SET status='processing' WHERE id=$1 AND status='confirmed'`, orderID)
		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, actor_role, note)
			VALUES ($1,'confirmed','processing','seller','Seller accepted the order')`, orderID)
		return err
	})
	if err != nil {
		return err
	}
	s.hook(ctx, orderID, "order.processing", sellerID)
	return nil
}

// CancelForSeller cancels the seller's part of an order with a reason, releasing
// the reserved stock for the seller's items. If the whole order belongs to this
// seller, the order is marked cancelled.
func (s *Service) CancelForSeller(ctx context.Context, orderID, sellerID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return httpx.BadRequest("reason_required", "A cancellation reason is required")
	}
	err := pgx.BeginFunc(ctx, s.repo.Pool(), func(tx pgx.Tx) error {
		// Load the seller's not-yet-shipped items in this order.
		rows, err := tx.Query(ctx, `
			SELECT id, seller_listing_id, quantity FROM order_items
			WHERE order_id=$1 AND seller_id=$2
			  AND item_status IN ('confirmed','processing','packed','ready_to_ship','pending_payment')`,
			orderID, sellerID)
		if err != nil {
			return err
		}
		type oi struct {
			id, listing string
			qty         int
		}
		var items []oi
		for rows.Next() {
			var x oi
			if err := rows.Scan(&x.id, &x.listing, &x.qty); err != nil {
				rows.Close()
				return err
			}
			items = append(items, x)
		}
		rows.Close()
		if len(items) == 0 {
			return httpx.BadRequest("not_cancellable", "These items can no longer be cancelled")
		}
		for _, x := range items {
			if err := s.inv.ReleaseTx(ctx, tx, x.listing, x.qty); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE order_items SET item_status='cancelled' WHERE id=$1`, x.id); err != nil {
				return err
			}
		}
		// If no active items remain across the whole order, cancel the order.
		var remaining int
		if err := tx.QueryRow(ctx, `
			SELECT count(*) FROM order_items
			WHERE order_id=$1 AND item_status NOT IN ('cancelled','refunded','return_received')`,
			orderID).Scan(&remaining); err != nil {
			return err
		}
		if remaining == 0 {
			_, _ = tx.Exec(ctx, `UPDATE orders SET status='cancelled' WHERE id=$1`, orderID)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, actor_role, note)
			VALUES ($1,'', 'cancelled','seller',$2)`, orderID, "Seller cancelled: "+reason)
		return err
	})
	if err != nil {
		return err
	}
	s.hook(ctx, orderID, "order.seller_cancelled", sellerID)
	return nil
}

// ---- helpers ----

type addressSnapshot struct {
	name, phone, line1, line2, city, state, pincode, country string
}

func (s *Service) loadAddress(ctx context.Context, customerID, addressID string) (*addressSnapshot, error) {
	var a addressSnapshot
	q := `SELECT contact_name, contact_phone, line1, line2, city, state, pincode, country
	      FROM customer_addresses WHERE customer_id=$1 AND `
	var row pgx.Row
	if addressID != "" {
		row = s.repo.Pool().QueryRow(ctx, q+`id=$2`, customerID, addressID)
	} else {
		row = s.repo.Pool().QueryRow(ctx, q+`is_default=TRUE ORDER BY created_at LIMIT 1`, customerID)
	}
	if err := row.Scan(&a.name, &a.phone, &a.line1, &a.line2, &a.city, &a.state, &a.pincode, &a.country); err != nil {
		return nil, httpx.BadRequest("no_address", "A delivery address is required")
	}
	return &a, nil
}

func scanOrderTx(ctx context.Context, tx pgx.Tx, orderID string) (*Order, error) {
	return scanOrder(tx.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id=$1`, orderID))
}

// nextOrderNumber returns the next sequential order number, e.g. CRL-00001.
// It draws from the order_number_seq sequence within the checkout transaction.
func nextOrderNumber(ctx context.Context, tx pgx.Tx) (string, error) {
	var n int64
	if err := tx.QueryRow(ctx, `SELECT nextval('order_number_seq')`).Scan(&n); err != nil {
		return "", err
	}
	return fmt.Sprintf("CRL-%05d", n), nil
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100.0
}

func asInvalid(err error, target **InvalidTransitionError) bool {
	if ite, ok := err.(*InvalidTransitionError); ok {
		*target = ite
		return true
	}
	return false
}
