// Package catalog implements the marketplace catalogue: taxonomy (genders,
// categories, brands, attributes) and the normalized product model (products,
// variants, seller listings). It powers the public storefront, seller catalog
// management and admin product approval.
package catalog

import "time"

// Gender is a top-level navigation dimension.
type Gender struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// Category (also used for subcategories via ParentID / Level).
type Category struct {
	ID          string  `json:"id"`
	ParentID    *string `json:"parent_id"`
	GenderID    *string `json:"gender_id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Level       int     `json:"level"`
	Description string  `json:"description"`
	ImageKey    *string `json:"image_key"`
	SortOrder   int     `json:"sort_order"`
	IsActive    bool    `json:"is_active"`
	Children    []Category `json:"children,omitempty"`
}

// Brand of a product.
type Brand struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Slug     string  `json:"slug"`
	LogoKey  *string `json:"logo_key"`
	IsActive bool    `json:"is_active"`
}

// Attribute definition (Size, Color, ...).
type Attribute struct {
	ID           string           `json:"id"`
	Code         string           `json:"code"`
	Name         string           `json:"name"`
	InputType    string           `json:"input_type"`
	IsVariant    bool             `json:"is_variant"`
	IsFilterable bool             `json:"is_filterable"`
	SortOrder    int              `json:"sort_order"`
	Values       []AttributeValue `json:"values,omitempty"`
}

// AttributeValue is an allowed value for an attribute.
type AttributeValue struct {
	ID           string         `json:"id"`
	AttributeID  string         `json:"attribute_id"`
	Value        string         `json:"value"`
	DisplayValue string         `json:"display_value"`
	Meta         map[string]any `json:"meta"`
	SortOrder    int            `json:"sort_order"`
}

// Product is the canonical catalogue item.
type Product struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Slug            string     `json:"slug"`
	Description     string     `json:"description"`
	BrandID         *string    `json:"brand_id"`
	BrandName       string     `json:"brand_name,omitempty"`
	GenderID        *string    `json:"gender_id"`
	CategoryID      *string    `json:"category_id"`
	SubcategoryID   *string    `json:"subcategory_id"`
	Status          string     `json:"status"`
	RejectionReason string     `json:"rejection_reason,omitempty"`
	RatingAvg       float64    `json:"rating_avg"`
	RatingCount     int        `json:"rating_count"`
	HSNCode         string     `json:"hsn_code"`
	TaxPercent      float64    `json:"tax_percent"`
	Season          string     `json:"season"`
	CreatedBySeller *string    `json:"created_by_seller"`
	CreatedAt       time.Time  `json:"created_at"`
	Images          []Image    `json:"images,omitempty"`
	Variants        []Variant  `json:"variants,omitempty"`
	// AvailableQty is the seller's current stock for the primary listing.
	// Populated for seller-owned reads so the seller sees a live count.
	AvailableQty *int `json:"available_quantity,omitempty"`
	// SellingPrice / MRP from the seller's listing, surfaced on list views so
	// the seller sees the final price without hydrating variants per row.
	SellingPrice *float64 `json:"selling_price,omitempty"`
	MRP          *float64 `json:"mrp,omitempty"`
}

// Image belonging to a product.
type Image struct {
	ID        string `json:"id"`
	FileKey   string `json:"file_key"`
	URL       string `json:"url,omitempty"`
	AltText   string `json:"alt_text"`
	SortOrder int    `json:"sort_order"`
	IsPrimary bool   `json:"is_primary"`
}

// Variant is a purchasable variation of a product.
type Variant struct {
	ID           string  `json:"id"`
	ProductID    string  `json:"product_id"`
	VariantCode  string  `json:"variant_code"`
	Barcode      *string `json:"barcode"`
	MRP          float64 `json:"mrp"`
	SellingPrice float64 `json:"selling_price"`
	WeightGrams  int     `json:"weight_grams"`
	IsActive     bool    `json:"is_active"`
	// Size label (e.g. "M" or "8"), the seller's listing id for this variant,
	// and its live available stock. Populated when a size-scoped context is
	// available (seller owns it, or storefront where a single active listing).
	Size            string `json:"size,omitempty"`
	SellerListingID string `json:"seller_listing_id,omitempty"`
	AvailableQty    int    `json:"available_quantity"`
}

// SellerListing connects a seller to a product/variant with commercial data.
type SellerListing struct {
	ID              string    `json:"id"`
	SellerID        string    `json:"seller_id"`
	ProductID       string    `json:"product_id"`
	VariantID       string    `json:"variant_id"`
	SKU             string    `json:"sku"`
	MRP             float64   `json:"mrp"`
	SellingPrice    float64   `json:"selling_price"`
	DiscountPercent float64   `json:"discount_percent"`
	Currency        string    `json:"currency"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}
