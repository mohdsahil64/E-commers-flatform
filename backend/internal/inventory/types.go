package inventory

import "time"

// Transaction is a single ledger entry (view model).
type Transaction struct {
	ChangeType    string    `json:"change_type"`
	QuantityDelta int       `json:"quantity_delta"`
	BalanceAfter  int       `json:"balance_after"`
	ReferenceType string    `json:"reference_type"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}
