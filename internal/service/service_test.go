package service

import "testing"

func TestMinutesHandlesOvernightTransfer(t *testing.T) {
	if got := minutes("23:58", "00:08"); got != 10 {
		t.Fatalf("minutes() = %d, want 10", got)
	}
}
