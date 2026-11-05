// Package fulfillment ties orders to the courier provider: sellers create
// shipments, generate labels/AWBs, schedule pickups and view tracking; the
// shipment lifecycle is persisted and mirrored onto the order items.
package fulfillment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/carlitos/backend/internal/httpx"
	"github.com/carlitos/backend/internal/labelpdf"
	"github.com/carlitos/backend/internal/shipping"
)

// Shipment view model.
type Shipment struct {
	ID                 string `json:"id"`
	OrderID            string `json:"order_id"`
	OrderNumber        string `json:"order_number,omitempty"`
	SellerID           string `json:"seller_id"`
	Provider           string `json:"provider"`
	ShipType           string `json:"ship_type"`
	AWBNumber          string `json:"awb_number"`
	CourierName        string `json:"courier_name"`
	LabelURL           string `json:"label_url"`
	Status             string `json:"status"`
	Direction          string `json:"direction"`
	TrackingURL        string `json:"tracking_url"`
	ProviderShipmentID string `json:"provider_shipment_id,omitempty"`

	// Context shown on the Shipments / Shipping pages.
	StoreName         string     `json:"store_name"`
	CreatedAt         time.Time  `json:"created_at"`
	PickupScheduledAt *time.Time `json:"pickup_scheduled_at"`
	ShippedAt         *time.Time `json:"shipped_at"`
	DeliveredAt       *time.Time `json:"delivered_at"`
	FreightCharge     float64    `json:"freight_charge"`
	CustomerName      string     `json:"customer_name"`
	ShipCity          string     `json:"ship_city"`
	ShipState         string     `json:"ship_state"`
	ShipPincode       string     `json:"ship_pincode"`
	PaymentMethod     string     `json:"payment_method"`
	// CODAmount is what the courier/seller must collect (0 for prepaid).
	CODAmount float64 `json:"cod_amount"`
	ItemCount int     `json:"item_count"`
	ItemTitle string  `json:"item_title"`
	// Attention is a short reason when the shipment looks stuck or failed.
	Attention string `json:"attention"`
}

// StatusNotifier lets fulfillment inform the customer of shipment status
// changes without importing the notification package (avoids a cycle).
type StatusNotifier func(ctx context.Context, orderID, headline, message, courier, awb string)

// Repository provides shipment persistence.
type Repository struct{ pool *pgxpool.Pool }

// NewRepository builds a fulfillment Repository.
func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// Service composes the courier provider with persistence.
type Service struct {
	repo     *Repository
	courier  shipping.Provider
	notify   StatusNotifier
	baseURL  string // storefront base URL for self-ship labels
}

// NewService builds a fulfillment Service.
func NewService(repo *Repository, courier shipping.Provider) *Service {
	return &Service{repo: repo, courier: courier, notify: func(context.Context, string, string, string, string, string) {}}
}

// WithNotifier attaches a customer status notifier.
func (s *Service) WithNotifier(n StatusNotifier) *Service {
	if n != nil {
		s.notify = n
	}
	return s
}

// WithBaseURL sets the storefront base URL (used in self-ship label links).
func (s *Service) WithBaseURL(u string) *Service {
	s.baseURL = u
	return s
}

