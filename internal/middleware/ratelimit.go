package middleware

import (
	"net"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	visitors = make(map[string]*Visitor)
	mu       sync.Mutex
)

// init starts a background goroutine to clean up stale visitors
func init() {
	go cleanupVisitors()
}

func cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

// getVisitor returns the rate limiter for the provided IP address.
func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[ip]
	if !exists {
		// Limit to 1 request per second with a burst of 5
		limiter := rate.NewLimiter(1, 5)
		v = &Visitor{limiter: limiter, lastSeen: time.Now()}
		visitors[ip] = v
		return v.limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// LoginRateLimitAllow checks if the login attempt from the given remote address should be allowed.
// It extracts the IP address from the remote address, which may include a port.
func LoginRateLimitAllow(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// If SplitHostPort fails, fall back to using the raw remoteAddr.
		// This can happen if remoteAddr is just an IP without a port.
		host = remoteAddr
	}
	limiter := getVisitor(host)
	return limiter.Allow()
}
