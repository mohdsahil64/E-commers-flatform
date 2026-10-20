package orders

import "time"

// Order is the order aggregate (view model).
type Order struct {
	ID             string      `json:"id"`
	OrderNumber    string      `json:"order_number"`
	CustomerID     string      `json:"customer_id"`
	Status         string      `json:"status"`
	PaymentMethod  string      `json:"payment_method"`
	PaymentStatus  string      `json:"payment_status"`
	Currency       string      `json:"currency"`
	ItemsSubtotal  float64     `json:"items_subtotal"`
	DiscountTotal  float64     `json:"discount_total"`
	TaxTotal       float64     `json:"tax_total"`
	ShippingTotal  float64     `json:"shipping_total"`
	GrandTotal     float64     `json:"grand_total"`
	ShipName       string      `json:"ship_contact_name"`
	ShipPhone      string      `json:"ship_contact_phone"`
	ShipLine1      string      `json:"ship_line1"`
	ShipLine2      string      `json:"ship_line2"`
	ShipCity       string      `json:"ship_city"`
	ShipState      string      `json:"ship_state"`
	ShipPincode    string      `json:"ship_pincode"`
	ShipCountry    string      `json:"ship_country"`
	PlacedAt       *time.Time  `json:"placed_at"`
	CreatedAt      time.Time   `json:"created_at"`
	Items          []OrderItem `json:"items,omitempty"`
}

// OrderItem is a single fulfilment line with its financial breakdown.
type OrderItem struct {
	ID                string  `json:"id"`
	SellerID          string  `json:"seller_id"`
	SellerName        string  `json:"seller_name,omitempty"`
	SellerListingID   string  `json:"seller_listing_id"`
	ProductTitle      string  `json:"product_title"`
	ProductSlug       string  `json:"product_slug"`
	Image             string  `json:"image"`
	VariantLabel      string     `json:"variant_label"`
	SKU               string     `json:"sku"`
	ItemStatus        string     `json:"item_status"`
	DeliveredAt       *time.Time `json:"delivered_at"`
	Quantity          int     `json:"quantity"`
	UnitPrice         float64 `json:"unit_price"`
	GrossAmount       float64 `json:"gross_amount"`
	DiscountAmount    float64 `json:"discount_amount"`
	TaxAmount         float64 `json:"tax_amount"`
	ShippingCharge    float64 `json:"shipping_charge"`
	CommissionPercent float64 `json:"commission_percent"`
	CommissionAmount  float64 `json:"commission_amount"`
	PaymentFee        float64 `json:"payment_fee"`
	RefundAmount      float64 `json:"refund_amount"`
	SellerPayable     float64 `json:"seller_payable"`
	// UI helpers: whether this item was already reviewed, and the id of an
	// existing return/exchange request for it (empty when none). These let the
	// order detail switch the Review / Return-Exchange buttons appropriately.
	HasReview         bool    `json:"has_review"`
	ReturnID          string  `json:"return_id,omitempty"`
}

// StatusEvent is a row in the order status history.
type StatusEvent struct {
	FromStatus string    `json:"from_status"`
	ToStatus   string    `json:"to_status"`
	ActorRole  string    `json:"actor_role"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
}
