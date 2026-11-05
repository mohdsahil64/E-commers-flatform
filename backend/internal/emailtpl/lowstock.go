package emailtpl

import "fmt"

// LowStockItem is a single variant that has fallen to/below its low-stock level.
type LowStockItem struct {
	Title     string
	Variant   string
	Available int
}

// SellerLowStockData feeds the low-stock alert email.
type SellerLowStockData struct {
	StoreName string
	Items     []LowStockItem
	DashURL   string
}

func lowStockRows(items []LowStockItem) string {
	rows := ""
	for _, it := range items {
		variant := ""
		if it.Variant != "" {
			variant = `<span style="color:` + colorMuted + `;font-size:12px"> · ` + esc(it.Variant) + `</span>`
		}
		left := fmt.Sprintf("%d left", it.Available)
		if it.Available <= 0 {
			left = "Out of stock"
		}
		rows += fmt.Sprintf(
			`<tr><td style="padding:9px 0;border-bottom:1px solid %s;font-size:13px;color:%s">%s%s</td>`+
				`<td align="right" style="padding:9px 0;border-bottom:1px solid %s;font-size:13px;font-weight:600;color:%s;white-space:nowrap">%s</td></tr>`,
			colorLine, colorInk, esc(it.Title), variant, colorLine, colorInk, esc(left))
	}
	return `<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:10px 0 6px">` + rows + `</table>`
}

// SellerLowStock tells a seller that some of their items are running low.
func SellerLowStock(d SellerLowStockData) string {
	content := h2("Low stock alert") +
		p("Hi "+esc(d.StoreName)+", the following items are running low. Restock soon so you don't lose sales.") +
		lowStockRows(d.Items) +
		btn("Update inventory", d.DashURL) +
		hr() +
		note("You can turn these alerts off anytime from Settings → Notifications.")
	return render("Low stock alert on CARLITOS", "Some of your items are running low on stock.", content)
}
