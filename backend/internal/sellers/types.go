// Package sellers implements the seller lifecycle: onboarding (business/KYC),
// admin review (approve/reject/suspend/activate/block) and seller profile
// access. It is the source of truth for whether a seller may use the desk.
package sellers

import "time"

// Seller is the seller store/account record.
type Seller struct {
	ID                string    `json:"id"`
	UserID            string    `json:"user_id"`
	StoreName         string    `json:"store_name"`
	StoreSlug         string    `json:"store_slug"`
	StoreDescription  string    `json:"store_description"`
	Status            string    `json:"status"`
	LegalBusinessName string    `json:"legal_business_name"`
	BusinessType      string    `json:"business_type"`
	OwnerName         string    `json:"owner_name"`
	ContactEmail      string    `json:"contact_email"`
	ContactPhone      string    `json:"contact_phone"`
	GSTNumber         string    `json:"gst_number"`
	PANNumber         string    `json:"pan_number"`
	CommissionPercent *float64  `json:"commission_percent"`
	RatingAvg         float64   `json:"rating_avg"`
	RatingCount       int       `json:"rating_count"`
	RejectionReason   string    `json:"rejection_reason"`
	SuspendedReason   string    `json:"suspended_reason"`
	ResubmitCount     int       `json:"resubmit_count"`
	OnboardingDone    bool      `json:"onboarding_completed"`
	ContractSigned    bool      `json:"contract_signed"`
	ContractCommission *float64 `json:"contract_commission"`
	NotifyOrderEmails      bool `json:"notify_order_emails"`
	NotifyReturnEmails     bool `json:"notify_return_emails"`
	NotifySettlementEmails bool `json:"notify_settlement_emails"`
	NotifyLowStock         bool `json:"notify_low_stock"`
	NotifyMarketing        bool `json:"notify_marketing"`
	// CODEnabled controls whether Cash on Delivery is offered on this
	// seller's products. When false, COD is disabled at checkout for every
	// item from this seller.
	CODEnabled        bool      `json:"cod_enabled"`
	CreatedAt         time.Time `json:"created_at"`
}

// ProfileUpdate carries the fields a seller may edit about their own store.
type ProfileUpdate struct {
	StoreName        string `json:"store_name"`
	StoreDescription string `json:"store_description"`
	NotifyOrderEmails      bool `json:"notify_order_emails"`
	NotifyReturnEmails     bool `json:"notify_return_emails"`
	NotifySettlementEmails bool `json:"notify_settlement_emails"`
	NotifyLowStock         bool `json:"notify_low_stock"`
	NotifyMarketing        bool `json:"notify_marketing"`
	// CODEnabled is the resolved value written to the DB.
	CODEnabled bool `json:"cod_enabled"`
	// CODEnabledPtr carries the client's intent: nil means "leave unchanged",
	// so saving notification prefs without a COD field never flips the setting.
	CODEnabledPtr *bool `json:"-"`
}

// MaxResubmits is the number of times a rejected seller may resubmit their
// application before the account is permanently blocked.
const MaxResubmits = 3

// Onboarding is the seller's onboarding progress record.
type Onboarding struct {
	SellerID           string `json:"seller_id"`
	Step               string `json:"step"`
	BusinessCompleted  bool   `json:"business_completed"`
	ContactCompleted   bool   `json:"contact_completed"`
	PickupCompleted    bool   `json:"pickup_completed"`
	BankCompleted      bool   `json:"bank_completed"`
	DocumentsCompleted bool   `json:"documents_completed"`
	ContractCompleted  bool   `json:"contract_completed"`
	ReviewNotes        string `json:"review_notes"`
}

// BankAccount is a seller's payout bank account.
type BankAccount struct {
	AccountHolder string `json:"account_holder"`
	AccountNumber string `json:"account_number"`
	IFSC          string `json:"ifsc"`
	BankName      string `json:"bank_name"`
	Branch        string `json:"branch"`
}

// PickupAddress is a seller's order pickup / store location.
type PickupAddress struct {
	Label        string `json:"label"`
	ContactName  string `json:"contact_name"`
	ContactPhone string `json:"contact_phone"`
	Line1        string `json:"line1"`
	Line2        string `json:"line2"`
	City         string `json:"city"`
	State        string `json:"state"`
	Pincode      string `json:"pincode"`
	Country      string `json:"country"`
}

// OnboardingBundle is everything the seller-web needs to render onboarding
// with prefilled data (so previously entered details are never re-asked).
type OnboardingBundle struct {
	Seller     *Seller        `json:"seller"`
	Onboarding *Onboarding    `json:"onboarding"`
	Bank       *BankAccount   `json:"bank"`
	Pickup     *PickupAddress `json:"pickup"`
}
