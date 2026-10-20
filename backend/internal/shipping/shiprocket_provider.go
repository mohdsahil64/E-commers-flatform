package shipping

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carlitos/backend/internal/config"
)

// shiprocketProvider implements the courier Provider against Shiprocket's v1
// REST API. Auth is token-based: we log in with the API user's email/password
// (mapped from SHIPPING_API_KEY / SHIPPING_API_SECRET) to obtain a bearer token
// that is cached and refreshed automatically (Shiprocket tokens last ~10 days).
//
// The full forward flow is: create adhoc order -> assign AWB (auto-selects a
// courier) -> generate label -> request pickup. Tracking is polled by AWB and
// also delivered via webhook (handled in the fulfillment/webhook layer).
type shiprocketProvider struct {
	email    string
	password string
	base     string
	pickupID string // optional default pickup location nickname
	http     *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// NewShiprocketProvider builds the Shiprocket adapter.
func NewShiprocketProvider(cfg config.ShippingConfig) Provider {
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = "https://apiv2.shiprocket.in/v1/external"
	}
	return &shiprocketProvider{
		email:    cfg.APIKey,
		password: cfg.APISecret,
		base:     base,
		http:     &http.Client{Timeout: 20 * time.Second},
	}
}

func (p *shiprocketProvider) Name() string { return "shiprocket" }

// ── auth ────────────────────────────────────────────────────────────────────

func (p *shiprocketProvider) ensureToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Now().Before(p.tokenExp) {
		return p.token, nil
	}
	body, _ := json.Marshal(map[string]string{"email": p.email, "password": p.password})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+"/auth/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("shiprocket login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("shiprocket login failed: %s: %s", resp.Status, truncate(string(b), 300))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", fmt.Errorf("shiprocket login: empty token")
	}
	p.token = out.Token
	p.tokenExp = time.Now().Add(9 * 24 * time.Hour) // refresh well before 10-day expiry
	return p.token, nil
}

