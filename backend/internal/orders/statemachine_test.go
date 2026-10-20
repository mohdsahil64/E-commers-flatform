package orders

import "testing"

func TestValidForwardTransitions(t *testing.T) {
	path := []Status{
		StatusPendingPayment, StatusConfirmed, StatusProcessing, StatusPacked,
		StatusReadyToShip, StatusShipped, StatusInTransit, StatusOutForDelivery,
		StatusDelivered,
	}
	for i := 0; i < len(path)-1; i++ {
		if !CanTransition(path[i], path[i+1]) {
			t.Errorf("expected %s -> %s to be allowed", path[i], path[i+1])
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	cases := [][2]Status{
		{StatusPendingPayment, StatusShipped},   // can't skip to shipped
		{StatusDelivered, StatusCancelled},      // can't cancel after delivery
		{StatusRefunded, StatusConfirmed},       // terminal
		{StatusShipped, StatusCancelled},        // can't cancel once shipped
	}
	for _, c := range cases {
		if CanTransition(c[0], c[1]) {
			t.Errorf("expected %s -> %s to be rejected", c[0], c[1])
		}
	}
}

func TestReturnFlow(t *testing.T) {
	path := []Status{
		StatusDelivered, StatusReturnRequested, StatusReturnApproved,
		StatusReturnPicked, StatusReturnReceived, StatusRefundProcessing, StatusRefunded,
	}
	for i := 0; i < len(path)-1; i++ {
		if !CanTransition(path[i], path[i+1]) {
			t.Errorf("expected return flow %s -> %s", path[i], path[i+1])
		}
	}
}

func TestTerminalAndCancellable(t *testing.T) {
	if !IsTerminal(StatusRefunded) || !IsTerminal(StatusCancelled) {
		t.Error("refunded and cancelled should be terminal")
	}
	if IsCancellable(StatusDelivered) {
		t.Error("delivered should not be cancellable")
	}
	if !IsCancellable(StatusConfirmed) {
		t.Error("confirmed should be cancellable")
	}
}
