package service

import "testing"

func TestMinutesHandlesOvernightTransfer(t *testing.T) {
	if got := minutes("23:58", "00:08"); got != 10 {
		t.Fatalf("minutes() = %d, want 10", got)
	}
}

func TestMinutesHandlesSecondsFromKCI(t *testing.T) {
	if got := minutes("05:34:30", "06:02:15"); got != 27 {
		t.Fatalf("minutes() = %d, want 27", got)
	}
}
