package auth

import (
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

	if creds.Password == a.Password {
		cookie := &http.Cookie{
			Name:     a.SessionCookie,
			Value:    a.Password,
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
	cookie := &http.Cookie{
		Name:     a.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, cookie)
	w.WriteHeader(http.StatusOK)
}

func (a *Authenticator) IsAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(a.SessionCookie)
	return err == nil && cookie.Value == a.Password
}
