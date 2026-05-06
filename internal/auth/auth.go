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
	mu            sync.Mutex
	loginAttempts map[string]time.Time
}

func NewAuthenticator(password string, trustProxy bool) *Authenticator {
	// Generate a secure random session token on startup
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Fatalf("Failed to generate session token: %v", err)
	}
	sessionToken := base64.URLEncoding.EncodeToString(tokenBytes)

	auth := &Authenticator{
		Password:      password,
		SessionToken:  sessionToken,
		SessionCookie: "soulmask_session",
		TrustProxy:    trustProxy,
		loginAttempts: make(map[string]time.Time),
	}

	// Basic cleanup for rate limiting map to prevent memory leak
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			auth.mu.Lock()
			now := time.Now()
			for ip, t := range auth.loginAttempts {
				if now.Sub(t) > 5*time.Second {
					delete(auth.loginAttempts, ip)
				}
			}
			auth.mu.Unlock()
		}
	}()

	return auth
}

func (a *Authenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Rate Limiting Logic
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	a.mu.Lock()
	lastAttempt, exists := a.loginAttempts[ip]
	if exists && time.Since(lastAttempt) < 2*time.Second {
		a.loginAttempts[ip] = time.Now() // Update to prevent spamming from keeping it blocked indefinitely
		a.mu.Unlock()
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}
	a.loginAttempts[ip] = time.Now()
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
