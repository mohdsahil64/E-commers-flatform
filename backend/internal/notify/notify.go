package notify

import (
	"context"
	"log/slog"

	"github.com/carlitos/backend/internal/config"
)

type EmailMessage struct {
	To      string
	Subject string
	Body    string
	IsHTML  bool
}

type SMSMessage struct {
	To   string
	Body string
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
	Name() string
}

type SMSSender interface {
	Send(ctx context.Context, msg SMSMessage) error
	Name() string
}

type Dispatcher struct {
	Email EmailSender
	SMS   SMSSender
}

func NewDispatcher(cfg *config.Config, logger *slog.Logger) *Dispatcher {
	return &Dispatcher{
		Email: newEmailSender(cfg.Email, logger),
		SMS:   newSMSSender(cfg.SMS, logger),
	}
}

type logEmailSender struct {
	from   string
	logger *slog.Logger
}

func (s *logEmailSender) Send(_ context.Context, msg EmailMessage) error {
	s.logger.Info("email (dev — not sent)", "from", s.from, "to", msg.To, "subject", msg.Subject)
	return nil
}
func (s *logEmailSender) Name() string { return "log" }

type logSMSSender struct {
	sender string
	logger *slog.Logger
}

func (s *logSMSSender) Send(_ context.Context, msg SMSMessage) error {
	s.logger.Info("sms (dev — not sent)", "sender", s.sender, "to", msg.To)
	return nil
}
func (s *logSMSSender) Name() string { return "log" }

func newEmailSender(cfg config.EmailConfig, logger *slog.Logger) EmailSender {
	switch cfg.Provider {
	case "smtp":
		if cfg.SMTPHost == "" || cfg.SMTPUser == "" {
			logger.Warn("EMAIL_PROVIDER=smtp but SMTP_HOST/SMTP_USER not set — falling back to log")
			return &logEmailSender{from: cfg.From, logger: logger}
		}
		logger.Info("email adapter: smtp", "host", cfg.SMTPHost, "port", cfg.SMTPPort)
		return newSMTPEmailSender(cfg)
	default:
		return &logEmailSender{from: cfg.From, logger: logger}
	}
}

func newSMSSender(cfg config.SMSConfig, logger *slog.Logger) SMSSender {
	return &logSMSSender{sender: cfg.SenderID, logger: logger}
}
