package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
)

type freeModelRateLimitRepositoryStub struct {
	*billingRepositoryStub
	limit domainbilling.FreeModelRateLimit
}

func (r *freeModelRateLimitRepositoryStub) GetFreeModelRateLimit(context.Context) (domainbilling.FreeModelRateLimit, error) {
	return r.limit, nil
}

type failingFreeModelRateLimiter struct {
	err error
}

func (r failingFreeModelRateLimiter) AllowFreeModelUsage(context.Context, uint, int, int, time.Time) (bool, bool, bool, error) {
	return false, false, false, r.err
}

func TestEnforceFreeModelRateLimitFailsClosedWhenLimiterUnavailable(t *testing.T) {
	limiterErr := errors.New("rate limiter unavailable")
	service := NewService(&freeModelRateLimitRepositoryStub{
		billingRepositoryStub: &billingRepositoryStub{},
		limit: domainbilling.FreeModelRateLimit{
			RequestsPerMinute: 1,
			DailyRequests:     10,
		},
	})
	service.SetFreeModelRateLimiter(failingFreeModelRateLimiter{err: limiterErr})

	err := service.enforceFreeModelRateLimit(context.Background(), 42, "free-model", time.Now())
	if !errors.Is(err, limiterErr) {
		t.Fatalf("expected limiter error to fail closed, got %v", err)
	}
}
