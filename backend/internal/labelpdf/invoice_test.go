package labelpdf

import (
	"bytes"
	"testing"
)

func TestGenerateInvoiceProducesValidPDF(t *testing.T) {
	pdf := GenerateInvoice(Invoice{
		OrderNumber: "CRL-2026-000123",
		Date:        "12 Feb 2026",
		BillToName:  "Riya Sharma",
		BillToPhone: "9812345678",
		ShipLines:   []string{"44 Park Lane", "Mumbai, MH, 400001"},
		Lines: []InvoiceLine{
			{Title: "Classic Oxford Shirt", Qty: 1, Amount: "Rs 1,299"},
			{Title: "Slim Chinos", Qty: 2, Amount: "Rs 2,998"},
		},
		Subtotal:    "Rs 4,297",
		Tax:         "Rs 215",
		Shipping:    "FREE",
		Total:       "Rs 4,512",
		PaymentInfo: "Cash on Delivery",
	})
	if !bytes.HasPrefix(pdf, []byte("%PDF-1.4")) {
		t.Fatal("expected PDF header")
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Error("expected PDF EOF marker")
	}
	if !bytes.Contains(pdf, []byte("TAX INVOICE")) {
		t.Error("expected invoice heading in content")
	}
	if len(pdf) < 600 {
		t.Errorf("invoice PDF unexpectedly small: %d bytes", len(pdf))
	}
}
