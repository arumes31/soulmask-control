package middleware

import (
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Login rate-limiting parameters. The limit is deliberately strict because the
// /login endpoint authenticates against a single shared admin secret, which
// makes brute-force and credential-stuffing attacks cheap if left unbounded.
const (
	// loginRate caps sustained login attempts per client to 1 per second.
	loginRate = rate.Limit(1)
	// loginBurst absorbs short bursts (e.g. a quick retry) before throttling.
	loginBurst = 5
	// visitorTTL is how long an idle client's limiter is retained before it is
	// eligible for eviction by the cleanup loop.
	visitorTTL = 3 * time.Minute
	// cleanupInterval is how often stale limiters are swept from memory.
	cleanupInterval = time.Minute
)

// visitor pairs a per-client token-bucket limiter with its last-seen time so
// idle entries can be evicted, preventing unbounded memory growth (a DoS vector
// for naive in-memory limiters).
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	visitors = make(map[string]*visitor)
	mu       sync.Mutex
)

func init() {
	go cleanupVisitors()
}

// cleanupVisitors periodically evicts limiters for clients that have been idle
// longer than visitorTTL.
func cleanupVisitors() {
	for {
		time.Sleep(cleanupInterval)
		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > visitorTTL {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

// getVisitor returns the rate limiter for ip, creating one on first contact.
func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(loginRate, loginBurst)
		visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// LoginRateLimitAllow reports whether a login attempt from remoteAddr is within
// the configured rate limit. remoteAddr is expected to already reflect the true
// client IP (IPMiddleware normalizes r.RemoteAddr from trusted proxy headers
// before routing). The port is stripped so all attempts from one host share a
// single limiter.
func LoginRateLimitAllow(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// remoteAddr may be a bare IP without a port; fall back to it as-is.
		host = remoteAddr
	}
	return getVisitor(host).Allow()
}
