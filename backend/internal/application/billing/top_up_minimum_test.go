package billing

import "testing"

func TestParseTopUpMinimumAmountCents(t *testing.T) {
	tests := []struct {
		value string
		want  int64
	}{
		{value: "0", want: 0},
		{value: "0.01", want: 1},
		{value: "10", want: 1_000},
		{value: "10.5", want: 1_050},
		{value: "1000000.00", want: 100_000_000},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseTopUpMinimumAmountCents(tt.value)
			if err != nil {
				t.Fatalf("ParseTopUpMinimumAmountCents(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("ParseTopUpMinimumAmountCents(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseTopUpMinimumAmountCentsRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "-1", "0.001", "1000000.01", "1e2", ".5", "1."} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseTopUpMinimumAmountCents(value); err == nil {
				t.Fatalf("ParseTopUpMinimumAmountCents(%q) expected error", value)
			}
		})
	}
}

func TestTopUpMinimumAmountUSDFailsClosed(t *testing.T) {
	if got := TopUpMinimumAmountUSD(1_050); got != 10.5 {
		t.Fatalf("TopUpMinimumAmountUSD(1050) = %v, want 10.5", got)
	}
	for _, value := range []int64{-1, maxTopUpMinimumAmountCents + 1} {
		if got := TopUpMinimumAmountUSD(value); got != 0 {
			t.Fatalf("TopUpMinimumAmountUSD(%d) = %v, want 0", value, got)
		}
	}
}
