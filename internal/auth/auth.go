package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type Authenticator struct {
	Password      string
	SessionToken  string
	SessionCookie string
	TrustProxy    bool

	mu       sync.Mutex
	attempts map[string]int
	lastSeen map[string]time.Time
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
		attempts:      make(map[string]int),
		lastSeen:      make(map[string]time.Time),
	}
}

func (a *Authenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	a.mu.Lock()
	now := time.Now()

	// Basic cleanup to prevent memory leak
	if len(a.attempts) > 1000 {
		for k, v := range a.lastSeen {
			if now.Sub(v) > 15*time.Minute {
				delete(a.attempts, k)
				delete(a.lastSeen, k)
			}
		}
	}

	if now.Sub(a.lastSeen[ip]) > 15*time.Minute {
		a.attempts[ip] = 0
	}

	if a.attempts[ip] >= 5 {
		a.mu.Unlock()
		http.Error(w, "Too many login attempts", http.StatusTooManyRequests)
		return
	}
	a.lastSeen[ip] = now
	a.attempts[ip]++ // Increment early to prevent TOCTOU concurrent brute force
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
		delete(a.attempts, ip)
		delete(a.lastSeen, ip)
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
