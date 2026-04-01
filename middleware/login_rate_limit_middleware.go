package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultLoginRateLimitWindow       = time.Minute
	defaultLoginRateLimitLockDuration = 5 * time.Minute
	defaultLoginRateLimitMaxFailures  = 10
)

type loginAttemptState struct {
	failedCount   int
	firstFailedAt time.Time
	lockedUntil   time.Time
}

type LoginAttemptLimiter struct {
	mu          sync.Mutex
	attempts    map[string]loginAttemptState
	maxFailures int
	window      time.Duration
	lockFor     time.Duration
	now         func() time.Time
}

func NewLoginAttemptLimiter(maxFailures int, window, lockFor time.Duration) *LoginAttemptLimiter {
	if maxFailures <= 0 {
		maxFailures = defaultLoginRateLimitMaxFailures
	}
	if window <= 0 {
		window = defaultLoginRateLimitWindow
	}
	if lockFor <= 0 {
		lockFor = defaultLoginRateLimitLockDuration
	}

	return &LoginAttemptLimiter{
		attempts:    make(map[string]loginAttemptState),
		maxFailures: maxFailures,
		window:      window,
		lockFor:     lockFor,
		now:         time.Now,
	}
}

func (l *LoginAttemptLimiter) RemainingLock(key string) time.Duration {
	if key == "" {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	state, ok := l.attempts[key]
	if !ok {
		return 0
	}

	now := l.now()
	if state.lockedUntil.After(now) {
		return state.lockedUntil.Sub(now)
	}

	if state.firstFailedAt.IsZero() || now.Sub(state.firstFailedAt) > l.window {
		delete(l.attempts, key)
	}

	return 0
}

func (l *LoginAttemptLimiter) RecordFailure(key string) {
	if key == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	state := l.attempts[key]
	if state.lockedUntil.After(now) {
		return
	}

	if state.firstFailedAt.IsZero() || now.Sub(state.firstFailedAt) > l.window {
		state = loginAttemptState{
			failedCount:   1,
			firstFailedAt: now,
		}
		l.attempts[key] = state
		return
	}

	state.failedCount++
	if state.failedCount >= l.maxFailures {
		state.failedCount = 0
		state.firstFailedAt = time.Time{}
		state.lockedUntil = now.Add(l.lockFor)
	}

	l.attempts[key] = state
}

func (l *LoginAttemptLimiter) Reset(key string) {
	if key == "" {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func NewLoginRateLimitMiddleware(limiter *LoginAttemptLimiter) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		clientKey := ctx.ClientIP()
		if remaining := limiter.RemainingLock(clientKey); remaining > 0 {
			ctx.Header("Retry-After", retryAfterHeaderValue(remaining))
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts"})
			return
		}

		ctx.Next()

		switch ctx.Writer.Status() {
		case http.StatusOK:
			limiter.Reset(clientKey)
		case http.StatusUnauthorized:
			limiter.RecordFailure(clientKey)
		}
	}
}

func retryAfterHeaderValue(d time.Duration) string {
	seconds := int(math.Ceil(d.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
