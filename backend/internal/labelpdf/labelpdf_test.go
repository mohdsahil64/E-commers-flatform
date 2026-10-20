package labelpdf

import (
	"bytes"
	"testing"
)

func TestGenerateProducesValidPDF(t *testing.T) {
	pdf := Generate(Label{
		OrderNumber: "CRL-2026-000123",
		AWB:         "SELFABC12345",
		StoreName:   "Acme Threads",
		FromLines:   []string{"12 Market Rd", "Bengaluru, KA, 560001", "Phone: 9876543210"},
		ToName:      "Riya Sharma",
		ToPhone:     "9812345678",
		ToLines:     []string{"44 Park Lane", "Mumbai, MH, 400001"},
		Items:       []string{"Classic Oxford Shirt x1", "Slim Chinos x2"},
	})
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatalf("expected PDF header, got: %q", pdf[:min(12, len(pdf))])
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Error("expected PDF EOF marker")
	}
	if !bytes.Contains(pdf, []byte("CARLITOS")) {
		t.Error("expected CARLITOS header text in content stream")
	}
	if len(pdf) < 500 {
		t.Errorf("PDF unexpectedly small: %d bytes", len(pdf))
	}
}

func TestEscapePDF(t *testing.T) {
	got := escapePDF(`A (test) \ with é`)
	if got != `A \(test\) \\ with ` {
		t.Errorf("escapePDF mismatch: %q", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
