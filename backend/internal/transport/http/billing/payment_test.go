package billing

import (
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

func TestResolveTopUpCheckoutAmountUsesUSDForStripe(t *testing.T) {
	req := CreateCheckoutRequest{AmountMinorUnits: 10_000}
	settings := billingPaymentSettings{DisplayCurrency: "CNY"}

	stripeAmount, stripeCurrency := resolveTopUpCheckoutAmount(req, settings, domainbilling.PaymentProviderStripe)
	if stripeAmount != 10_000 || stripeCurrency != "USD" {
		t.Fatalf("stripe amount = %d %s, want 10000 USD", stripeAmount, stripeCurrency)
	}
	epayAmount, epayCurrency := resolveTopUpCheckoutAmount(req, settings, domainbilling.PaymentProviderEPay)
	if epayAmount != 10_000 || epayCurrency != "CNY" {
		t.Fatalf("epay amount = %d %s, want 10000 CNY", epayAmount, epayCurrency)
	}
	zeroStripeAmount, zeroStripeCurrency := resolveTopUpCheckoutAmount(CreateCheckoutRequest{}, settings, domainbilling.PaymentProviderStripe)
	if zeroStripeAmount != 0 || zeroStripeCurrency != "USD" {
		t.Fatalf("empty stripe amount = %d %s, want 0 USD", zeroStripeAmount, zeroStripeCurrency)
	}
}

func TestParseStripeFeeRateBasisPointsFailsClosed(t *testing.T) {
	if got := parseStripeFeeRateBasisPoints("2.9"); got != 290 {
		t.Fatalf("parseStripeFeeRateBasisPoints(2.9) = %d, want 290", got)
	}
	for _, value := range []string{"", "2.901", "101", "bad"} {
		if got := parseStripeFeeRateBasisPoints(value); got != 0 {
			t.Fatalf("parseStripeFeeRateBasisPoints(%q) = %d, want 0", value, got)
		}
	}
}

func TestParseTopUpMinimumAmountCentsFailsOpenToNoMinimum(t *testing.T) {
	if got := parseTopUpMinimumAmountCents("10.25"); got != 1_025 {
		t.Fatalf("parseTopUpMinimumAmountCents(10.25) = %d, want 1025", got)
	}
	for _, value := range []string{"", "-1", "10.001", "1000000.01", "bad"} {
		if got := parseTopUpMinimumAmountCents(value); got != 0 {
			t.Fatalf("parseTopUpMinimumAmountCents(%q) = %d, want 0", value, got)
		}
	}
}

func TestMinimumTopUpAmountCentsUsesSelectedProvider(t *testing.T) {
	settings := billingPaymentSettings{
		StripeMinimumTopUpCents: 1_000,
		EPayMinimumTopUpCents:   2_000,
	}
	if got := minimumTopUpAmountCents(settings, domainbilling.PaymentProviderStripe); got != 1_000 {
		t.Fatalf("Stripe minimum = %d, want 1000", got)
	}
	if got := minimumTopUpAmountCents(settings, domainbilling.PaymentProviderEPay); got != 2_000 {
		t.Fatalf("EPay minimum = %d, want 2000", got)
	}
}

func TestValidateStripeCheckoutSessionChecksFeeInclusiveAmountAndCurrency(t *testing.T) {
	order := &domainbilling.PaymentOrder{
		Provider:           domainbilling.PaymentProviderStripe,
		ExternalCheckoutID: "cs_fee_order",
		PayCurrency:        "USD",
		PayAmountCents:     10_300,
	}
	session := stripeCheckoutSession{
		ID:          "cs_fee_order",
		AmountTotal: 10_300,
		Currency:    "usd",
	}
	if err := validateStripeCheckoutSession(order, session); err != nil {
		t.Fatalf("validateStripeCheckoutSession() error = %v", err)
	}

	session.AmountTotal = 10_000
	if err := validateStripeCheckoutSession(order, session); err == nil || err.Error() != "amount mismatch" {
		t.Fatalf("amount mismatch error = %v", err)
	}
	session.AmountTotal = 10_300
	session.Currency = "cny"
	if err := validateStripeCheckoutSession(order, session); err == nil || err.Error() != "currency mismatch" {
		t.Fatalf("currency mismatch error = %v", err)
	}
}
