package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// apiHomeOTPSender delivers OTP codes as SMS via the apihome.in bulksms API.
//
// Request (GET): {base}?key={API_KEY}&mobile={MOBILE}&otp={CODE}
// Response (JSON): {"status":"Success","remark":"...","requestId":"..."}
//
// It is selected in production by setting SMS_PROVIDER=apihome and SMS_API_KEY.
// In dev the log sender is used instead and the fixed test code 123456 works.
type apiHomeOTPSender struct {
	apiKey  string
	baseURL string
	client  *http.Client
	logger  *slog.Logger
}

// NewAPIHomeOTPSender builds the apihome.in SMS OTP sender.
func NewAPIHomeOTPSender(apiKey, baseURL string, logger *slog.Logger) OTPSender {
	if logger == nil {
		logger = slog.Default()
	}
	if baseURL == "" {
		baseURL = "https://apihome.in/panel/api/bulksms/"
	}
	return &apiHomeOTPSender{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		logger:  logger,
	}
}

func (s *apiHomeOTPSender) Send(ctx context.Context, phone, code string) error {
	// The gateway expects a bare 10-digit mobile (strip +country code / spaces).
	mobile := normalizeMobile(phone)

	u, err := url.Parse(s.baseURL)
	if err != nil {
		return fmt.Errorf("sms: bad base url: %w", err)
	}
	q := u.Query()
	q.Set("key", s.apiKey)
	q.Set("mobile", mobile)
	q.Set("otp", code)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Error("sms send failed", "to", mobile, "error", err)
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	// Best-effort parse of the gateway's JSON envelope.
	var out struct {
		Status    string `json:"status"`
		Remark    string `json:"remark"`
		RequestID string `json:"requestId"`
	}
	_ = json.Unmarshal(body, &out)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !strings.EqualFold(out.Status, "Success") {
		s.logger.Error("sms send rejected", "to", mobile, "http", resp.StatusCode,
			"status", out.Status, "remark", out.Remark, "body", string(body))
		return fmt.Errorf("sms gateway error: %s", nonEmpty(out.Remark, string(body)))
	}
	s.logger.Info("sms sent", "to", mobile, "requestId", out.RequestID)
	return nil
}

func (s *apiHomeOTPSender) Name() string { return "apihome" }

// normalizeMobile reduces a phone to the local 10-digit number the gateway
// expects: it strips non-digits and any leading country code (91) so both
// "+91 98765 43210" and "9876543210" work.
func normalizeMobile(phone string) string {
	var b strings.Builder
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if len(d) > 10 && strings.HasPrefix(d, "91") {
		d = d[len(d)-10:]
	}
	return d
}

func nonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
