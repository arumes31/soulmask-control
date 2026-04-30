package auth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticator(t *testing.T) {
	password := "testpass"
	auth := NewAuthenticator(password, false)

	h := sha256.Sum256([]byte(password))
	expectedCookieValue := hex.EncodeToString(h[:])

	t.Run("Login success", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"password": password})
		req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		auth.LoginHandler(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		cookie := w.Result().Cookies()[0]
		if cookie.Name != auth.SessionCookie || cookie.Value != expectedCookieValue {
			t.Errorf("Cookie not set correctly, expected %s got %s", expectedCookieValue, cookie.Value)
		}
	})

	t.Run("Login failure", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"password": "wrong"})
		req := httptest.NewRequest("POST", "/login", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		auth.LoginHandler(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Login decode error", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/login", bytes.NewBufferString("invalid json"))
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

		req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: expectedCookieValue})
		if !auth.IsAuthenticated(req) {
			t.Error("Should be authenticated with correct cookie")
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
