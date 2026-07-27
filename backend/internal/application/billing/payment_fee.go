package billing

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

const maxStripeFeeRateBasisPoints int64 = 10_000

// ParseStripeFeeRateBasisPoints parses a percentage with at most two decimal
// places into integer basis points. One basis point equals 0.01%.
func ParseStripeFeeRateBasisPoints(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("stripe fee rate is required")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || !decimalDigits(parts[0]) {
		return 0, errors.New("stripe fee rate must be a decimal percentage")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" || len(fraction) > 2 || !decimalDigits(fraction) {
			return 0, errors.New("stripe fee rate supports at most two decimal places")
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 || whole > 100 {
		return 0, errors.New("stripe fee rate must be between 0 and 100")
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	fractionValue := int64(0)
	if fraction != "" {
		fractionValue, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("stripe fee rate must be a decimal percentage")
		}
	}
	basisPoints := whole*100 + fractionValue
	if basisPoints < 0 || basisPoints > maxStripeFeeRateBasisPoints {
		return 0, errors.New("stripe fee rate must be between 0 and 100")
	}
	return basisPoints, nil
}

// StripeFeeRatePercent converts stored basis points into a percentage for API
// responses. Invalid stored values fail closed to zero.
func StripeFeeRatePercent(basisPoints int64) float64 {
	return float64(normalizeStripeFeeRateBasisPoints(basisPoints)) / 100
}

func normalizeStripeFeeRateBasisPoints(value int64) int64 {
	if value < 0 || value > maxStripeFeeRateBasisPoints {
		return 0
	}
	return value
}

func calculateStripeFeeAmountCents(baseAmountCents int64, basisPoints int64) (int64, error) {
	if baseAmountCents < 0 {
		return 0, errors.New("base payment amount must be non-negative")
	}
	basisPoints = normalizeStripeFeeRateBasisPoints(basisPoints)
	if baseAmountCents == 0 || basisPoints == 0 {
		return 0, nil
	}

	// Split the multiplication to avoid overflowing for otherwise valid int64
	// monetary amounts. Adding half the denominator implements round-half-up.
	feeAmountCents := (baseAmountCents/10_000)*basisPoints +
		((baseAmountCents%10_000)*basisPoints+5_000)/10_000
	if feeAmountCents > math.MaxInt64-baseAmountCents {
		return 0, errors.New("payment amount is too large")
	}
	return feeAmountCents, nil
}

func decimalDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}