// CreateForOrder creates a forward shipment for a seller's items in an order.
// It calls the courier provider to obtain an AWB + label, persists the shipment
// and marks the seller's order items as ready_to_ship.
func (s *Service) CreateForOrder(ctx context.Context, orderID, sellerID string) (*Shipment, error) {
	// Load order delivery address + seller pickup + weight.
	var del shipping.Address
	var orderNumber string
	err := s.repo.pool.QueryRow(ctx, `
		SELECT order_number, ship_contact_name, ship_contact_phone, ship_line1, ship_line2,
		       ship_city, ship_state, ship_pincode, ship_country
		FROM orders WHERE id = $1`, orderID).
		Scan(&orderNumber, &del.Name, &del.Phone, &del.Line1, &del.Line2, &del.City,
			&del.State, &del.Pincode, &del.Country)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	if err != nil {
		return nil, err
	}

	// Verify the seller actually has items in this order.
	var itemCount int
	if err := s.repo.pool.QueryRow(ctx,
		`SELECT count(*) FROM order_items WHERE order_id=$1 AND seller_id=$2`,
		orderID, sellerID).Scan(&itemCount); err != nil {
		return nil, err
	}
	if itemCount == 0 {
		return nil, httpx.Forbidden("not_your_order", "This order has no items for your store")
	}

	pickup := s.loadPickup(ctx, sellerID)

	// Each seller ships from their OWN pickup location. We register the seller's
	// warehouse with the courier under a deterministic nickname (idempotent), so
	// the courier collects from the correct address. Sellers never touch this.
	nickname := pickupNickname(sellerID)
	var sellerEmail string
	_ = s.repo.pool.QueryRow(ctx, `SELECT coalesce(contact_email,'') FROM sellers WHERE id=$1`, sellerID).Scan(&sellerEmail)
	if reg, rerr := s.courier.RegisterPickupLocation(ctx, shipping.PickupLocationInput{
		Nickname: nickname, Address: pickup, Email: sellerEmail,
	}); rerr == nil && reg != "" {
		nickname = reg
	}

	// Real line items for this seller's part of the order (name/sku/qty/price).
	shipItems, codAmount := s.loadShipItems(ctx, orderID, sellerID)
	var isCOD bool
	_ = s.repo.pool.QueryRow(ctx, `SELECT payment_method='cod' FROM orders WHERE id=$1`, orderID).Scan(&isCOD)

	created, err := s.courier.CreateShipment(ctx, shipping.CreateShipmentInput{
		OrderNumber:    orderNumber,
		Pickup:         pickup,
		Delivery:       del,
		WeightGrams:    estimateWeight(shipItems),
		COD:            isCOD,
		CODAmount:      codAmount,
		PickupLocation: nickname,
		Items:          shipItems,
	})
	if err != nil {
		return nil, httpx.Internal("courier error: " + err.Error())
	}

	var shipmentID string
	err = pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		freight := float64(created.FreightMinor) / 100.0
		if err := tx.QueryRow(ctx, `
			INSERT INTO shipments (order_id, seller_id, provider, ship_type, awb_number, courier_name,
				label_key, status, direction, tracking_url, provider_shipment_id, freight_charge)
			VALUES ($1,$2,$3,'courier',$4,$5,$6,'label_generated','forward',$7,$8,$9)
			RETURNING id`,
			orderID, sellerID, s.courier.Name(), created.AWBNumber, created.Courier,
			created.LabelURL, created.TrackingURL, created.ProviderShipmentID, freight).Scan(&shipmentID); err != nil {
			return err
		}
		// Link the seller's order items to this shipment + mark ready_to_ship.
		rows, err := tx.Query(ctx,
			`SELECT id, quantity FROM order_items WHERE order_id=$1 AND seller_id=$2`, orderID, sellerID)
		if err != nil {
			return err
		}
		type oi struct {
			id  string
			qty int
		}
		var items []oi
		for rows.Next() {
			var x oi
			if err := rows.Scan(&x.id, &x.qty); err != nil {
				rows.Close()
				return err
			}
			items = append(items, x)
		}
		rows.Close()
		// Spread the actual courier freight across this seller's items so the
		// seller settlement later deducts the real delivery cost (admin fronts
		// it to the courier, recovers it from the seller's remittance).
		var perItem, remainder float64
		if len(items) > 0 && freight > 0 {
			perItem = float64(int((freight/float64(len(items)))*100)) / 100.0
			remainder = freight - perItem*float64(len(items))
		}
		for i, x := range items {
			if _, err := tx.Exec(ctx,
				`INSERT INTO shipment_items (shipment_id, order_item_id, quantity) VALUES ($1,$2,$3)`,
				shipmentID, x.id, x.qty); err != nil {
				return err
			}
			charge := perItem
			if i == len(items)-1 {
				charge += remainder // put any rounding remainder on the last item
			}
			if _, err := tx.Exec(ctx,
				`UPDATE order_items SET item_status='ready_to_ship', shipping_charge=$2 WHERE id=$1`,
				x.id, charge); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sh, err := s.Get(ctx, shipmentID)
	if err == nil && sh != nil {
		s.addEvent(ctx, s.repo.pool, shipmentID, "label_generated", "",
			"Shipment booked with "+nonEmptyStr(sh.CourierName, "the courier")+" · AWB "+sh.AWBNumber)
		// Best-effort: request a courier pickup right away.
		if sh.ProviderShipmentID != "" {
			_ = s.courier.SchedulePickup(ctx, sh.ProviderShipmentID, time.Now().Add(24*time.Hour))
			_, _ = s.repo.pool.Exec(ctx,
				`UPDATE shipments SET status='pickup_scheduled', pickup_scheduled_at=now() WHERE id=$1`, shipmentID)
			s.addEvent(ctx, s.repo.pool, shipmentID, "pickup_scheduled", "", "Courier pickup requested")
		}
		// Notify the customer their order is being shipped.
		go s.notify(context.Background(), orderID, "Your order has shipped",
			"your order is on its way", sh.CourierName, sh.AWBNumber)
	}
	return s.Get(ctx, shipmentID)
}

// SelfShip records a seller-fulfilled (self-delivery) shipment. No courier is
// contacted; the seller prints the CARLITOS self-ship label and delivers the
// order themselves, then marks it delivered. Returns the created shipment.
func (s *Service) SelfShip(ctx context.Context, orderID, sellerID string) (*Shipment, error) {
	var orderNumber string
	err := s.repo.pool.QueryRow(ctx, `SELECT order_number FROM orders WHERE id=$1`, orderID).Scan(&orderNumber)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("order_not_found", "Order not found")
	}
	if err != nil {
		return nil, err
	}
	var itemCount int
	if err := s.repo.pool.QueryRow(ctx,
		`SELECT count(*) FROM order_items WHERE order_id=$1 AND seller_id=$2`,
		orderID, sellerID).Scan(&itemCount); err != nil {
		return nil, err
	}
	if itemCount == 0 {
		return nil, httpx.Forbidden("not_your_order", "This order has no items for your store")
	}

	// A synthetic AWB so the customer can still see a reference and track state.
	awb := "SELF" + uuidShort()
	var shipmentID string
	err = pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `
			INSERT INTO shipments (order_id, seller_id, provider, ship_type, awb_number, courier_name,
				status, direction, tracking_url)
			VALUES ($1,$2,'self','self',$3,'Self Ship','pickup_scheduled','forward','')
			RETURNING id`,
			orderID, sellerID, awb).Scan(&shipmentID); err != nil {
			return err
		}
		rows, err := tx.Query(ctx,
			`SELECT id, quantity FROM order_items WHERE order_id=$1 AND seller_id=$2`, orderID, sellerID)
		if err != nil {
			return err
		}
		type oi struct {
			id  string
			qty int
		}
		var items []oi
		for rows.Next() {
			var x oi
			if err := rows.Scan(&x.id, &x.qty); err != nil {
				rows.Close()
				return err
			}
			items = append(items, x)
		}
		rows.Close()
		for _, x := range items {
			if _, err := tx.Exec(ctx,
				`INSERT INTO shipment_items (shipment_id, order_item_id, quantity) VALUES ($1,$2,$3)`,
				shipmentID, x.id, x.qty); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE order_items SET item_status='ready_to_ship' WHERE id=$1`, x.id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.addEvent(ctx, s.repo.pool, shipmentID, "pickup_scheduled", "",
		"Self-ship created · print the label, pack and deliver it yourself")
	return s.Get(ctx, shipmentID)
}

// SelfShipLabelData gathers the fields for a self-ship label PDF, verifying the
// shipment belongs to the seller.
func (s *Service) SelfShipLabelData(ctx context.Context, shipmentID, sellerID string) (*labelpdf.Label, error) {
	var (
		orderID, orderNumber, awb, shipType string
		toName, toPhone, l1, l2, city, state, pin string
	)
	err := s.repo.pool.QueryRow(ctx, `
		SELECT s.order_id, o.order_number, coalesce(s.awb_number,''), coalesce(s.ship_type,'courier'),
		       o.ship_contact_name, o.ship_contact_phone, o.ship_line1, o.ship_line2,
		       o.ship_city, o.ship_state, o.ship_pincode
		FROM shipments s JOIN orders o ON o.id = s.order_id
		WHERE s.id=$1 AND s.seller_id=$2`, shipmentID, sellerID).
		Scan(&orderID, &orderNumber, &awb, &shipType, &toName, &toPhone, &l1, &l2, &city, &state, &pin)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	if err != nil {
		return nil, err
	}

	// Seller store + pickup address.
	var store string
	_ = s.repo.pool.QueryRow(ctx, `SELECT coalesce(store_name,'') FROM sellers WHERE id=$1`, sellerID).Scan(&store)
	pickup := s.loadPickup(ctx, sellerID)
	fromLines := compact([]string{
		pickup.Line1, pickup.Line2,
		joinNonEmpty(", ", pickup.City, pickup.State, pickup.Pincode),
		phoneLine(pickup.Phone),
	})

	toLines := compact([]string{
		l1, l2, joinNonEmpty(", ", city, state, pin),
	})

	// Item summary for this seller.
	var items []string
	rows, err := s.repo.pool.Query(ctx, `
		SELECT oi.product_title, oi.quantity FROM order_items oi
		JOIN shipment_items si ON si.order_item_id = oi.id
		WHERE si.shipment_id=$1`, shipmentID)
	if err == nil {
		for rows.Next() {
			var title string
			var qty int
			if rows.Scan(&title, &qty) == nil {
				items = append(items, fmt.Sprintf("%s x%d", title, qty))
			}
		}
		rows.Close()
	}

	return &labelpdf.Label{
		OrderNumber: orderNumber,
		AWB:         awb,
		StoreName:   store,
		FromLines:   fromLines,
		ToName:      toName,
		ToPhone:     toPhone,
		ToLines:     toLines,
		Items:       items,
	}, nil
}

// MarkShipped transitions a shipment (and its items) to shipped, and notifies
// the customer.
func (s *Service) MarkShipped(ctx context.Context, shipmentID, sellerID string) error {
	// Only a shipment that is still waiting to leave can be marked as shipped.
	if err := s.transition(ctx, shipmentID, sellerID, "in_transit", "shipped",
		[]string{"created", "label_generated", "pickup_scheduled"}); err != nil {
		return err
	}
	s.addEvent(ctx, s.repo.pool, shipmentID, "in_transit", "", "Marked as shipped by the seller")
	sh, err := s.Get(ctx, shipmentID)
	if err == nil && sh != nil {
		go s.notify(context.Background(), sh.OrderID, "Your order has shipped",
			"your order is on its way", sh.CourierName, sh.AWBNumber)
	}
	return nil
}

// MarkDelivered transitions a (self-ship or courier) shipment to delivered and
// notifies the customer. Sellers use this to close out self-shipped orders.
func (s *Service) MarkDelivered(ctx context.Context, shipmentID, sellerID string) error {
	if err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE shipments SET status='delivered', delivered_at=now(), shipped_at=COALESCE(shipped_at, now())
			 WHERE id=$1 AND seller_id=$2 AND status NOT IN ('delivered','cancelled','rto')`,
			shipmentID, sellerID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return s.explainBlocked(ctx, shipmentID, sellerID)
		}
		_, err = tx.Exec(ctx, `
			UPDATE order_items SET item_status='delivered', delivered_at=now()
			WHERE id IN (SELECT order_item_id FROM shipment_items WHERE shipment_id=$1)`, shipmentID)
		return err
	}); err != nil {
		return err
	}
	s.addEvent(ctx, s.repo.pool, shipmentID, "delivered", "", "Marked as delivered by the seller")
	sh, err := s.Get(ctx, shipmentID)
	if err == nil && sh != nil {
		go s.notify(context.Background(), sh.OrderID, "Your order was delivered",
			"your order has been delivered", sh.CourierName, sh.AWBNumber)
	}
	return nil
}

