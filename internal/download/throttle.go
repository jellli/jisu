package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxThrottleRetries = 5
const maxRetryDelay = 30 * time.Second

type throttleKey struct{}

type requestThrottle struct {
	mu      sync.Mutex
	changed chan struct{}
	active  int
	limited bool
	until   time.Time
}

func newRequestThrottle() *requestThrottle {
	return &requestThrottle{changed: make(chan struct{})}
}

func retryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseUint(value, 10, 64); err == nil {
		return time.Duration(min(seconds, uint64(maxRetryDelay/time.Second))) * time.Second
	}
	if date, err := http.ParseTime(value); err == nil {
		return min(max(date.Sub(now), 0), maxRetryDelay)
	}
	return 0
}

func waitThrottle(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t *requestThrottle) acquire(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		t.mu.Lock()
		delay := time.Until(t.until)
		if delay <= 0 && (!t.limited || t.active == 0) {
			t.active++
			t.mu.Unlock()
			return nil
		}
		changed := t.changed
		t.mu.Unlock()
		if delay > 0 {
			if err := waitThrottle(ctx, delay); err != nil {
				return err
			}
		} else {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-changed:
			}
		}
	}
}

func (t *requestThrottle) release() {
	t.mu.Lock()
	t.active--
	close(t.changed)
	t.changed = make(chan struct{})
	t.mu.Unlock()
}

type throttleBody struct {
	io.ReadCloser
	once    sync.Once
	release func()
}

func (b *throttleBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.release)
	return err
}

func (t *requestThrottle) request(ctx context.Context, q *Queue, rawURL, span, validator string, options ...RequestOptions) (*http.Response, error) {
	for retries := 0; ; retries++ {
		if err := t.acquire(ctx); err != nil {
			return nil, err
		}
		resp, err := q.requestOnce(ctx, rawURL, span, validator, options...)
		if err != nil {
			t.release()
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
			resp.Body = &throttleBody{ReadCloser: resp.Body, release: t.release}
			return resp, nil
		}
		delay := time.Second << retries
		if q.retryDelay != nil {
			delay = q.retryDelay(retries)
		}
		delay = min(maxRetryDelay, max(delay, retryAfter(resp.Header.Get("Retry-After"), time.Now())))
		t.mu.Lock()
		t.limited = true
		t.until = maxTime(t.until, time.Now().Add(delay))
		t.mu.Unlock()
		resp.Body.Close()
		t.release()
		if retries >= maxThrottleRetries {
			return nil, fmt.Errorf("server returned HTTP %d after %d throttle retries", resp.StatusCode, retries)
		}
	}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
