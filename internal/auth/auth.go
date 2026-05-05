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

	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type Authenticator struct {
	Password      string
	SessionToken  string
	SessionCookie string
	TrustProxy    bool
	mu            sync.Mutex
	limiters      map[string]*clientLimiter
}

func NewAuthenticator(password string, trustProxy bool) *Authenticator {
	// Generate a secure random session token on startup
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Fatalf("Failed to generate session token: %v", err)
	}
	sessionToken := base64.URLEncoding.EncodeToString(tokenBytes)

	a := &Authenticator{
		Password:      password,
		SessionToken:  sessionToken,
		SessionCookie: "soulmask_session",
		TrustProxy:    trustProxy,
		limiters:      make(map[string]*clientLimiter),
	}

	// Start cleanup routine for old limiters
	go a.cleanupLimiters()

	return a
}

func (a *Authenticator) cleanupLimiters() {
	for {
		time.Sleep(10 * time.Minute)
		a.mu.Lock()
		for ip, cl := range a.limiters {
			if time.Since(cl.lastSeen) > 10*time.Minute {
				delete(a.limiters, ip)
			}
		}
		a.mu.Unlock()
	}
}

func (a *Authenticator) getLimiter(ip string) *rate.Limiter {
	a.mu.Lock()
	defer a.mu.Unlock()

	cl, exists := a.limiters[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Every(10*time.Second), 5)
		cl = &clientLimiter{limiter: limiter, lastSeen: time.Now()}
		a.limiters[ip] = cl
	} else {
		cl.lastSeen = time.Now()
	}

	return cl.limiter
}

func (a *Authenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// middleware IPMiddleware handles TrustProxy and sets r.RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !a.getLimiter(ip).Allow() {
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}

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
