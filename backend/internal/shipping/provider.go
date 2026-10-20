// Package shipping provides a courier-agnostic abstraction plus a development
// (mock) adapter. The system supports serviceability, shipment creation, AWB
// and label generation, pickup, tracking, cancellation and return shipments
// without hardcoding any single courier (Shiprocket/Delhivery/Blue Dart/...).
package shipping

import (
	"context"
	"time"
)

// Address is a pickup or delivery address.
type Address struct {
	Name    string
	Phone   string
	Line1   string
	Line2   string
	City    string
	State   string
	Pincode string
	Country string
}

// ServiceabilityInput checks whether a pincode pair is serviceable.
type ServiceabilityInput struct {
	FromPincode string
	ToPincode   string
	WeightGrams int
	COD         bool
}

// ServiceabilityResult reports serviceability and an indicative charge/ETA.
type ServiceabilityResult struct {
	Serviceable bool
	Courier     string
	ChargeMinor int64 // in paise
	ETADays     int
}

// CreateShipmentInput requests a shipment (forward or return).
type CreateShipmentInput struct {
	OrderNumber string
	Pickup      Address
	Delivery    Address
	WeightGrams int
	COD         bool
	CODAmount   float64
	Return      bool
	// PickupLocation is the courier-registered pickup nickname for the seller
	// whose items are shipping. Each seller has its own pickup location so the
	// courier collects from the correct warehouse. Empty falls back to default.
	PickupLocation string
	// Items are the line items in this seller's shipment (name/qty/price) so the
	// courier manifest/label reflects the real contents.
	Items []ShipmentItem
}

// ShipmentItem is a single line in a shipment manifest.
type ShipmentItem struct {
	Name  string
	SKU   string
	Qty   int
	Price float64
}

// PickupLocationInput registers a seller's warehouse/pickup point with the
// courier so shipments can be collected from that seller's address.
type PickupLocationInput struct {
	Nickname string // unique per seller, e.g. "seller_<id>"
	Address  Address
	Email    string
}

// Shipment is the courier's created shipment handle.
type Shipment struct {
	AWBNumber          string
	Courier            string
	LabelURL           string
	TrackingURL        string
	Status             string
	ProviderShipmentID string // courier's internal shipment id (for label/pickup/track)
	// FreightMinor is the actual courier freight charge (in paise) that the
	// platform's courier account is billed for this shipment. It is captured at
	// AWB-assignment time and later deducted from the seller's settlement.
	FreightMinor int64
}

// TrackingEvent is a normalized tracking update.
type TrackingEvent struct {
	Status      string
	Location    string
	Description string
	EventTime   time.Time
}

// Provider is the courier abstraction.
type Provider interface {
	CheckServiceability(ctx context.Context, in ServiceabilityInput) (*ServiceabilityResult, error)
	CreateShipment(ctx context.Context, in CreateShipmentInput) (*Shipment, error)
	GenerateLabel(ctx context.Context, awb string) (labelURL string, err error)
	SchedulePickup(ctx context.Context, awb string, when time.Time) error
	Track(ctx context.Context, awb string) ([]TrackingEvent, error)
	Cancel(ctx context.Context, awb string) error
	// RegisterPickupLocation registers (or confirms) a seller's pickup point and
	// returns the nickname the courier knows it by. Idempotent per nickname.
	RegisterPickupLocation(ctx context.Context, in PickupLocationInput) (nickname string, err error)
	Name() string
}
