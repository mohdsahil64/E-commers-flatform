// Package orders implements the order domain: creation from cart (with atomic
// inventory reservation), a validated state machine, and seller/customer/admin
// operations. Business rules live here, never in the frontend.
package orders

// Status is an order (or order item) lifecycle state.
type Status string

const (
	StatusPendingPayment  Status = "pending_payment"
	StatusConfirmed       Status = "confirmed"
	StatusProcessing      Status = "processing"
	StatusPacked          Status = "packed"
	StatusReadyToShip     Status = "ready_to_ship"
	StatusShipped         Status = "shipped"
	StatusInTransit       Status = "in_transit"
	StatusOutForDelivery  Status = "out_for_delivery"
	StatusDelivered       Status = "delivered"
	StatusCancelled       Status = "cancelled"
	StatusReturnRequested Status = "return_requested"
	StatusReturnApproved  Status = "return_approved"
	StatusReturnPicked    Status = "return_picked"
	StatusReturnReceived  Status = "return_received"
	StatusRefundProcessing Status = "refund_processing"
	StatusRefunded        Status = "refunded"
)

// transitions defines the allowed next states for each state. Any transition
// not listed here is rejected — arbitrary status changes are not permitted.
var transitions = map[Status][]Status{
	StatusPendingPayment: {StatusConfirmed, StatusCancelled},
	StatusConfirmed:      {StatusProcessing, StatusCancelled},
	StatusProcessing:     {StatusPacked, StatusCancelled},
	StatusPacked:         {StatusReadyToShip, StatusCancelled},
	StatusReadyToShip:    {StatusShipped, StatusCancelled},
	StatusShipped:        {StatusInTransit, StatusOutForDelivery, StatusDelivered},
	StatusInTransit:      {StatusOutForDelivery, StatusDelivered},
	StatusOutForDelivery: {StatusDelivered},
	// Post-delivery flows.
	StatusDelivered:        {StatusReturnRequested},
	StatusReturnRequested:  {StatusReturnApproved, StatusDelivered}, // reject => back to delivered
	StatusReturnApproved:   {StatusReturnPicked},
	StatusReturnPicked:     {StatusReturnReceived},
	StatusReturnReceived:   {StatusRefundProcessing},
	StatusRefundProcessing: {StatusRefunded},
	// Terminal states have no outgoing transitions.
	StatusCancelled: {},
	StatusRefunded:  {},
}

// CanTransition reports whether moving from -> to is allowed.
func CanTransition(from, to Status) bool {
	for _, next := range transitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// AllowedNext returns the valid next states for a given state.
func AllowedNext(from Status) []Status {
	return transitions[from]
}

// IsTerminal reports whether a status has no outgoing transitions.
func IsTerminal(s Status) bool {
	next, ok := transitions[s]
	return ok && len(next) == 0
}

// IsCancellable reports whether an order in this state may still be cancelled.
func IsCancellable(s Status) bool {
	return CanTransition(s, StatusCancelled)
}
