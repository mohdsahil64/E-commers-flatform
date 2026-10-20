// Package config loads all application configuration from environment
// variables. It is the single source of truth for runtime configuration and
// keeps the rest of the codebase free of direct os.Getenv calls.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the fully-resolved application configuration.
type Config struct {
	App      AppConfig
	DB       DBConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Storage  StorageConfig
	Payment  PaymentConfig
	Shipping ShippingConfig
	Email    EmailConfig
	SMS      SMSConfig
	CORS     CORSConfig

	// Marketplace defaults
	DefaultCommissionPercent float64
	DefaultCurrency          string
}

type AppConfig struct {
	Name     string
	Env      string
	Port     string
	BaseURL  string
	LogLevel string
}

func (a AppConfig) IsProduction() bool { return a.Env == "production" }

type DBConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
	BcryptCost    int
}

// StorageConfig configures media storage. The active backend is chosen by
// Provider ("local" or "cloudinary").
//
// local:      files are written under LocalRoot and served by the API's /media
//             static route. PublicBaseURL is the URL prefix the frontends use.
// cloudinary: files are uploaded to Cloudinary; objects are delivered straight
//             from Cloudinary's global CDN (fast loads). The stored object key
//             is reused verbatim as the Cloudinary public_id, so existing DB
//             image keys keep resolving after the one-time migration.
type StorageConfig struct {
	Provider      string // "local" (default) | "cloudinary"
	LocalRoot     string
	PublicBaseURL string

	// Cloudinary credentials (only used when Provider == "cloudinary").
	CloudinaryCloudName string
	CloudinaryAPIKey    string
	CloudinaryAPISecret string
	// Optional folder prefix applied to Cloudinary public_ids. Empty by default
	// so keys map 1:1 with the local layout (admin/..., seller/..., etc.).
	CloudinaryFolder string
}

type PaymentConfig struct {
	Provider      string // mock | razorpay | stripe
	KeyID         string
	KeySecret     string
	WebhookSecret string
	Currency      string
}

type ShippingConfig struct {
	Provider     string // mock | shiprocket | delhivery | bluedart
	APIKey       string
	APISecret    string
	BaseURL      string
	WebhookToken string // shared token validated on the courier tracking webhook
}

type EmailConfig struct {
	Provider string // log | smtp | ses
	From     string
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
}

type SMSConfig struct {
	Provider string // log (dev/test OTP) | apihome (real SMS)
	SenderID string
	APIKey   string
	BaseURL  string // SMS gateway endpoint (apihome bulksms API)
}

type CORSConfig struct {
	AllowedOrigins []string
}

