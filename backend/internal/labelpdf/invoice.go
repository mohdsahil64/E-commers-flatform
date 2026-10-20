package labelpdf

import (
	"bytes"
	"fmt"
)

// InvoiceLine is one row on the invoice.
type InvoiceLine struct {
	Title  string
	Qty    int
	Amount string // pre-formatted, e.g. "Rs 1,299"
}

// Invoice is the data rendered onto a customer invoice PDF.
type Invoice struct {
	OrderNumber string
	Date        string
	BillToName  string
	BillToPhone string
	ShipLines   []string
	Lines       []InvoiceLine
	Subtotal    string
	Discount    string
	Tax         string
	Shipping    string
	Total       string
	PaymentInfo string // e.g. "Cash on Delivery" | "Paid online"
}

// GenerateInvoice returns an A4 customer invoice as PDF bytes. Like the label
// generator it is hand-built (no third-party dependency) using Helvetica.
func GenerateInvoice(in Invoice) []byte {
	var c bytes.Buffer
	text := func(x, y, size float64, s string) {
		fmt.Fprintf(&c, "BT /F1 %.0f Tf %.1f %.1f Td (%s) Tj ET\n", size, x, y, escapePDF(s))
	}
	textR := func(xRight, y, size float64, s string) {
		// Right-align by estimating width (Helvetica avg ~0.5em per char).
		w := float64(len(s)) * size * 0.5
		text(xRight-w, y, size, s)
	}
	hline := func(x1, y, x2 float64) {
		fmt.Fprintf(&c, "%.1f %.1f m %.1f %.1f l S\n", x1, y, x2, y)
	}

	// A4 = 595 x 842 points. Left margin 48, right edge 547.
	const left, right = 48.0, 547.0
	y := 800.0

	text(left, y, 22, "CARLITOS")
	textR(right, y, 12, "TAX INVOICE")
	y -= 16
	text(left, y, 9, "CARLITOS Marketplace")
	y -= 20
	hline(left, y, right)
	y -= 22

	text(left, y, 10, "Invoice / Order: "+in.OrderNumber)
	textR(right, y, 10, "Date: "+in.Date)
	y -= 24

	// Bill to / ship to.
	text(left, y, 11, "Billed / Shipped To:")
	y -= 15
	text(left, y, 10, in.BillToName)
	y -= 13
	if in.BillToPhone != "" {
		text(left, y, 10, "Phone: "+in.BillToPhone)
		y -= 13
	}
	for _, ln := range in.ShipLines {
		text(left, y, 10, ln)
		y -= 13
	}
	y -= 10
	hline(left, y, right)
	y -= 20

	// Table header.
	text(left, y, 10, "Item")
	text(430, y, 10, "Qty")
	textR(right, y, 10, "Amount")
	y -= 6
	hline(left, y, right)
	y -= 16

	// Lines.
	for _, l := range in.Lines {
		if y < 160 {
			break
		}
		title := l.Title
		if len(title) > 60 {
			title = title[:57] + "..."
		}
		text(left, y, 10, title)
		text(434, y, 10, fmt.Sprintf("%d", l.Qty))
		textR(right, y, 10, l.Amount)
		y -= 16
	}

	y -= 4
	hline(left, y, right)
	y -= 20

	// Totals block (right-aligned label + amount).
	totalRow := func(label, amount string, bold bool) {
		size := 10.0
		if bold {
			size = 12.0
		}
		textR(460, y, size, label)
		textR(right, y, size, amount)
		y -= 18
	}
	totalRow("Subtotal", in.Subtotal, false)
	if in.Discount != "" {
		totalRow("Discount", "-"+in.Discount, false)
	}
	totalRow("Tax", in.Tax, false)
	totalRow("Shipping", in.Shipping, false)
	y -= 2
	hline(360, y+8, right)
	totalRow("Grand Total", in.Total, true)

	y -= 10
	text(left, y, 10, "Payment: "+in.PaymentInfo)
	y -= 40
	text(left, y, 9, "This is a computer-generated invoice and does not require a signature.")
	y -= 12
	text(left, y, 9, "Thank you for shopping with CARLITOS.")

	content := c.Bytes()

	// Assemble PDF.
	var buf bytes.Buffer
	offsets := make([]int, 0, 6)
	write := func(s string) { buf.WriteString(s) }
	obj := func(body string) {
		offsets = append(offsets, buf.Len())
		write(body)
	}
	write("%PDF-1.4\n")
	obj("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	obj("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	obj("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] " +
		"/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n")
	obj("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")
	obj(fmt.Sprintf("5 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content))

	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(offsets)+1)
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF",
		len(offsets)+1, xrefStart)
	return buf.Bytes()
}
