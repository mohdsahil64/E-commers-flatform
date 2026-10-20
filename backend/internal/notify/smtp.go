package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/carlitos/backend/internal/config"
)

type smtpEmailSender struct {
	cfg config.EmailConfig
}

func newSMTPEmailSender(cfg config.EmailConfig) EmailSender {
	return &smtpEmailSender{cfg: cfg}
}

func (s *smtpEmailSender) Name() string { return "smtp" }

func (s *smtpEmailSender) Send(_ context.Context, msg EmailMessage) error {
	host := s.cfg.SMTPHost
	addr := net.JoinHostPort(host, strconv.Itoa(s.cfg.SMTPPort))
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, host)
	raw := s.buildRaw(msg)

	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Quit() //nolint:errcheck

	if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("smtp starttls: %w", err)
	}
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, to := range strings.Split(msg.To, ",") {
		if t := strings.TrimSpace(to); t != "" {
			if err := client.Rcpt(t); err != nil {
				return fmt.Errorf("smtp rcpt: %w", err)
			}
		}
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := fmt.Fprint(wc, raw); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	return wc.Close()
}

func (s *smtpEmailSender) buildRaw(msg EmailMessage) string {
	date := time.Now().Format(time.RFC1123Z)
	var b strings.Builder

	if !msg.IsHTML {
		b.WriteString("MIME-Version: 1.0\r\n")
		b.WriteString("Date: " + date + "\r\n")
		b.WriteString("From: " + s.cfg.From + "\r\n")
		b.WriteString("To: " + msg.To + "\r\n")
		b.WriteString("Subject: " + msg.Subject + "\r\n")
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		b.WriteString("\r\n")
		b.WriteString(msg.Body)
		return b.String()
	}

	boundary := fmt.Sprintf("==_carlitos_%d", time.Now().UnixNano())
	plain := stripTags(msg.Body)

	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Date: " + date + "\r\n")
	b.WriteString("From: " + s.cfg.From + "\r\n")
	b.WriteString("To: " + msg.To + "\r\n")
	b.WriteString("Subject: " + msg.Subject + "\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	b.WriteString("\r\n")

	// plain part
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(plain + "\r\n\r\n")

	// html part — base64 encoded to handle large embedded images safely
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("\r\n")
	b.WriteString(chunkBase64(msg.Body))
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "--\r\n")
	return b.String()
}

// chunkBase64 base64-encodes s and wraps lines at 76 chars per RFC 2045.
func chunkBase64(s string) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	data := []byte(s)
	var b strings.Builder
	col := 0
	for i := 0; i < len(data); i += 3 {
		var v uint32
		n := 0
		for j := 0; j < 3 && i+j < len(data); j++ {
			v = v<<8 | uint32(data[i+j])
			n++
		}
		v <<= uint((3 - n) * 8)
		out := [4]byte{
			chars[v>>18&0x3F],
			chars[v>>12&0x3F],
			chars[v>>6&0x3F],
			chars[v&0x3F],
		}
		if n < 3 {
			out[3] = '='
		}
		if n < 2 {
			out[2] = '='
		}
		b.Write(out[:])
		col += 4
		if col >= 76 {
			b.WriteString("\r\n")
			col = 0
		}
	}
	if col > 0 {
		b.WriteString("\r\n")
	}
	return b.String()
}

func stripTags(html string) string {
	var b strings.Builder
	in := false
	for _, r := range html {
		if r == '<' {
			in = true
		} else if r == '>' {
			in = false
		} else if !in {
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
