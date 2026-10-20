package config

import "testing"

func baseProdConfig() *Config {
	c := &Config{}
	c.App.Env = "production"
	c.JWT.AccessSecret = "a-strong-access-secret"
	c.JWT.RefreshSecret = "a-strong-refresh-secret"
	c.Payment.Provider = "razorpay"
	c.CORS.AllowedOrigins = []string{"https://carlitos.example"}
	return c
}

func TestProductionRejectsDefaultSecrets(t *testing.T) {
	c := baseProdConfig()
	c.JWT.AccessSecret = "dev-access-secret"
	if err := c.validateProduction(); err == nil {
		t.Error("expected production validation to reject the dev access secret")
	}
}

func TestProductionRejectsMockPayment(t *testing.T) {
	c := baseProdConfig()
	c.Payment.Provider = "mock"
	if err := c.validateProduction(); err == nil {
		t.Error("expected production validation to reject the mock payment provider")
	}
}

func TestProductionRejectsEmptyCORS(t *testing.T) {
	c := baseProdConfig()
	c.CORS.AllowedOrigins = nil
	if err := c.validateProduction(); err == nil {
		t.Error("expected production validation to reject empty CORS origins")
	}
}

func TestProductionRejectsIdenticalSecrets(t *testing.T) {
	c := baseProdConfig()
	c.JWT.RefreshSecret = c.JWT.AccessSecret
	if err := c.validateProduction(); err == nil {
		t.Error("expected production validation to reject identical access/refresh secrets")
	}
}

func TestValidProductionConfigPasses(t *testing.T) {
	if err := baseProdConfig().validateProduction(); err != nil {
		t.Errorf("expected a well-formed production config to pass, got: %v", err)
	}
}

func TestDevelopmentSkipsValidation(t *testing.T) {
	c := &Config{}
	c.App.Env = "development"
	// All insecure defaults, but validation must be skipped in dev.
	c.JWT.AccessSecret = "dev-access-secret"
	c.Payment.Provider = "mock"
	if err := c.validateProduction(); err != nil {
		t.Errorf("expected development to skip production validation, got: %v", err)
	}
}
