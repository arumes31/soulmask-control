package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
)

type Authenticator struct {
	Password      string
	SessionCookie string
	TrustProxy    bool
	SessionToken  string
}

func NewAuthenticator(password string, trustProxy bool) *Authenticator {
	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)

	return &Authenticator{
		Password:      password,
		SessionCookie: "soulmask_session",
		TrustProxy:    trustProxy,
		SessionToken:  hex.EncodeToString(tokenBytes),
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
