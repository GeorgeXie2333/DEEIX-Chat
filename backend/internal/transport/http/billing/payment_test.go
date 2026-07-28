package billing

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	"github.com/gin-gonic/gin"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

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

func TestCreateStripeCheckoutSessionUsesSingleUSDFeeInclusiveLineItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var posted url.Values
	previousClient := stripeHTTPClient
	stripeHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read stripe request body: %v", err)
		}
		posted, err = url.ParseQuery(string(raw))
		if err != nil {
			t.Fatalf("parse stripe request body: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"cs_test","url":"https://checkout.stripe.test/session"}`)),
			Header:     make(http.Header),
		}, nil
	})}
	t.Cleanup(func() {
		stripeHTTPClient = previousClient
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "http://api.example.test/api/v1/billing/payments/checkout", nil)
	c.Request.Header.Set("Origin", "http://web.example.test")

	h := &Handler{}
	order := &domainbilling.PaymentOrder{
		OrderNo:            "pay_fee_checkout",
		OrderType:          domainbilling.PaymentOrderTypeTopUp,
		UserID:             1,
		Provider:           domainbilling.PaymentProviderStripe,
		BaseCurrency:       "USD",
		BaseAmountCents:    10_000,
		PayCurrency:        "USD",
		PayAmountCents:     10_300,
		FeeRateBasisPoints: 300,
		FeeAmountCents:     300,
		FXRate:             "1",
		CreditNanousd:      100_000_000_000,
	}
	checkoutID, checkoutURL, err := h.createStripeCheckoutSession(
		c,
		billingPaymentSettings{StripeSecretKey: "sk_test"},
		order,
		nil,
		nil,
		CreateCheckoutRequest{},
	)
	if err != nil {
		t.Fatalf("createStripeCheckoutSession() error = %v", err)
	}
	if checkoutID != "cs_test" || checkoutURL != "https://checkout.stripe.test/session" {
		t.Fatalf("checkout = %q %q", checkoutID, checkoutURL)
	}
	if posted.Get("line_items[0][price_data][currency]") != "usd" {
		t.Fatalf("stripe currency = %q, want usd", posted.Get("line_items[0][price_data][currency]"))
	}
	if posted.Get("line_items[0][price_data][unit_amount]") != "10300" {
		t.Fatalf("stripe amount = %q, want 10300", posted.Get("line_items[0][price_data][unit_amount]"))
	}
	if posted.Get("line_items[1][price_data][unit_amount]") != "" {
		t.Fatal("unexpected second Stripe line item")
	}
	if posted.Get("metadata[pay_subtotal_amount_cents]") != "10000" ||
		posted.Get("metadata[fee_rate_basis_points]") != "300" ||
		posted.Get("metadata[fee_rate_percent]") != "3" ||
		posted.Get("metadata[fee_amount_cents]") != "300" {
		t.Fatalf("unexpected fee metadata: %v", posted)
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
