package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type loginAttempt struct {
	count       int
	lockoutTo   time.Time
	lastAttempt time.Time
}

type Authenticator struct {
	Password      string
	SessionToken  string
	SessionCookie string
	TrustProxy    bool

	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func NewAuthenticator(password string, trustProxy bool) *Authenticator {
	// Generate a secure random session token on startup
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Fatalf("Failed to generate session token: %v", err)
	}
	sessionToken := base64.URLEncoding.EncodeToString(tokenBytes)

	return &Authenticator{
		Password:      password,
		SessionToken:  sessionToken,
		SessionCookie: "soulmask_session",
		TrustProxy:    trustProxy,
		attempts:      make(map[string]loginAttempt),
	}
}

func (a *Authenticator) getClientIP(r *http.Request) string {
	if a.TrustProxy {
		if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
			return cfIP
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			ips := strings.Split(xff, ",")
			return strings.TrimSpace(ips[0])
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (a *Authenticator) cleanupAttempts() {
	now := time.Now()
	for ip, attempt := range a.attempts {
		if now.After(attempt.lockoutTo) && now.Sub(attempt.lastAttempt) > 30*time.Minute {
			delete(a.attempts, ip)
		}
	}
}

func (a *Authenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ip := a.getClientIP(r)
	now := time.Now()

	a.mu.Lock()
	// Opportunistic cleanup
	if len(a.attempts) > 1000 {
		a.cleanupAttempts()
	}

	attempt := a.attempts[ip]
	if now.Before(attempt.lockoutTo) {
		a.mu.Unlock()
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}
	a.mu.Unlock()

	var creds struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Use ConstantTimeCompare to mitigate timing attacks
	if subtle.ConstantTimeCompare([]byte(creds.Password), []byte(a.Password)) == 1 {
		a.mu.Lock()
		delete(a.attempts, ip) // Reset attempts on success
		a.mu.Unlock()

		cookie := &http.Cookie{ // #nosec G124
			Name:     a.SessionCookie,
			Value:    a.SessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   a.TrustProxy,
			SameSite: http.SameSiteLaxMode,
		}
		http.SetCookie(w, cookie)
		w.WriteHeader(http.StatusOK)
		return
	}

	a.mu.Lock()
	attempt = a.attempts[ip] // Re-fetch to avoid race conditions
	attempt.count++
	attempt.lastAttempt = time.Now()
	if attempt.count >= 5 {
		attempt.lockoutTo = attempt.lastAttempt.Add(15 * time.Minute)
		attempt.count = 0 // Reset count but keep lockout
	}
	a.attempts[ip] = attempt
	a.mu.Unlock()

	http.Error(w, "Unauthorized", http.StatusUnauthorized)
}

func (a *Authenticator) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie := &http.Cookie{ // #nosec G124
		Name:     a.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   a.TrustProxy,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusOK)
}

func (a *Authenticator) IsAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(a.SessionCookie)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(a.SessionToken)) == 1
}
