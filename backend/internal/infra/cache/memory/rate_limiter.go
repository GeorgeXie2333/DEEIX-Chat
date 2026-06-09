package memory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type fixedWindowCounter struct {
	count     int
	expiresAt time.Time
}

func (c *Cache) AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error) {
	if c == nil || strings.TrimSpace(key) == "" || limit <= 0 {
		return true, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	now := time.Now()
	cutoff := now.Add(-window)
	c.mu.Lock()
	defer c.mu.Unlock()
	events := c.slidingHTTP[key][:0]
	for _, item := range c.slidingHTTP[key] {
		if item.After(cutoff) {
			events = append(events, item)
		}
	}
	allowed := len(events) < limit
	if allowed {
		events = append(events, now)
	}
	c.slidingHTTP[key] = events
	c.maybeSweepLocked(now)
	return allowed, nil
}

func (c *Cache) AllowFixedWindow(ctx context.Context, keys []string, limit int, ttl time.Duration) (bool, error) {
	if c == nil || len(keys) == 0 || limit <= 0 {
		return true, nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	allowed := true
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		item := c.fixedHTTP[key]
		if now.After(item.expiresAt) {
			item = fixedWindowCounter{expiresAt: now.Add(ttl)}
		}
		item.count++
		if item.count > limit {
			allowed = false
		}
		c.fixedHTTP[key] = item
	}
	c.maybeSweepLocked(now)
	return allowed, nil
}

func (c *Cache) AllowFreeModelUsage(ctx context.Context, userID uint, requestsPerMinute int, dailyLimit int, now time.Time) (bool, bool, bool, error) {
	if c == nil || userID == 0 || (requestsPerMinute <= 0 && dailyLimit <= 0) {
		return true, false, false, nil
	}
	if now.IsZero() {
		now = time.Now()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	minuteExceeded := false
	if requestsPerMinute > 0 {
		cutoff := now.Add(-time.Minute)
		events := c.freeModelMinute[userID][:0]
		for _, item := range c.freeModelMinute[userID] {
			if item.After(cutoff) {
				events = append(events, item)
			}
		}
		c.freeModelMinute[userID] = events
		minuteExceeded = len(events) >= requestsPerMinute
	}

	dailyExceeded := false
	dailyKey := ""
	if dailyLimit > 0 {
		dailyKey = freeModelDailyKey(userID, now)
		item := c.freeModelDaily[dailyKey]
		if now.After(item.expiresAt) {
			item = fixedWindowCounter{expiresAt: now.Add(time.Duration(secondsUntilNextLocalDay(now)) * time.Second)}
			c.freeModelDaily[dailyKey] = item
		}
		dailyExceeded = item.count >= dailyLimit
	}

	if minuteExceeded || dailyExceeded {
		c.maybeSweepLocked(now)
		return false, minuteExceeded, dailyExceeded, nil
	}

	if requestsPerMinute > 0 {
		c.freeModelMinute[userID] = append(c.freeModelMinute[userID], now)
	}
	if dailyLimit > 0 {
		item := c.freeModelDaily[dailyKey]
		if now.After(item.expiresAt) {
			item = fixedWindowCounter{expiresAt: now.Add(time.Duration(secondsUntilNextLocalDay(now)) * time.Second)}
		}
		item.count++
		c.freeModelDaily[dailyKey] = item
	}
	c.maybeSweepLocked(now)
	return true, false, false, nil
}

func freeModelDailyKey(userID uint, now time.Time) string {
	return fmt.Sprintf("%d:%s", userID, now.Format("20060102"))
}

func secondsUntilNextLocalDay(now time.Time) int {
	nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	ttl := int(nextDay.Sub(now).Seconds())
	if ttl < 60 {
		return 60
	}
	return ttl
}