// Track returns live tracking events from the courier for a shipment.
func (s *Service) Track(ctx context.Context, shipmentID, sellerID string) ([]shipping.TrackingEvent, error) {
	var awb string
	// Scoped to the calling seller: one seller must not read another's shipment.
	if err := s.repo.pool.QueryRow(ctx,
		`SELECT coalesce(awb_number,'') FROM shipments WHERE id=$1 AND seller_id=$2`, shipmentID, sellerID).
		Scan(&awb); err != nil {
		return nil, httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	events, err := s.courier.Track(ctx, awb)
	if err != nil {
		return nil, httpx.Internal("tracking error")
	}
	// Persist events for history (best-effort, never duplicated).
	for _, e := range events {
		s.addEventAt(ctx, s.repo.pool, shipmentID, e.Status, e.Location, e.Description, e.EventTime)
	}
	return events, nil
}

// TrackEvent is a persisted tracking timeline entry returned to the customer.
type TrackEvent struct {
	Status      string    `json:"status"`
	Location    string    `json:"location"`
	Description string    `json:"description"`
	EventTime   time.Time `json:"event_time"`
}

// CustomerTracking is the AWB-less tracking payload a customer sees for an order.
type CustomerTracking struct {
	ShipType    string       `json:"ship_type"`
	Courier     string       `json:"courier"`
	AWB         string       `json:"awb"`
	Status      string       `json:"status"`
	TrackingURL string       `json:"tracking_url"`
	Events      []TrackEvent `json:"events"`
}

// TrackForCustomerUser resolves the shipment for a customer's own order (no AWB
// entry needed) and returns the tracking timeline. It verifies the order
// belongs to the requesting user, refreshes courier events best-effort, and
// returns the persisted timeline newest-first.
func (s *Service) TrackForCustomerUser(ctx context.Context, orderID, userID string) (*CustomerTracking, error) {
	var (
		shipmentID, shipType, awb, courier, status, trackingURL string
	)
	err := s.repo.pool.QueryRow(ctx, `
		SELECT sh.id, coalesce(sh.ship_type,'courier'), coalesce(sh.awb_number,''),
		       sh.courier_name, sh.status, sh.tracking_url
		FROM shipments sh
		JOIN orders o ON o.id = sh.order_id
		JOIN customers c ON c.id = o.customer_id
		WHERE sh.order_id = $1 AND c.user_id = $2 AND sh.status <> 'cancelled'
		ORDER BY sh.created_at DESC
		LIMIT 1`, orderID, userID).
		Scan(&shipmentID, &shipType, &awb, &courier, &status, &trackingURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("not_shipped", "This order has not shipped yet")
	}
	if err != nil {
		return nil, err
	}

	// For courier shipments, refresh from the courier best-effort (webhook is
	// the primary source, this is a fallback for on-demand views).
	if shipType == "courier" && awb != "" {
		if evs, terr := s.courier.Track(ctx, awb); terr == nil {
			for _, e := range evs {
				s.addEventAt(ctx, s.repo.pool, shipmentID, e.Status, e.Location, e.Description, e.EventTime)
			}
		}
	}

	events, err := s.timeline(ctx, shipmentID)
	if err != nil {
		return nil, err
	}
	return &CustomerTracking{
		ShipType: shipType, Courier: courier, AWB: awb, Status: status,
		TrackingURL: trackingURL, Events: events,
	}, nil
}

func (s *Service) timeline(ctx context.Context, shipmentID string) ([]TrackEvent, error) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT status, location, description, event_time
		FROM shipment_tracking_events WHERE shipment_id=$1
		ORDER BY event_time DESC, created_at DESC`, shipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrackEvent
	for rows.Next() {
		var e TrackEvent
		if err := rows.Scan(&e.Status, &e.Location, &e.Description, &e.EventTime); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// HandleWebhook processes a courier tracking webhook: it maps the AWB to a
// shipment, appends the tracking event, updates the shipment + order-item
// status, and notifies the customer on key transitions (shipped, out for
// delivery, delivered). Idempotent: duplicate terminal transitions are ignored.
func (s *Service) HandleWebhook(ctx context.Context, awb, status, location, description string) error {
	if awb == "" {
		return nil
	}
	canonical := courierStatusToShipment(shipping.NormalizeStatus(status))

	var shipmentID, orderID, courier, currentStatus string
	err := s.repo.pool.QueryRow(ctx, `
		SELECT id, order_id, courier_name, status FROM shipments WHERE awb_number=$1
		ORDER BY created_at DESC LIMIT 1`, awb).
		Scan(&shipmentID, &orderID, &courier, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // unknown AWB — ack silently
	}
	if err != nil {
		return err
	}

	// Append the tracking event (deduplicated, so courier retries are harmless).
	s.addEvent(ctx, s.repo.pool, shipmentID, canonical, location, nonEmptyStr(description, status))

	s.applyStatus(ctx, shipmentID, orderID, courier, awb, currentStatus, canonical)
	return nil
}

// courierStatusToShipment maps the courier's canonical status to one allowed by
// the shipments.status CHECK constraint ("returned" is stored as "rto").
func courierStatusToShipment(canonical string) string {
	if canonical == "returned" {
		return "rto"
	}
	return canonical
}

// statusRank orders the forward journey of a shipment.
var statusRank = map[string]int{
	"created": 0, "label_generated": 1, "pickup_scheduled": 2,
	"picked_up": 3, "in_transit": 4, "out_for_delivery": 5, "delivered": 6,
}

// canAdvance reports whether a courier-reported status may replace the current
// one. Delivered / RTO / cancelled are final, and a shipment never moves
// backwards (webhooks can arrive late or out of order).
func canAdvance(current, next string) bool {
	if current == next {
		return false
	}
	switch current {
	case "delivered", "rto", "cancelled":
		return false
	}
	switch next {
	case "rto", "failed", "cancelled":
		return true
	}
	return statusRank[next] > statusRank[current]
}

// applyStatus moves a shipment (and its order items) to a courier-reported
// status and notifies the customer on the key milestones. It is shared by the
// courier webhook and the manual "refresh tracking" action.
func (s *Service) applyStatus(ctx context.Context, shipmentID, orderID, courier, awb, current, canonical string) {
	if !canAdvance(current, canonical) {
		return
	}
	itemStatus := ""
	var headline, message string
	switch canonical {
	case "picked_up", "in_transit":
		itemStatus = "shipped"
		if current != "in_transit" && current != "picked_up" && current != "out_for_delivery" {
			headline, message = "Your order has shipped", "your order is on its way"
		}
	case "out_for_delivery":
		itemStatus = "out_for_delivery"
		headline, message = "Out for delivery", "your order is out for delivery today"
	case "delivered":
		itemStatus = "delivered"
		headline, message = "Delivered", "your order has been delivered"
	case "rto":
		itemStatus = "return_received"
	case "cancelled":
		itemStatus = "cancelled"
	}

	err := pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			UPDATE shipments SET status=$2,
			    shipped_at = CASE WHEN $2 IN ('picked_up','in_transit','out_for_delivery') AND shipped_at IS NULL
			                      THEN now() ELSE shipped_at END,
			    delivered_at = CASE WHEN $2 = 'delivered' THEN now() ELSE delivered_at END
			WHERE id=$1`, shipmentID, canonical); err != nil {
			return err
		}
		if itemStatus != "" {
			if _, err := tx.Exec(ctx, `
				UPDATE order_items SET item_status=$2,
				       delivered_at = CASE WHEN $2 = 'delivered' AND delivered_at IS NULL THEN now() ELSE delivered_at END
				WHERE id IN (SELECT order_item_id FROM shipment_items WHERE shipment_id=$1)`,
				shipmentID, itemStatus); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return // keep the old status rather than notifying about a failed update
	}
	if headline != "" {
		go s.notify(context.Background(), orderID, headline, message, courier, awb)
	}
}