// doJSON performs an authenticated JSON request and decodes the response into out.
func (p *shiprocketProvider) doJSON(ctx context.Context, method, path string, payload any, out any) error {
	token, err := p.ensureToken(ctx)
	if err != nil {
		return err
	}
	var bodyReader io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.base+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("shiprocket %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("shiprocket %s %s: %s: %s", method, path, resp.Status, truncate(string(b), 400))
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ── serviceability ────────────────────────────────────────────────────────────

func (p *shiprocketProvider) CheckServiceability(ctx context.Context, in ServiceabilityInput) (*ServiceabilityResult, error) {
	q := fmt.Sprintf("/courier/serviceability/?pickup_postcode=%s&delivery_postcode=%s&weight=%.2f&cod=%d",
		in.FromPincode, in.ToPincode, float64(in.WeightGrams)/1000.0, boolToInt(in.COD))
	var out struct {
		Data struct {
			AvailableCourierCompanies []struct {
				CourierName    string  `json:"courier_name"`
				Rate           float64 `json:"rate"`
				EstimatedDays  string  `json:"estimated_delivery_days"`
			} `json:"available_courier_companies"`
		} `json:"data"`
	}
	if err := p.doJSON(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	if len(out.Data.AvailableCourierCompanies) == 0 {
		return &ServiceabilityResult{Serviceable: false}, nil
	}
	best := out.Data.AvailableCourierCompanies[0]
	eta, _ := strconv.Atoi(strings.TrimSpace(best.EstimatedDays))
	return &ServiceabilityResult{
		Serviceable: true,
		Courier:     best.CourierName,
		ChargeMinor: int64(best.Rate*100 + 0.5),
		ETADays:     eta,
	}, nil
}

// ── create shipment (adhoc order + assign AWB) ──────────────────────────────

func (p *shiprocketProvider) CreateShipment(ctx context.Context, in CreateShipmentInput) (*Shipment, error) {
	// Build the order_items from the real shipment lines (fallback to a single
	// line if none were supplied).
	var items []map[string]any
	var subTotal float64
	for _, it := range in.Items {
		units := it.Qty
		if units <= 0 {
			units = 1
		}
		items = append(items, map[string]any{
			"name": it.Name, "sku": nonEmpty(it.SKU, in.OrderNumber),
			"units": units, "selling_price": it.Price,
		})
		subTotal += it.Price * float64(units)
	}
	if len(items) == 0 {
		items = []map[string]any{
			{"name": "Order " + in.OrderNumber, "sku": in.OrderNumber, "units": 1, "selling_price": in.CODAmount},
		}
		subTotal = in.CODAmount
	}
	if in.COD {
		subTotal = in.CODAmount // COD amount is the authoritative collectable
	}

	// 1) Create an adhoc order from THIS seller's pickup location.
	payload := map[string]any{
		"order_id":              in.OrderNumber,
		"order_date":            time.Now().Format("2006-01-02 15:04"),
		"pickup_location":       nonEmpty(in.PickupLocation, p.pickupIDOrDefault()),
		"billing_customer_name": in.Delivery.Name,
		"billing_last_name":     "",
		"billing_address":       in.Delivery.Line1,
		"billing_address_2":     in.Delivery.Line2,
		"billing_city":          in.Delivery.City,
		"billing_pincode":       in.Delivery.Pincode,
		"billing_state":         in.Delivery.State,
		"billing_country":       nonEmpty(in.Delivery.Country, "India"),
		"billing_phone":         in.Delivery.Phone,
		"shipping_is_billing":   true,
		"order_items":           items,
		"payment_method":        codMethod(in.COD),
		"sub_total":             subTotal,
		"length":                10, "breadth": 10, "height": 5,
		"weight":                float64(maxInt(in.WeightGrams, 100)) / 1000.0,
	}
	var created struct {
		OrderID     int    `json:"order_id"`
		ShipmentID  int    `json:"shipment_id"`
		Status      string `json:"status"`
	}
	if err := p.doJSON(ctx, http.MethodPost, "/orders/create/adhoc", payload, &created); err != nil {
		return nil, err
	}

	// 2) Assign an AWB (auto-selects the cheapest available courier). The AWB
	//    response also carries the freight charge the platform account is billed.
	var awbResp struct {
		Response struct {
			Data struct {
				AWBCode       string  `json:"awb_code"`
				CourierName   string  `json:"courier_name"`
				FreightCharge float64 `json:"freight_charge"`
				Rate          float64 `json:"rate"`
			} `json:"data"`
		} `json:"response"`
	}
	_ = p.doJSON(ctx, http.MethodPost, "/courier/assign/awb",
		map[string]any{"shipment_id": created.ShipmentID}, &awbResp)

	awb := awbResp.Response.Data.AWBCode
	courier := nonEmpty(awbResp.Response.Data.CourierName, "Shiprocket")

	// Capture the actual freight billed to the platform. Prefer the AWB
	// response's freight_charge/rate; fall back to a serviceability quote.
	freight := awbResp.Response.Data.FreightCharge
	if freight <= 0 {
		freight = awbResp.Response.Data.Rate
	}
	if freight <= 0 {
		if sv, serr := p.CheckServiceability(ctx, ServiceabilityInput{
			FromPincode: in.Pickup.Pincode, ToPincode: in.Delivery.Pincode,
			WeightGrams: in.WeightGrams, COD: in.COD,
		}); serr == nil && sv != nil {
			freight = float64(sv.ChargeMinor) / 100.0
		}
	}

	// 3) Generate a label (best-effort).
	label := ""
	if awb != "" {
		label, _ = p.GenerateLabel(ctx, strconv.Itoa(created.ShipmentID))
	}

	return &Shipment{
		AWBNumber:          awb,
		Courier:            courier,
		LabelURL:           label,
		TrackingURL:        "https://shiprocket.co/tracking/" + awb,
		Status:             "created",
		ProviderShipmentID: strconv.Itoa(created.ShipmentID),
		FreightMinor:       int64(freight*100 + 0.5),
	}, nil
}

// GenerateLabel returns a label PDF URL for a shipment id.
func (p *shiprocketProvider) GenerateLabel(ctx context.Context, shipmentID string) (string, error) {
	sid, _ := strconv.Atoi(shipmentID)
	var out struct {
		LabelURL string `json:"label_url"`
	}
	if err := p.doJSON(ctx, http.MethodPost, "/courier/generate/label",
		map[string]any{"shipment_id": []int{sid}}, &out); err != nil {
		return "", err
	}
	return out.LabelURL, nil
}

// SchedulePickup requests a courier pickup for a shipment (best-effort; the AWB
// param here is the shipment id string).
func (p *shiprocketProvider) SchedulePickup(ctx context.Context, shipmentID string, _ time.Time) error {
	sid, _ := strconv.Atoi(shipmentID)
	return p.doJSON(ctx, http.MethodPost, "/courier/generate/pickup",
		map[string]any{"shipment_id": []int{sid}}, nil)
}

// Track returns normalized tracking events for an AWB.
func (p *shiprocketProvider) Track(ctx context.Context, awb string) ([]TrackingEvent, error) {
	var out struct {
		TrackingData struct {
			ShipmentTrack []struct {
				CurrentStatus string `json:"current_status"`
			} `json:"shipment_track"`
			ShipmentTrackActivities []struct {
				Date     string `json:"date"`
				Status   string `json:"status"`
				Activity string `json:"activity"`
				Location string `json:"location"`
			} `json:"shipment_track_activities"`
		} `json:"tracking_data"`
	}
	if err := p.doJSON(ctx, http.MethodGet, "/courier/track/awb/"+awb, nil, &out); err != nil {
		return nil, err
	}
	var events []TrackingEvent
	for _, a := range out.TrackingData.ShipmentTrackActivities {
		t, _ := time.Parse("2006-01-02 15:04:05", a.Date)
		events = append(events, TrackingEvent{
			Status:      normalizeStatus(a.Status),
			Location:    a.Location,
			Description: nonEmpty(a.Activity, a.Status),
			EventTime:   t,
		})
	}
	return events, nil
}

// Cancel cancels a shipment by AWB.
func (p *shiprocketProvider) Cancel(ctx context.Context, awb string) error {
	return p.doJSON(ctx, http.MethodPost, "/orders/cancel/shipment/awbs",
		map[string]any{"awbs": []string{awb}}, nil)
}

func (p *shiprocketProvider) pickupIDOrDefault() string {
	if p.pickupID != "" {
		return p.pickupID
	}
	return "Primary" // Shiprocket's default pickup nickname
}

// RegisterPickupLocation registers a seller's warehouse as a named pickup
// location in the marketplace's Shiprocket account. The nickname is returned so
// shipments can route pickup to the correct seller. If the location already
// exists (duplicate nickname), Shiprocket returns an error we treat as success
// (idempotent), since the location is already usable.
func (p *shiprocketProvider) RegisterPickupLocation(ctx context.Context, in PickupLocationInput) (string, error) {
	payload := map[string]any{
		"pickup_location": in.Nickname,
		"name":            nonEmpty(in.Address.Name, in.Nickname),
		"email":           nonEmpty(in.Email, "seller@carlitos.in"),
		"phone":           in.Address.Phone,
		"address":         in.Address.Line1,
		"address_2":       in.Address.Line2,
		"city":            in.Address.City,
		"state":           in.Address.State,
		"country":         nonEmpty(in.Address.Country, "India"),
		"pin_code":        in.Address.Pincode,
	}
	err := p.doJSON(ctx, http.MethodPost, "/settings/company/addpickup", payload, nil)
	if err != nil {
		// Already-registered nicknames come back as an error; treat as success so
		// the operation is idempotent and safe to retry.
		if strings.Contains(strings.ToLower(err.Error()), "already") ||
			strings.Contains(err.Error(), "422") {
			return in.Nickname, nil
		}
		return "", err
	}
	return in.Nickname, nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func codMethod(cod bool) string {
	if cod {
		return "COD"
	}
	return "Prepaid"
}
func nonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// normalizeStatus maps courier status text to our canonical tracking statuses.
func normalizeStatus(s string) string { return NormalizeStatus(s) }

// NormalizeStatus maps arbitrary courier status text to our canonical tracking
// statuses (picked_up | in_transit | out_for_delivery | delivered | returned |
// cancelled). Exported so webhook handlers can reuse the same mapping.
func NormalizeStatus(s string) string {
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "delivered"):
		return "delivered"
	case strings.Contains(l, "out for delivery"), strings.Contains(l, "ofd"):
		return "out_for_delivery"
	case strings.Contains(l, "in transit"), strings.Contains(l, "transit"):
		return "in_transit"
	case strings.Contains(l, "picked"), strings.Contains(l, "pickup"):
		return "picked_up"
	case strings.Contains(l, "rto"), strings.Contains(l, "return"):
		return "returned"
	case strings.Contains(l, "cancel"):
		return "cancelled"
	default:
		return "in_transit"
	}
}
