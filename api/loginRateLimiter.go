package api

import (
	"crypto/sha256"
	"net"
	"sync"
	"time"
)

const (
	defaultLoginMaxFailures = 5
	defaultLoginBaseDelay   = time.Second
	defaultLoginMaxDelay    = 15 * time.Minute
	defaultLoginEntryTTL    = 30 * time.Minute
	defaultLoginMaxEntries  = 10_000
)

type loginRateLimiterConfig struct {
	MaxFailures int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	EntryTTL    time.Duration
	MaxEntries  int
	Now         func() time.Time
}

type loginRateLimitKey struct {
	clientIP     string
	usernameHash [sha256.Size]byte
}

type loginRateLimitEntry struct {
	failures    int
	retryAfter  time.Time
	lastUpdated time.Time
}

// loginRateLimiter tracks failed login attempts in process memory. It is keyed
// by both the client IP and a non-reversible username digest, so attempts for
// one account or client do not affect another.
type loginRateLimiter struct {
	mu      sync.Mutex
	config  loginRateLimiterConfig
	entries map[loginRateLimitKey]loginRateLimitEntry
}

func defaultLoginRateLimiterConfig() loginRateLimiterConfig {
	return loginRateLimiterConfig{
		MaxFailures: defaultLoginMaxFailures,
		BaseDelay:   defaultLoginBaseDelay,
		MaxDelay:    defaultLoginMaxDelay,
		EntryTTL:    defaultLoginEntryTTL,
		MaxEntries:  defaultLoginMaxEntries,
		Now:         time.Now,
	}
}

func newLoginRateLimiter(config loginRateLimiterConfig) *loginRateLimiter {
	if config.MaxFailures < 1 {
		config.MaxFailures = defaultLoginMaxFailures
	}
	if config.BaseDelay <= 0 {
		config.BaseDelay = defaultLoginBaseDelay
	}
	if config.MaxDelay < config.BaseDelay {
		config.MaxDelay = defaultLoginMaxDelay
	}
	if config.EntryTTL <= 0 {
		config.EntryTTL = defaultLoginEntryTTL
	}
	if config.MaxEntries < 1 {
		config.MaxEntries = defaultLoginMaxEntries
	}
	if config.Now == nil {
		config.Now = time.Now
	}

	return &loginRateLimiter{
		config:  config,
		entries: make(map[loginRateLimitKey]loginRateLimitEntry),
	}
}

// RetryAfter reports the current backoff duration for an IP/username pair.
// A zero duration means the pair may attempt a login now.
func (l *loginRateLimiter) RetryAfter(clientIP, username string) time.Duration {
	now := l.config.Now()
	key := newLoginRateLimitKey(clientIP, username)

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.entries[key]
	if !ok {
		return 0
	}
	if now.Sub(entry.lastUpdated) >= l.config.EntryTTL {
		delete(l.entries, key)
		return 0
	}
	if !now.Before(entry.retryAfter) {
		return 0
	}
	return entry.retryAfter.Sub(now)
}

func (l *loginRateLimiter) RecordFailure(clientIP, username string) {
	now := l.config.Now()
	key := newLoginRateLimitKey(clientIP, username)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.removeExpiredLocked(now)
	entry := l.entries[key]
	entry.failures++
	entry.lastUpdated = now
	if entry.failures >= l.config.MaxFailures {
		entry.retryAfter = now.Add(l.backoffDelay(entry.failures))
	}
	l.ensureCapacityLocked(key)
	l.entries[key] = entry
}

func (l *loginRateLimiter) Clear(clientIP, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, newLoginRateLimitKey(clientIP, username))
}

func newLoginRateLimitKey(clientIP, username string) loginRateLimitKey {
	// Canonicalizing valid IP input prevents equivalent IPv6 representations
	// from getting separate limiter entries. Invalid values remain isolated.
	if parsedIP := net.ParseIP(clientIP); parsedIP != nil {
		clientIP = parsedIP.String()
	}
	return loginRateLimitKey{
		clientIP:     clientIP,
		usernameHash: sha256.Sum256([]byte(username)),
	}
}

func (l *loginRateLimiter) backoffDelay(failures int) time.Duration {
	delay := l.config.BaseDelay
	for attempt := l.config.MaxFailures; attempt < failures && delay < l.config.MaxDelay; attempt++ {
		if delay > l.config.MaxDelay/2 {
			return l.config.MaxDelay
		}
		delay *= 2
	}
	if delay > l.config.MaxDelay {
		return l.config.MaxDelay
	}
	return delay
}

func (l *loginRateLimiter) removeExpiredLocked(now time.Time) {
	for key, entry := range l.entries {
		if now.Sub(entry.lastUpdated) >= l.config.EntryTTL {
			delete(l.entries, key)
		}
	}
}

func (l *loginRateLimiter) ensureCapacityLocked(exempt loginRateLimitKey) {
	if len(l.entries) < l.config.MaxEntries {
		return
	}

	var oldestKey loginRateLimitKey
	var oldest time.Time
	for key, entry := range l.entries {
		if key == exempt {
			continue
		}
		if oldest.IsZero() || entry.lastUpdated.Before(oldest) {
			oldestKey = key
			oldest = entry.lastUpdated
		}
	}
	if !oldest.IsZero() {
		delete(l.entries, oldestKey)
	}
}
