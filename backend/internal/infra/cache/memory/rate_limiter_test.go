package memory

import (
	"context"
	"testing"
	"time"
)

func TestAllowFreeModelUsageRequiresBothWindowsBeforeIncrementing(t *testing.T) {
	cache := New()
	ctx := context.Background()
	now := time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)

	allowed, minuteExceeded, dailyExceeded, err := cache.AllowFreeModelUsage(ctx, 7, 1, 5, now)
	if err != nil || !allowed || minuteExceeded || dailyExceeded {
		t.Fatalf("first minute request = (%v,%v,%v,%v), want (true,false,false,nil)", allowed, minuteExceeded, dailyExceeded, err)
	}

	allowed, minuteExceeded, dailyExceeded, err = cache.AllowFreeModelUsage(ctx, 7, 1, 5, now.Add(10*time.Second))
	if err != nil || allowed || !minuteExceeded || dailyExceeded {
		t.Fatalf("second minute request = (%v,%v,%v,%v), want (false,true,false,nil)", allowed, minuteExceeded, dailyExceeded, err)
	}

	allowed, minuteExceeded, dailyExceeded, err = cache.AllowFreeModelUsage(ctx, 8, 5, 1, now)
	if err != nil || !allowed || minuteExceeded || dailyExceeded {
		t.Fatalf("first daily request = (%v,%v,%v,%v), want (true,false,false,nil)", allowed, minuteExceeded, dailyExceeded, err)
	}

	allowed, minuteExceeded, dailyExceeded, err = cache.AllowFreeModelUsage(ctx, 8, 5, 1, now.Add(10*time.Second))
	if err != nil || allowed || minuteExceeded || !dailyExceeded {
		t.Fatalf("second daily request = (%v,%v,%v,%v), want (false,false,true,nil)", allowed, minuteExceeded, dailyExceeded, err)
	}
}
