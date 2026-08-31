package cache

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
)

var freeModelRateLimitScript = redis.NewScript(`
local minute_limit = tonumber(ARGV[3]) or 0
local daily_limit = tonumber(ARGV[5]) or 0

local minute_exceeded = 0
local daily_exceeded = 0

if minute_limit > 0 then
  local now = tonumber(ARGV[1])
  local window_ms = tonumber(ARGV[2])
  redis.call('ZREMRANGEBYSCORE', KEYS[1], '0', tostring(now - window_ms))
  local minute_count = tonumber(redis.call('ZCARD', KEYS[1])) or 0
  if minute_count >= minute_limit then
    minute_exceeded = 1
  end
end

if daily_limit > 0 then
  local daily_count = tonumber(redis.call('GET', KEYS[2]) or '0') or 0
  if daily_count >= daily_limit then
    daily_exceeded = 1
  end
end

if minute_exceeded == 1 or daily_exceeded == 1 then
  return {0, minute_exceeded, daily_exceeded}
end

if minute_limit > 0 then
  redis.call('ZADD', KEYS[1], tonumber(ARGV[1]), ARGV[7])
  redis.call('EXPIRE', KEYS[1], tonumber(ARGV[4]))
end

if daily_limit > 0 then
  redis.call('INCR', KEYS[2])
  redis.call('EXPIRE', KEYS[2], tonumber(ARGV[6]))
end

return {1, 0, 0}
`)

var slidingWindowRateLimitScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])

redis.call('ZREMRANGEBYSCORE', KEYS[1], '0', tostring(now - window_ms))
local count = tonumber(redis.call('ZCARD', KEYS[1])) or 0
redis.call('ZADD', KEYS[1], now, ARGV[5])
redis.call('PEXPIRE', KEYS[1], ttl_ms)

if count < limit then
  return 1
end
return 0
`)

var rateLimitMemberSequence uint64

// rateLimiter 提供基于 Redis 的 HTTP 限流存储能力。
type rateLimiter struct {
	client *redis.Client
}

// NewRateLimiter 创建 Redis 限流器。
func NewRateLimiter(client *redis.Client) *rateLimiter {
	if client == nil {
		return nil
	}
	return &rateLimiter{client: client}
}

// AllowSlidingWindow 使用有序集合实现滑动窗口限流。
func (r *rateLimiter) AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil || key == "" || limit <= 0 {
		return true, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	if ttl <= 0 {
		ttl = window * 2
	}

	nowNanos := time.Now().UnixNano()
	now := nowNanos / int64(time.Millisecond)
	member := fmt.Sprintf("%d:%d", nowNanos, atomic.AddUint64(&rateLimitMemberSequence, 1))
	allowed, err := slidingWindowRateLimitScript.Run(
		ctx,
		r.client,
		[]string{key},
		now,
		window.Milliseconds(),
		limit,
		ttl.Milliseconds(),
		member,
	).Int()
	if err != nil {
		return true, err
	}
	return allowed == 1, nil
}

// AllowFixedWindow 使用计数器实现固定窗口限流。
func (r *rateLimiter) AllowFixedWindow(ctx context.Context, keys []string, limit int, ttl time.Duration) (bool, error) {
	if r == nil || r.client == nil || len(keys) == 0 || limit <= 0 {
		return true, nil
	}
	if ttl <= 0 {
		ttl = time.Minute
	}

	pipe := r.client.Pipeline()
	incrCmds := make([]*redis.IntCmd, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		incrCmds = append(incrCmds, pipe.Incr(ctx, key))
		pipe.Expire(ctx, key, ttl)
	}
	if len(incrCmds) == 0 {
		return true, nil
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return true, err
	}

	for _, cmd := range incrCmds {
		if cmd.Val() > int64(limit) {
			return false, nil
		}
	}
	return true, nil
}

// AllowFreeModelUsage 原子检查并记录免费模型共享池的分钟与每日窗口。
func (r *rateLimiter) AllowFreeModelUsage(ctx context.Context, userID uint, requestsPerMinute int, dailyLimit int, now time.Time) (bool, bool, bool, error) {
	if r == nil || r.client == nil || userID == 0 || (requestsPerMinute <= 0 && dailyLimit <= 0) {
		return true, false, false, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	minuteKey := fmt.Sprintf("ratelimit:free-model:user:%d:minute", userID)
	dailyKey := fmt.Sprintf("ratelimit:free-model:user:%d:day:%s", userID, now.Format("20060102"))
	member := fmt.Sprintf("%d:%d", now.UnixMilli(), atomic.AddUint64(&rateLimitMemberSequence, 1))
	dailyTTL := secondsUntilNextLocalDay(now)
	raw, err := freeModelRateLimitScript.Run(
		ctx,
		r.client,
		[]string{minuteKey, dailyKey},
		now.UnixMilli(),
		time.Minute.Milliseconds(),
		requestsPerMinute,
		int((2 * time.Minute).Seconds()),
		dailyLimit,
		dailyTTL,
		member,
	).Result()
	if err != nil {
		return false, false, false, err
	}
	return parseFreeModelRateLimitResult(raw)
}

func parseFreeModelRateLimitResult(raw interface{}) (bool, bool, bool, error) {
	values, ok := raw.([]interface{})
	if !ok || len(values) < 3 {
		return false, false, false, fmt.Errorf("invalid free model rate limit response")
	}
	allowed := redisScriptInt(values[0]) == 1
	minuteExceeded := redisScriptInt(values[1]) == 1
	dailyExceeded := redisScriptInt(values[2]) == 1
	return allowed, minuteExceeded, dailyExceeded, nil
}

func secondsUntilNextLocalDay(now time.Time) int {
	nextDay := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	ttl := int(nextDay.Sub(now).Seconds())
	if ttl < 60 {
		return 60
	}
	return ttl
}

func redisScriptInt(value interface{}) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}
