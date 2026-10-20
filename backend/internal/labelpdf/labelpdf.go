// Package labelpdf generates a minimal, dependency-free PDF shipping label for
// self-shipped orders. It produces a single-page A6-style label with the
// CARLITOS header, order number, AWB reference, seller (from) and customer (to)
// addresses, and item summary — enough for a seller to print and affix.
//
// The PDF is hand-built (a small subset of the PDF spec) so no third-party
// library is required. Text is drawn with the standard Helvetica font.
package labelpdf

import (
	"bytes"
	"fmt"
	"strings"
)

// Label is the data rendered onto the shipping label.
type Label struct {
	OrderNumber string
	AWB         string
	StoreName   string
	FromLines   []string // seller pickup address lines
	ToName      string
	ToPhone     string
	ToLines     []string // customer delivery address lines
	Items       []string // "Product xN"
}

// Generate returns the label as PDF bytes.
func Generate(l Label) []byte {
	// Build the page content stream (PDF text operators). Coordinates are in
	// points from the bottom-left; the page is 298x420 (A6 portrait-ish).
	var c bytes.Buffer
	line := func(x, y float64, size float64, s string) {
		fmt.Fprintf(&c, "BT /F1 %.0f Tf %.1f %.1f Td (%s) Tj ET\n", size, x, y, escapePDF(s))
	}
	rect := func(x, y, w, h float64) {
		fmt.Fprintf(&c, "%.1f %.1f %.1f %.1f re S\n", x, y, w, h)
	}
	hline := func(x1, y, x2 float64) {
		fmt.Fprintf(&c, "%.1f %.1f m %.1f %.1f l S\n", x1, y, x2, y)
	}

	// Border.
	c.WriteString("0.6 w\n")
	rect(12, 12, 274, 396)

	y := 388.0
	line(24, y, 18, "CARLITOS")
	y -= 16
	line(24, y, 9, "Self-Ship Shipping Label")
	y -= 8
	hline(24, y, 274)
	y -= 18

	line(24, y, 10, "Order: "+l.OrderNumber)
	y -= 14
	line(24, y, 10, "Ref (AWB): "+l.AWB)
	y -= 8
	hline(24, y, 274)
	y -= 18

	// FROM
	line(24, y, 9, "FROM (Seller):")
	y -= 13
	line(24, y, 10, l.StoreName)
	y -= 13
	for _, ln := range l.FromLines {
		line(24, y, 9, ln)
		y -= 12
	}
	y -= 6
	hline(24, y, 274)
	y -= 18

	// TO
	line(24, y, 9, "DELIVER TO:")
	y -= 13
	line(24, y, 11, l.ToName)
	y -= 13
	if l.ToPhone != "" {
		line(24, y, 10, "Phone: "+l.ToPhone)
		y -= 13
	}
	for _, ln := range l.ToLines {
		line(24, y, 10, ln)
		y -= 13
	}
	y -= 6
	hline(24, y, 274)
	y -= 16

	// Items
	line(24, y, 9, "ITEMS:")
	y -= 12
	for _, it := range l.Items {
		if y < 30 {
			break
		}
		line(28, y, 9, "- "+it)
		y -= 11
	}

	content := c.Bytes()

	// Assemble the PDF objects.
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
	obj("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 298 420] " +
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

// escapePDF escapes characters special to PDF string literals.
func escapePDF(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "(", `\(`)
	s = strings.ReplaceAll(s, ")", `\)`)
	// Drop non-ASCII to keep the WinAnsi/Helvetica encoding simple.
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r < 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}