// Load reads configuration from the environment. It attempts to load a `.env`
// file first (ignored if absent) so local development is friction-free.
func Load() (*Config, error) {
	// Best-effort .env load; real env vars always win.
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")
	_ = godotenv.Load("../../.env")

	cfg := &Config{
		App: AppConfig{
			Name:     getEnv("APP_NAME", "CARLITOS"),
			Env:      getEnv("APP_ENV", "development"),
			Port:     getEnv("APP_PORT", "8080"),
			BaseURL:  getEnv("APP_BASE_URL", "http://localhost:8080"),
			LogLevel: getEnv("LOG_LEVEL", "debug"),
		},
		DB: DBConfig{
			URL: resolveDatabaseURL(),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://localhost:6379/0"),
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("JWT_ACCESS_SECRET", "dev-access-secret"),
			RefreshSecret: getEnv("JWT_REFRESH_SECRET", "dev-refresh-secret"),
			AccessTTL:     time.Duration(getEnvInt("JWT_ACCESS_TTL_MINUTES", 30)) * time.Minute,
			RefreshTTL:    time.Duration(getEnvInt("JWT_REFRESH_TTL_HOURS", 168)) * time.Hour,
			BcryptCost:    getEnvInt("BCRYPT_COST", 12),
		},
		Storage: resolveStorageConfig(),
		Payment: PaymentConfig{
			Provider:      getEnv("PAYMENT_PROVIDER", "mock"),
			KeyID:         getEnv("PAYMENT_KEY_ID", ""),
			KeySecret:     getEnv("PAYMENT_KEY_SECRET", ""),
			WebhookSecret: getEnv("PAYMENT_WEBHOOK_SECRET", ""),
			Currency:      getEnv("PAYMENT_CURRENCY", "INR"),
		},
		Shipping: ShippingConfig{
			Provider:     getEnv("SHIPPING_PROVIDER", "mock"),
			APIKey:       getEnv("SHIPPING_API_KEY", ""),
			APISecret:    getEnv("SHIPPING_API_SECRET", ""),
			BaseURL:      getEnv("SHIPPING_BASE_URL", ""),
			WebhookToken: getEnv("SHIPPING_WEBHOOK_TOKEN", ""),
		},
		Email: EmailConfig{
			Provider: getEnv("EMAIL_PROVIDER", "log"),
			From:     getEnv("EMAIL_FROM", "no-reply@carlitos.local"),
			SMTPHost: getEnv("SMTP_HOST", ""),
			SMTPPort: getEnvInt("SMTP_PORT", 587),
			SMTPUser: getEnv("SMTP_USER", ""),
			SMTPPass: getEnv("SMTP_PASSWORD", ""),
		},
		SMS: SMSConfig{
			// "log" (default) = development: OTPs are logged and the fixed test
			// code 123456 is accepted (no real SMS). Set SMS_PROVIDER=apihome
			// with SMS_API_KEY to send real OTP SMS via the apihome bulksms API.
			Provider: getEnv("SMS_PROVIDER", "log"),
			SenderID: getEnv("SMS_SENDER_ID", "CRLTOS"),
			APIKey:   getEnv("SMS_API_KEY", ""),
			BaseURL:  getEnv("SMS_BASE_URL", "https://apihome.in/panel/api/bulksms/"),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitAndTrim(getEnv("CORS_ALLOWED_ORIGINS",
				"http://localhost:5173,http://localhost:5174,http://localhost:5175")),
		},
		DefaultCommissionPercent: getEnvFloat("DEFAULT_COMMISSION_PERCENT", 12.0),
		DefaultCurrency:          getEnv("DEFAULT_CURRENCY", "INR"),
	}

	if err := cfg.validateProduction(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateProduction refuses to start with insecure defaults when running in a
// production environment. In development these checks are skipped so the app
// stays friction-free.
func (c *Config) validateProduction() error {
	if !c.App.IsProduction() {
		return nil
	}
	var problems []string
	if c.JWT.AccessSecret == "" || c.JWT.AccessSecret == "dev-access-secret" {
		problems = append(problems, "JWT_ACCESS_SECRET must be set to a strong secret")
	}
	if c.JWT.RefreshSecret == "" || c.JWT.RefreshSecret == "dev-refresh-secret" {
		problems = append(problems, "JWT_REFRESH_SECRET must be set to a strong secret")
	}
	if c.JWT.AccessSecret == c.JWT.RefreshSecret {
		problems = append(problems, "JWT access and refresh secrets must differ")
	}
	if strings.EqualFold(c.Payment.Provider, "mock") || c.Payment.Provider == "" {
		problems = append(problems, "PAYMENT_PROVIDER must be a real gateway (not 'mock') in production")
	}
	if len(c.CORS.AllowedOrigins) == 0 {
		problems = append(problems, "CORS_ALLOWED_ORIGINS must list explicit origins in production")
	}
	// If a real courier is configured, require its credentials + webhook token so
	// shipments and tracking callbacks are authenticated. The mock courier is
	// still allowed in production for self-ship-only deployments.
	if !strings.EqualFold(c.Shipping.Provider, "mock") && c.Shipping.Provider != "" {
		if c.Shipping.APIKey == "" || c.Shipping.APISecret == "" {
			problems = append(problems, "SHIPPING_API_KEY and SHIPPING_API_SECRET are required for the configured courier")
		}
		if c.Shipping.WebhookToken == "" {
			problems = append(problems, "SHIPPING_WEBHOOK_TOKEN must be set to authenticate courier tracking webhooks")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("insecure production configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

// resolveStorageConfig builds the media StorageConfig from the environment.
//
// Cloudinary credentials are read from this project's existing .env variable
// names first (NAME_CLOUDINARY / API_KEY_CLOUDINART / SECRET_KEY), falling back
// to the conventional CLOUDINARY_* names. When a cloud name + api key + secret
// are all present the provider defaults to "cloudinary" (unless STORAGE_PROVIDER
// explicitly overrides); otherwise it stays on local-disk storage.
func resolveStorageConfig() StorageConfig {
	cloudName := firstNonEmpty(
		os.Getenv("NAME_CLOUDINARY"),
		os.Getenv("CLOUDINARY_CLOUD_NAME"),
	)
	apiKey := firstNonEmpty(
		os.Getenv("API_KEY_CLOUDINART"), // note: existing .env has this spelling
		os.Getenv("API_KEY_CLOUDINARY"),
		os.Getenv("CLOUDINARY_API_KEY"),
	)
	apiSecret := firstNonEmpty(
		os.Getenv("CLOUDINARY_API_SECRET"),
		os.Getenv("SECRET_KEY_CLOUDINARY"),
		os.Getenv("SECRET_KEY"),
	)

	haveCloudinary := cloudName != "" && apiKey != "" && apiSecret != ""

	// Provider selection: explicit STORAGE_PROVIDER wins; otherwise auto-pick
	// cloudinary when credentials are present, else local.
	provider := strings.ToLower(getEnv("STORAGE_PROVIDER", ""))
	if provider == "" {
		if haveCloudinary {
			provider = "cloudinary"
		} else {
			provider = "local"
		}
	}

	// For Cloudinary the public base URL defaults to the account's CDN delivery
	// prefix, so the frontends' existing resolveMediaUrl(base, key) keeps
	// working with no change: base + "/" + key → a valid Cloudinary URL.
	defaultPublicBase := "http://localhost:8080/media"
	if provider == "cloudinary" && cloudName != "" {
		defaultPublicBase = "https://res.cloudinary.com/" + cloudName + "/image/upload"
	}

	return StorageConfig{
		Provider:            provider,
		LocalRoot:           getEnv("STORAGE_LOCAL_ROOT", "./.data/storage"),
		PublicBaseURL:       getEnv("STORAGE_PUBLIC_BASE_URL", defaultPublicBase),
		CloudinaryCloudName: cloudName,
		CloudinaryAPIKey:    apiKey,
		CloudinaryAPISecret: apiSecret,
		CloudinaryFolder:    getEnv("CLOUDINARY_FOLDER", ""),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// resolveDatabaseURL prefers an explicit DATABASE_URL, otherwise assembles one
// from the discrete POSTGRES_* variables.
func resolveDatabaseURL() string {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		return v
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getEnv("POSTGRES_USER", "carlitos"),
		getEnv("POSTGRES_PASSWORD", "carlitos_secret"),
		getEnv("POSTGRES_HOST", "localhost"),
		getEnv("POSTGRES_PORT", "5432"),
		getEnv("POSTGRES_DB", "carlitos"),
		getEnv("POSTGRES_SSLMODE", "disable"),
	)
}

// ---- helpers ----

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
