package orders

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound indicates a missing order.
var ErrNotFound = errors.New("not found")

// Repository provides order persistence.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository builds an order Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Pool exposes the pool for the checkout transaction.
func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

const orderCols = `id, order_number, customer_id, status, payment_method, payment_status,
	currency, items_subtotal, discount_total, tax_total, shipping_total, grand_total,
	ship_contact_name, ship_contact_phone, ship_line1, ship_line2, ship_city, ship_state,
	ship_pincode, ship_country, placed_at, created_at`

func scanOrder(row pgx.Row) (*Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.OrderNumber, &o.CustomerID, &o.Status, &o.PaymentMethod,
		&o.PaymentStatus, &o.Currency, &o.ItemsSubtotal, &o.DiscountTotal, &o.TaxTotal,
		&o.ShippingTotal, &o.GrandTotal, &o.ShipName, &o.ShipPhone, &o.ShipLine1,
		&o.ShipLine2, &o.ShipCity, &o.ShipState, &o.ShipPincode, &o.ShipCountry,
		&o.PlacedAt, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &o, err
}

// GetByID returns an order with its items.
func (r *Repository) GetByID(ctx context.Context, id string) (*Order, error) {
	o, err := scanOrder(r.pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	items, err := r.loadItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

// GetByIDForCustomer returns an order only if it belongs to the given customer.
// Prevents one customer reading another customer's order (IDOR).
func (r *Repository) GetByIDForCustomer(ctx context.Context, id, customerID string) (*Order, error) {
	o, err := scanOrder(r.pool.QueryRow(ctx,
		`SELECT `+orderCols+` FROM orders WHERE id = $1 AND customer_id = $2`, id, customerID))
	if err != nil {
		return nil, err
	}
	items, err := r.loadItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

// GetByIDForSeller returns an order only if it contains at least one item sold
// by the given seller, and it strips items belonging to other sellers so a
// seller never sees another seller's line items or financials.
func (r *Repository) GetByIDForSeller(ctx context.Context, id, sellerID string) (*Order, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM order_items WHERE order_id = $1 AND seller_id = $2)`,
		id, sellerID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}
	o, err := scanOrder(r.pool.QueryRow(ctx, `SELECT `+orderCols+` FROM orders WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	items, err := r.loadItemsForSeller(ctx, o.ID, sellerID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

// OrderOwnedByCustomer reports whether an order belongs to the customer.
func (r *Repository) OrderOwnedByCustomer(ctx context.Context, id, customerID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM orders WHERE id = $1 AND customer_id = $2)`, id, customerID).Scan(&ok)
	return ok, err
}

func (r *Repository) loadItems(ctx context.Context, orderID string) ([]OrderItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT oi.id, oi.seller_id, s.store_name, oi.seller_listing_id, oi.product_title,
		       coalesce(p.slug,'') AS product_slug,
		       coalesce((SELECT file_key FROM product_images pi WHERE pi.product_id = p.id
		                 ORDER BY pi.is_primary DESC, pi.sort_order LIMIT 1),'') AS image,
		       oi.variant_label, oi.sku, oi.item_status, oi.delivered_at, oi.quantity, oi.unit_price,
		       oi.gross_amount, oi.discount_amount, oi.tax_amount, oi.shipping_charge,
		       oi.commission_percent, oi.commission_amount, oi.payment_fee, oi.refund_amount,
		       oi.seller_payable,
		       EXISTS (SELECT 1 FROM reviews rv WHERE rv.order_item_id = oi.id) AS has_review,
		       coalesce((SELECT ri.return_id::text FROM return_items ri WHERE ri.order_item_id = oi.id LIMIT 1),'') AS return_id
		FROM order_items oi
		JOIN sellers s ON s.id = oi.seller_id
		LEFT JOIN seller_listings sl ON sl.id = oi.seller_listing_id
		LEFT JOIN products p ON p.id = sl.product_id
		WHERE oi.order_id = $1
		ORDER BY oi.created_at`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrderItem
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ID, &it.SellerID, &it.SellerName, &it.SellerListingID,
			&it.ProductTitle, &it.ProductSlug, &it.Image, &it.VariantLabel, &it.SKU, &it.ItemStatus, &it.DeliveredAt, &it.Quantity,
			&it.UnitPrice, &it.GrossAmount, &it.DiscountAmount, &it.TaxAmount,
			&it.ShippingCharge, &it.CommissionPercent, &it.CommissionAmount,
			&it.PaymentFee, &it.RefundAmount, &it.SellerPayable,
			&it.HasReview, &it.ReturnID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// loadItemsForSeller loads only the items of an order that belong to a seller.
func (r *Repository) loadItemsForSeller(ctx context.Context, orderID, sellerID string) ([]OrderItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT oi.id, oi.seller_id, s.store_name, oi.seller_listing_id, oi.product_title,
		       oi.variant_label, oi.sku, oi.item_status, oi.quantity, oi.unit_price,
		       oi.gross_amount, oi.discount_amount, oi.tax_amount, oi.shipping_charge,
		       oi.commission_percent, oi.commission_amount, oi.payment_fee, oi.refund_amount,
		       oi.seller_payable
		FROM order_items oi
		JOIN sellers s ON s.id = oi.seller_id
		WHERE oi.order_id = $1 AND oi.seller_id = $2
		ORDER BY oi.created_at`, orderID, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrderItem
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.ID, &it.SellerID, &it.SellerName, &it.SellerListingID,
			&it.ProductTitle, &it.VariantLabel, &it.SKU, &it.ItemStatus, &it.Quantity,
			&it.UnitPrice, &it.GrossAmount, &it.DiscountAmount, &it.TaxAmount,
			&it.ShippingCharge, &it.CommissionPercent, &it.CommissionAmount,
			&it.PaymentFee, &it.RefundAmount, &it.SellerPayable); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ListForCustomer returns a customer's orders (paginated).
func (r *Repository) ListForCustomer(ctx context.Context, customerID string, limit, offset int) ([]Order, int64, error) {
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE customer_id = $1`, customerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx, `SELECT `+orderCols+`
		FROM orders WHERE customer_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		customerID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, total, err := collectOrders(rows, total)
	if err != nil {
		return nil, 0, err
	}
	// Attach each order's items (with a preview image) so the orders list can
	// show a product thumbnail/title — not just the order row.
	if err := r.attachItems(ctx, list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// attachItems loads the items for a page of orders in a single query (avoids
// N+1) and assigns them back to each order. The item query matches loadItems,
// including the primary-image lookup, so list cards render a thumbnail.
func (r *Repository) attachItems(ctx context.Context, list []Order) error {
	if len(list) == 0 {
		return nil
	}
	ids := make([]string, len(list))
	for i := range list {
		ids[i] = list[i].ID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT oi.order_id, oi.id, oi.seller_id, s.store_name, oi.seller_listing_id, oi.product_title,
		       coalesce(p.slug,'') AS product_slug,
		       coalesce((SELECT file_key FROM product_images pi WHERE pi.product_id = p.id
		                 ORDER BY pi.is_primary DESC, pi.sort_order LIMIT 1),'') AS image,
		       oi.variant_label, oi.sku, oi.item_status, oi.delivered_at, oi.quantity, oi.unit_price,
		       oi.gross_amount, oi.discount_amount, oi.tax_amount, oi.shipping_charge,
		       oi.commission_percent, oi.commission_amount, oi.payment_fee, oi.refund_amount,
		       oi.seller_payable,
		       EXISTS (SELECT 1 FROM reviews rv WHERE rv.order_item_id = oi.id) AS has_review,
		       coalesce((SELECT ri.return_id::text FROM return_items ri WHERE ri.order_item_id = oi.id LIMIT 1),'') AS return_id
		FROM order_items oi
		JOIN sellers s ON s.id = oi.seller_id
		LEFT JOIN seller_listings sl ON sl.id = oi.seller_listing_id
		LEFT JOIN products p ON p.id = sl.product_id
		WHERE oi.order_id = ANY($1)
		ORDER BY oi.order_id, oi.created_at`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	byOrder := make(map[string][]OrderItem, len(list))
	for rows.Next() {
		var orderID string
		var it OrderItem
		if err := rows.Scan(&orderID, &it.ID, &it.SellerID, &it.SellerName, &it.SellerListingID,
			&it.ProductTitle, &it.ProductSlug, &it.Image, &it.VariantLabel, &it.SKU, &it.ItemStatus, &it.DeliveredAt, &it.Quantity,
			&it.UnitPrice, &it.GrossAmount, &it.DiscountAmount, &it.TaxAmount,
			&it.ShippingCharge, &it.CommissionPercent, &it.CommissionAmount,
			&it.PaymentFee, &it.RefundAmount, &it.SellerPayable,
			&it.HasReview, &it.ReturnID); err != nil {
			return err
		}
		byOrder[orderID] = append(byOrder[orderID], it)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range list {
		list[i].Items = byOrder[list[i].ID]
	}
	return nil
}

// ListForSeller returns orders that include items sold by a seller.
func (r *Repository) ListForSeller(ctx context.Context, sellerID, itemStatus string, limit, offset int) ([]Order, int64, error) {
	return r.ListForSellerFiltered(ctx, SellerOrderFilter{
		SellerID: sellerID, ItemStatus: itemStatus, Limit: limit, Offset: offset,
	})
}

// SellerOrderFilter carries the optional filters for a seller's order list.
// DateFrom/DateTo are inclusive day bounds (RFC3339 or YYYY-MM-DD); empty means
// unbounded.
type SellerOrderFilter struct {
	SellerID   string
	ItemStatus string
	DateFrom   string
	DateTo     string
	Limit      int
	Offset     int
}

// ListForSellerFiltered returns a seller's orders with optional item-status and
// created-at date-range filters. All conditions are parameterized.
func (r *Repository) ListForSellerFiltered(ctx context.Context, f SellerOrderFilter) ([]Order, int64, error) {
	args := []any{f.SellerID}
	cond := ""
	if f.ItemStatus != "" {
		args = append(args, f.ItemStatus)
		cond += " AND oi.item_status = $" + itoa(len(args))
	}
	if f.DateFrom != "" {
		args = append(args, f.DateFrom)
		cond += " AND o.created_at >= $" + itoa(len(args))
	}
	if f.DateTo != "" {
		// Inclusive of the whole 'to' day.
		args = append(args, f.DateTo)
		cond += " AND o.created_at < ($" + itoa(len(args)) + "::date + INTERVAL '1 day')"
	}
	var total int64
	if err := r.pool.QueryRow(ctx,
		`SELECT count(DISTINCT o.id) FROM orders o
		 JOIN order_items oi ON oi.order_id = o.id
		 WHERE oi.seller_id = $1`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	q := `SELECT DISTINCT ` + prefixCols("o") + `
		FROM orders o JOIN order_items oi ON oi.order_id = o.id
		WHERE oi.seller_id = $1` + cond +
		` ORDER BY o.created_at DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return collectOrders(rows, total)
}

// ListAll returns all orders for admin (paginated, optional status filter).
func (r *Repository) ListAll(ctx context.Context, status string, limit, offset int) ([]Order, int64, error) {
	args := []any{}
	where := ""
	if status != "" {
		args = append(args, status)
		where = "WHERE status = $1"
	}
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM orders `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, limit, offset)
	q := `SELECT ` + orderCols + ` FROM orders ` + where +
		` ORDER BY created_at DESC LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return collectOrders(rows, total)
}

// History returns the status history of an order.
func (r *Repository) History(ctx context.Context, orderID string) ([]StatusEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT from_status, to_status, actor_role, note, created_at
		FROM order_status_history WHERE order_id = $1 ORDER BY created_at`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StatusEvent
	for rows.Next() {
		var e StatusEvent
		if err := rows.Scan(&e.FromStatus, &e.ToStatus, &e.ActorRole, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateOrderStatus validates and applies an order-level status transition,
// recording the change in order_status_history within a transaction.
func (r *Repository) UpdateOrderStatus(ctx context.Context, orderID string, to Status, actorID, actorRole, note string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		var current Status
		if err := tx.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1 FOR UPDATE`, orderID).
			Scan(&current); errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if !CanTransition(current, to) {
			return &InvalidTransitionError{From: string(current), To: string(to)}
		}
		if _, err := tx.Exec(ctx, `UPDATE orders SET status = $2 WHERE id = $1`, orderID, string(to)); err != nil {
			return err
		}
		var actor *string
		if actorID != "" {
			actor = &actorID
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO order_status_history (order_id, from_status, to_status, changed_by, actor_role, note)
			VALUES ($1,$2,$3,$4,$5,$6)`,
			orderID, string(current), string(to), actor, actorRole, note)
		return err
	})
}

// UpdateItemStatus updates a single order item's status (seller fulfilment).
func (r *Repository) UpdateItemStatus(ctx context.Context, orderItemID, sellerID string, to Status) error {
	ct, err := r.pool.Exec(ctx,
		`UPDATE order_items SET item_status = $3 WHERE id = $1 AND seller_id = $2`,
		orderItemID, sellerID, string(to))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// InvalidTransitionError is returned when a status change is not allowed.
type InvalidTransitionError struct{ From, To string }

func (e *InvalidTransitionError) Error() string {
	return "invalid status transition: " + e.From + " -> " + e.To
}

func collectOrders(rows pgx.Rows, total int64) ([]Order, int64, error) {
	var out []Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *o)
	}
	return out, total, rows.Err()
}

func prefixCols(p string) string {
	cols := []string{"id", "order_number", "customer_id", "status", "payment_method",
		"payment_status", "currency", "items_subtotal", "discount_total", "tax_total",
		"shipping_total", "grand_total", "ship_contact_name", "ship_contact_phone",
		"ship_line1", "ship_line2", "ship_city", "ship_state", "ship_pincode",
		"ship_country", "placed_at", "created_at"}
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += p + "." + c
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
