package billing

import "testing"

func TestParseStripeFeeRateBasisPoints(t *testing.T) {
	tests := []struct {
		value string
		want  int64
	}{
		{value: "0", want: 0},
		{value: "2.9", want: 290},
		{value: "2.90", want: 290},
		{value: "100.00", want: 10_000},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got, err := ParseStripeFeeRateBasisPoints(tt.value)
			if err != nil {
				t.Fatalf("ParseStripeFeeRateBasisPoints(%q) error = %v", tt.value, err)
			}
			if got != tt.want {
				t.Fatalf("ParseStripeFeeRateBasisPoints(%q) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseStripeFeeRateBasisPointsRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"", "-1", "100.01", "2.901", "1e1", ".5", "1."} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseStripeFeeRateBasisPoints(value); err == nil {
				t.Fatalf("ParseStripeFeeRateBasisPoints(%q) expected error", value)
			}
		})
	}
}

func TestCalculateStripeFeeAmountCentsRoundsHalfUp(t *testing.T) {
	tests := []struct {
		name            string
		baseAmountCents int64
		basisPoints     int64
		wantFeeAmount   int64
	}{
		{name: "three percent", baseAmountCents: 10_000, basisPoints: 300, wantFeeAmount: 300},
		{name: "two point nine percent", baseAmountCents: 1_001, basisPoints: 290, wantFeeAmount: 29},
		{name: "half cent rounds up", baseAmountCents: 1, basisPoints: 5_000, wantFeeAmount: 1},
		{name: "zero rate", baseAmountCents: 10_000, basisPoints: 0, wantFeeAmount: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculateStripeFeeAmountCents(tt.baseAmountCents, tt.basisPoints)
			if err != nil {
				t.Fatalf("calculateStripeFeeAmountCents() error = %v", err)
			}
			if got != tt.wantFeeAmount {
				t.Fatalf("calculateStripeFeeAmountCents() = %d, want %d", got, tt.wantFeeAmount)
			}
		})
	}
}
