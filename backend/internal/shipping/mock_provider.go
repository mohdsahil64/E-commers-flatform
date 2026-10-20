package shipping

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/carlitos/backend/internal/config"
)

// mockProvider is a development courier adapter. It always reports pincodes as
// serviceable and generates deterministic AWB/label/tracking data so the whole
// fulfilment flow can be exercised without a courier account.
type mockProvider struct {
	courier    string
	publicBase string
}

// NewMockProvider builds the development courier adapter.
func NewMockProvider() Provider {
	return &mockProvider{courier: "CarlitosExpress", publicBase: "https://track.carlitos.local"}
}

func (p *mockProvider) CheckServiceability(ctx context.Context, in ServiceabilityInput) (*ServiceabilityResult, error) {
	return &ServiceabilityResult{
		Serviceable: true,
		Courier:     p.courier,
		ChargeMinor: 4900, // ₹49 flat in dev
		ETADays:     4,
	}, nil
}

func (p *mockProvider) CreateShipment(ctx context.Context, in CreateShipmentInput) (*Shipment, error) {
	awb := "CRLAWB" + fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_0000)
	return &Shipment{
		AWBNumber:          awb,
		Courier:            p.courier,
		LabelURL:           p.publicBase + "/label/" + awb + ".pdf",
		TrackingURL:        p.publicBase + "/awb/" + awb,
		Status:             "created",
		ProviderShipmentID: "MOCK" + awb,
		FreightMinor:       4900, // ₹49 flat in dev, mirrors serviceability
	}, nil
}

func (p *mockProvider) GenerateLabel(ctx context.Context, awb string) (string, error) {
	return p.publicBase + "/label/" + awb + ".pdf", nil
}

func (p *mockProvider) SchedulePickup(ctx context.Context, awb string, when time.Time) error {
	return nil
}

func (p *mockProvider) Track(ctx context.Context, awb string) ([]TrackingEvent, error) {
	now := time.Now()
	return []TrackingEvent{
		{Status: "picked_up", Location: "Origin Hub", Description: "Shipment picked up", EventTime: now.Add(-48 * time.Hour)},
		{Status: "in_transit", Location: "Transit Hub", Description: "In transit", EventTime: now.Add(-24 * time.Hour)},
		{Status: "out_for_delivery", Location: "Destination", Description: "Out for delivery", EventTime: now},
	}, nil
}

func (p *mockProvider) Cancel(ctx context.Context, awb string) error { return nil }

// RegisterPickupLocation is a no-op in dev; it echoes the nickname back.
func (p *mockProvider) RegisterPickupLocation(ctx context.Context, in PickupLocationInput) (string, error) {
	if in.Nickname == "" {
		return "Primary", nil
	}
	return in.Nickname, nil
}

func (p *mockProvider) Name() string { return "mock" }

// NewProvider selects the configured courier provider. The mock adapter is used
// for development; "shiprocket" is the real courier aggregator. Unknown/unset
// providers fall back to mock so the app still boots in dev.
func NewProvider(cfg config.ShippingConfig) Provider {
	switch strings.ToLower(cfg.Provider) {
	case "shiprocket":
		if cfg.APIKey == "" || cfg.APISecret == "" {
			// Credentials missing — fall back to mock so dev still works.
			return NewMockProvider()
		}
		return NewShiprocketProvider(cfg)
	case "mock", "":
		return NewMockProvider()
	default:
		return NewMockProvider()
	}
}
