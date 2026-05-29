package cache

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

type fakeRedisLimiterServer struct {
	listener net.Listener
	done     chan struct{}
	wg       sync.WaitGroup

	mu      sync.Mutex
	minutes map[string]map[string]int64
	daily   map[string]int
}

func newFakeRedisLimiterServer(t *testing.T) *fakeRedisLimiterServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake redis: %v", err)
	}
	server := &fakeRedisLimiterServer{
		listener: listener,
		done:     make(chan struct{}),
		minutes:  make(map[string]map[string]int64),
		daily:    make(map[string]int),
	}
	server.wg.Add(1)
	go server.serve()
	t.Cleanup(server.close)
	return server
}

func (s *fakeRedisLimiterServer) addr() string {
	return s.listener.Addr().String()
}

func (s *fakeRedisLimiterServer) close() {
	select {
	case <-s.done:
		return
	default:
		close(s.done)
		_ = s.listener.Close()
		s.wg.Wait()
	}
}

func (s *fakeRedisLimiterServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
				return
			}
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}
}

func (s *fakeRedisLimiterServer) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		args, err := readRESPArray(reader)
		if err != nil {
			return
		}
		if len(args) == 0 {
			_, _ = conn.Write([]byte("-ERR empty command\r\n"))
			continue
		}
		switch strings.ToLower(args[0]) {
		case "ping":
			_, _ = conn.Write([]byte("+PONG\r\n"))
		case "evalsha", "eval":
			allowed, minuteExceeded, dailyExceeded := s.applyFreeModelScript(args)
			_, _ = conn.Write([]byte(respIntArray(allowed, minuteExceeded, dailyExceeded)))
		default:
			_, _ = conn.Write([]byte("+OK\r\n"))
		}
	}
}

func (s *fakeRedisLimiterServer) applyFreeModelScript(args []string) (int, int, int) {
	if len(args) < 12 {
		return 1, 0, 0
	}
	numKeys, err := strconv.Atoi(args[2])
	if err != nil || numKeys != 2 || len(args) < 3+numKeys+7 {
		return 1, 0, 0
	}

	minuteKey := args[3]
	dailyKey := args[4]
	argStart := 3 + numKeys
	nowMillis := parseInt64(args[argStart])
	windowMillis := parseInt64(args[argStart+1])
	requestsPerMinute := int(parseInt64(args[argStart+2]))
	dailyLimit := int(parseInt64(args[argStart+4]))
	member := args[argStart+6]

	s.mu.Lock()
	defer s.mu.Unlock()

	minuteExceeded := 0
	dailyExceeded := 0
	if requestsPerMinute > 0 {
		windowStart := nowMillis - windowMillis
		for existingMember, score := range s.minutes[minuteKey] {
			if score <= windowStart {
				delete(s.minutes[minuteKey], existingMember)
			}
		}
		if len(s.minutes[minuteKey]) >= requestsPerMinute {
			minuteExceeded = 1
		}
	}
	if dailyLimit > 0 && s.daily[dailyKey] >= dailyLimit {
		dailyExceeded = 1
	}
	if minuteExceeded == 1 || dailyExceeded == 1 {
		return 0, minuteExceeded, dailyExceeded
	}
	if requestsPerMinute > 0 {
		if s.minutes[minuteKey] == nil {
			s.minutes[minuteKey] = make(map[string]int64)
		}
		s.minutes[minuteKey][member] = nowMillis
	}
	if dailyLimit > 0 {
		s.daily[dailyKey]++
	}
	return 1, 0, 0
}

func readRESPArray(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	if !strings.HasPrefix(line, "*") {
		return nil, io.ErrUnexpectedEOF
	}
	count, err := strconv.Atoi(strings.TrimPrefix(line, "*"))
	if err != nil {
		return nil, err
	}
	args := make([]string, 0, count)
	for i := 0; i < count; i++ {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		header = strings.TrimSuffix(strings.TrimSuffix(header, "\n"), "\r")
		if !strings.HasPrefix(header, "$") {
			return nil, io.ErrUnexpectedEOF
		}
		length, err := strconv.Atoi(strings.TrimPrefix(header, "$"))
		if err != nil {
			return nil, err
		}
		if length < 0 {
			args = append(args, "")
			continue
		}
		payload := make([]byte, length+2)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
		args = append(args, string(payload[:length]))
	}
	return args, nil
}

func respIntArray(values ...int) string {
	var builder strings.Builder
	builder.WriteString("*")
	builder.WriteString(strconv.Itoa(len(values)))
	builder.WriteString("\r\n")
	for _, value := range values {
		builder.WriteString(":")
		builder.WriteString(strconv.Itoa(value))
		builder.WriteString("\r\n")
	}
	return builder.String()
}

func parseInt64(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func TestAllowFreeModelUsageRequiresBothWindowsBeforeIncrementing(t *testing.T) {
	server := newFakeRedisLimiterServer(t)
	client := redis.NewClient(&redis.Options{Addr: server.addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})
	limiter := NewRateLimiter(client)
	ctx := context.Background()
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.Local)

	allowed, minuteExceeded, dailyExceeded, err := limiter.AllowFreeModelUsage(ctx, 7, 1, 5, now)
	if err != nil {
		t.Fatalf("allow first request: %v", err)
	}
	if !allowed || minuteExceeded || dailyExceeded {
		t.Fatalf("expected first request to pass, got allowed=%v minute=%v daily=%v", allowed, minuteExceeded, dailyExceeded)
	}
	allowed, minuteExceeded, dailyExceeded, err = limiter.AllowFreeModelUsage(ctx, 7, 1, 5, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("limit second request: %v", err)
	}
	if allowed || !minuteExceeded || dailyExceeded {
		t.Fatalf("expected minute limit only, got allowed=%v minute=%v daily=%v", allowed, minuteExceeded, dailyExceeded)
	}

	minuteKey := "ratelimit:free-model:user:7:minute"
	dailyKey := "ratelimit:free-model:user:7:day:20260529"
	server.mu.Lock()
	if got := server.daily[dailyKey]; got != 1 {
		t.Fatalf("daily counter changed after minute denial: got %d want 1", got)
	}
	if got := len(server.minutes[minuteKey]); got != 1 {
		t.Fatalf("minute window changed after minute denial: got %d want 1", got)
	}
	server.mu.Unlock()

	allowed, minuteExceeded, dailyExceeded, err = limiter.AllowFreeModelUsage(ctx, 8, 5, 1, now)
	if err != nil {
		t.Fatalf("allow daily seed request: %v", err)
	}
	if !allowed || minuteExceeded || dailyExceeded {
		t.Fatalf("expected daily seed request to pass, got allowed=%v minute=%v daily=%v", allowed, minuteExceeded, dailyExceeded)
	}
	allowed, minuteExceeded, dailyExceeded, err = limiter.AllowFreeModelUsage(ctx, 8, 5, 1, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("limit daily request: %v", err)
	}
	if allowed || minuteExceeded || !dailyExceeded {
		t.Fatalf("expected daily limit only, got allowed=%v minute=%v daily=%v", allowed, minuteExceeded, dailyExceeded)
	}

	minuteKey = "ratelimit:free-model:user:8:minute"
	dailyKey = "ratelimit:free-model:user:8:day:20260529"
	server.mu.Lock()
	if got := server.daily[dailyKey]; got != 1 {
		t.Fatalf("daily counter changed after daily denial: got %d want 1", got)
	}
	if got := len(server.minutes[minuteKey]); got != 1 {
		t.Fatalf("minute window changed after daily denial: got %d want 1", got)
	}
	server.mu.Unlock()
}
