package billing

import (
	"errors"
	"strconv"
	"strings"
)

const maxTopUpMinimumAmountCents int64 = 100_000_000

// ParseTopUpMinimumAmountCents parses a USD amount with at most two decimal
// places into cents. Zero disables the provider-specific minimum.
func ParseTopUpMinimumAmountCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("top-up minimum amount is required")
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || !decimalDigits(parts[0]) {
		return 0, errors.New("top-up minimum amount must be a decimal USD amount")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" || len(fraction) > 2 || !decimalDigits(fraction) {
			return 0, errors.New("top-up minimum amount supports at most two decimal places")
		}
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole < 0 {
		return 0, errors.New("top-up minimum amount must be between 0 and 1000000")
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	fractionValue := int64(0)
	if fraction != "" {
		fractionValue, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, errors.New("top-up minimum amount must be a decimal USD amount")
		}
	}
	if whole > maxTopUpMinimumAmountCents/100 {
		return 0, errors.New("top-up minimum amount must be between 0 and 1000000")
	}
	amountCents := whole*100 + fractionValue
	if amountCents < 0 || amountCents > maxTopUpMinimumAmountCents {
		return 0, errors.New("top-up minimum amount must be between 0 and 1000000")
	}
	return amountCents, nil
}

// TopUpMinimumAmountUSD converts a valid stored minimum in cents to USD.
// Invalid stored values safely fall back to zero.
func TopUpMinimumAmountUSD(amountCents int64) float64 {
	if amountCents < 0 || amountCents > maxTopUpMinimumAmountCents {
		return 0
	}
	return float64(amountCents) / 100
}
