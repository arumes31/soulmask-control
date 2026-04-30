package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type Authenticator struct {
	Password      string
	SessionCookie string
	TrustProxy    bool
}

func NewAuthenticator(password string, trustProxy bool) *Authenticator {
	return &Authenticator{
		Password:      password,
		SessionCookie: "soulmask_session",
		TrustProxy:    trustProxy,
	}
}

func (a *Authenticator) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Prevent timing attacks by using ConstantTimeCompare
	if subtle.ConstantTimeCompare([]byte(creds.Password), []byte(a.Password)) == 1 {
		// Hash password before storing in cookie to avoid plaintext leakage
		h := sha256.Sum256([]byte(a.Password))
		cookieValue := hex.EncodeToString(h[:])

		cookie := &http.Cookie{ // #nosec G124
			Name:     a.SessionCookie,
			Value:    cookieValue,
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

	h := sha256.Sum256([]byte(a.Password))
	expectedCookieValue := hex.EncodeToString(h[:])

	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(expectedCookieValue)) == 1
}