// shipmentSelect is the shared SELECT (columns + joins) for shipment rows. The
// matching scan is scanShipment.
const shipmentSelect = `
	SELECT s.id, s.order_id, coalesce(o.order_number,''), s.seller_id, s.provider,
	       coalesce(s.ship_type,'courier'), coalesce(s.awb_number,''), s.courier_name,
	       coalesce(s.label_key,''), s.status, s.direction, s.tracking_url,
	       coalesce(s.provider_shipment_id,''),
	       coalesce(sl.store_name,''), s.created_at, s.pickup_scheduled_at, s.shipped_at,
	       s.delivered_at, s.freight_charge,
	       o.ship_contact_name, o.ship_city, o.ship_state, o.ship_pincode, o.payment_method,
	       coalesce(it.qty,0), coalesce(it.gross,0), coalesce(it.title,'')
	FROM shipments s
	JOIN orders o ON o.id = s.order_id
	LEFT JOIN sellers sl ON sl.id = s.seller_id
	LEFT JOIN LATERAL (
		SELECT sum(si.quantity)::int AS qty, sum(oi.gross_amount)::float8 AS gross,
		       min(oi.product_title) AS title
		FROM shipment_items si JOIN order_items oi ON oi.id = si.order_item_id
		WHERE si.shipment_id = s.id
	) it ON TRUE`

