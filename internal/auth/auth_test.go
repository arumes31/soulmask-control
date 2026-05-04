package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticator(t *testing.T) {
	password := "testpass"
	auth := NewAuthenticator(password, false)

	t.Run("Login success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"password": password})
		req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		auth.LoginHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		cookie := w.Result().Cookies()[0]
		if cookie.Name != auth.SessionCookie || cookie.Value != auth.SessionToken {
			t.Error("Cookie not set correctly")
		}
	})

	t.Run("Login failure", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"password": "wrong"})
		req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
		req.RemoteAddr = "192.168.1.1:12346" // Different IP to avoid rate limit from previous test
		w := httptest.NewRecorder()

		auth.LoginHandler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Login decode error", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/login", bytes.NewBufferString("invalid json"))
		req.RemoteAddr = "192.168.1.1:12347" // Different IP
		w := httptest.NewRecorder()

		auth.LoginHandler(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("IsAuthenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		if auth.IsAuthenticated(req) {
			t.Error("Should not be authenticated without cookie")
		}

		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: auth.SessionToken})
		if !auth.IsAuthenticated(req) {
			t.Error("Should be authenticated with correct cookie")
		}
	})

	t.Run("Rate limit", func(t *testing.T) {
		auth_rate := NewAuthenticator(password, false)

		for i := 0; i < 5; i++ {
			body, _ := json.Marshal(map[string]string{"password": "wrong"})
			req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
			req.RemoteAddr = "10.0.0.1:54321" // Consistent IP
			w := httptest.NewRecorder()

			auth_rate.LoginHandler(w, req)

			if i < 3 {
				if w.Code != http.StatusUnauthorized {
					t.Errorf("Expected status 401 for request %d, got %d", i, w.Code)
				}
			} else {
				if w.Code != http.StatusTooManyRequests {
					t.Errorf("Expected status 429 for request %d, got %d", i, w.Code)
				}
			}
		}
	})

	t.Run("Logout", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/logout", nil)
		w := httptest.NewRecorder()

		auth.LogoutHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		cookie := w.Result().Cookies()[0]
		if cookie.MaxAge != -1 {
			t.Error("Cookie should be expired on logout")
		}
	})
}

func TestTrustProxy(t *testing.T) {
	authProxy := NewAuthenticator("testpass", true)

	req1 := httptest.NewRequest("POST", "/login", nil)
	req1.RemoteAddr = "10.0.0.1:1234"
	req1.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")

	w1 := httptest.NewRecorder()
	authProxy.LoginHandler(w1, req1)

	authProxy.mu.Lock()
	_, exists := authProxy.limiters["192.168.1.100"]
	authProxy.mu.Unlock()

	if !exists {
		t.Error("Limiter should be created for X-Forwarded-For IP")
	}
}
