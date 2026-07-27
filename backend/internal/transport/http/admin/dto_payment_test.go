package admin

import (
	"testing"

	appadmin "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/admin"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

func TestPaymentOrderResponseIncludesStripeFeeBreakdown(t *testing.T) {
	response := toPaymentOrderResponse(domainbilling.PaymentOrder{
		PayCurrency:        "USD",
		PayAmountCents:     10_300,
		FeeRateBasisPoints: 300,
		FeeAmountCents:     300,
	}, appadmin.UserLabel{})

	if response.PaySubtotalAmountCents != 10_000 ||
		response.FeeRatePercent != 3 ||
		response.FeeAmountCents != 300 ||
		response.PayAmountCents != 10_300 {
		t.Fatalf("unexpected admin payment fee response: %+v", response)
	}
}