func scanShipment(row pgx.Row) (Shipment, error) {
	var s Shipment
	var gross float64
	err := row.Scan(&s.ID, &s.OrderID, &s.OrderNumber, &s.SellerID, &s.Provider,
		&s.ShipType, &s.AWBNumber, &s.CourierName, &s.LabelURL, &s.Status, &s.Direction,
		&s.TrackingURL, &s.ProviderShipmentID,
		&s.StoreName, &s.CreatedAt, &s.PickupScheduledAt, &s.ShippedAt, &s.DeliveredAt, &s.FreightCharge,
		&s.CustomerName, &s.ShipCity, &s.ShipState, &s.ShipPincode, &s.PaymentMethod,
		&s.ItemCount, &gross, &s.ItemTitle)
	if err != nil {
		return s, err
	}
	if s.PaymentMethod == "cod" {
		s.CODAmount = gross
	}
	s.Attention = attentionReason(s.Status, s.ShipType, s.CreatedAt, s.ShippedAt, time.Now())
	return s, nil
}

// ListForSeller returns a seller's shipments (newest first).
func (s *Service) ListForSeller(ctx context.Context, sellerID string) ([]Shipment, error) {
	rows, err := s.repo.pool.Query(ctx, shipmentSelect+`
		WHERE s.seller_id=$1 ORDER BY s.created_at DESC`, sellerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collect(rows)
}

// Get returns a single shipment.
func (s *Service) Get(ctx context.Context, id string) (*Shipment, error) {
	rows, err := s.repo.pool.Query(ctx, shipmentSelect+` WHERE s.id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := collect(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	return &list[0], nil
}

// transition moves a seller's shipment (and its items) to a new status. The
// UPDATE itself is guarded by the allowed current statuses, so two quick clicks
// (or a click racing a courier webhook) can't apply a stale transition.
func (s *Service) transition(ctx context.Context, shipmentID, sellerID, shipStatus, itemStatus string, from []string) error {
	return pgx.BeginFunc(ctx, s.repo.pool, func(tx pgx.Tx) error {
		ct, err := tx.Exec(ctx,
			`UPDATE shipments SET status=$3, shipped_at=COALESCE(shipped_at, now())
			 WHERE id=$1 AND seller_id=$2 AND status = ANY($4)`,
			shipmentID, sellerID, shipStatus, from)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return s.explainBlocked(ctx, shipmentID, sellerID)
		}
		_, err = tx.Exec(ctx, `
			UPDATE order_items SET item_status=$2,
			       delivered_at = CASE WHEN $2 = 'delivered' AND delivered_at IS NULL THEN now() ELSE delivered_at END
			WHERE id IN (SELECT order_item_id FROM shipment_items WHERE shipment_id=$1)`,
			shipmentID, itemStatus)
		return err
	})
}

// explainBlocked turns "0 rows updated" into the right HTTP error: the shipment
// doesn't exist for this seller, or it is in a state that doesn't allow the move.
func (s *Service) explainBlocked(ctx context.Context, shipmentID, sellerID string) error {
	var status string
	err := s.repo.pool.QueryRow(ctx,
		`SELECT status FROM shipments WHERE id=$1 AND seller_id=$2`, shipmentID, sellerID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return httpx.NotFound("shipment_not_found", "Shipment not found")
	}
	if err != nil {
		return err
	}
	return httpx.Conflict("invalid_state", fmt.Sprintf(
		"This shipment is already %s, so that action isn't available.", strings.ReplaceAll(status, "_", " ")))
}

func (s *Service) loadPickup(ctx context.Context, sellerID string) shipping.Address {
	var a shipping.Address
	_ = s.repo.pool.QueryRow(ctx, `
		SELECT contact_name, contact_phone, line1, line2, city, state, pincode, country
		FROM seller_pickup_addresses WHERE seller_id=$1 AND is_default=TRUE
		ORDER BY created_at LIMIT 1`, sellerID).
		Scan(&a.Name, &a.Phone, &a.Line1, &a.Line2, &a.City, &a.State, &a.Pincode, &a.Country)
	return a
}

// loadShipItems returns the seller's order items as courier shipment lines and
// the COD-collectable amount (sum of gross for this seller's items).
func (s *Service) loadShipItems(ctx context.Context, orderID, sellerID string) ([]shipping.ShipmentItem, float64) {
	rows, err := s.repo.pool.Query(ctx, `
		SELECT product_title, coalesce(sku,''), quantity, unit_price, gross_amount
		FROM order_items WHERE order_id=$1 AND seller_id=$2`, orderID, sellerID)
	if err != nil {
		return nil, 0
	}
	defer rows.Close()
	var items []shipping.ShipmentItem
	var cod float64
	for rows.Next() {
		var title, sku string
		var qty int
		var unit, gross float64
		if err := rows.Scan(&title, &sku, &qty, &unit, &gross); err != nil {
			continue
		}
		items = append(items, shipping.ShipmentItem{Name: title, SKU: sku, Qty: qty, Price: unit})
		cod += gross
	}
	return items, cod
}

// pickupNickname is the deterministic courier pickup-location nickname for a
// seller. Stable across shipments so registration stays idempotent.
func pickupNickname(sellerID string) string {
	id := strings.ReplaceAll(sellerID, "-", "")
	if len(id) > 12 {
		id = id[:12]
	}
	return "seller_" + id
}

// estimateWeight returns a conservative total weight (grams) for the shipment.
// Default 500g per unit until real product weights are wired through.
func estimateWeight(items []shipping.ShipmentItem) int {
	total := 0
	for _, it := range items {
		q := it.Qty
		if q <= 0 {
			q = 1
		}
		total += 500 * q
	}
	if total == 0 {
		return 500
	}
	return total
}

func collect(rows pgx.Rows) ([]Shipment, error) {
	out := []Shipment{}
	for rows.Next() {
		s, err := scanShipment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// uuidShort returns a short unique suffix for synthetic self-ship AWBs.
func uuidShort() string {
	return strings.ToUpper(uuid.NewString()[:8])
}

// compact drops empty strings from a slice.
func compact(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// joinNonEmpty joins non-empty parts with sep.
func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func phoneLine(p string) string {
	if strings.TrimSpace(p) == "" {
		return ""
	}
	return "Phone: " + p
}

func nonEmptyStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
