package oauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testClientID     = "client-id"
	testClientSecret = "client-secret"
	testCallbackURL  = "callback-url"
)

func TestNewService(t *testing.T) {
	service := NewService(testClientID, testClientSecret, testCallbackURL)
	
	if service.config.ClientID != testClientID {
		t.Errorf("NewService() ClientID = %v, want %v", service.config.ClientID, testClientID)
	}

	if service.config.ClientSecret != testClientSecret {
		t.Errorf("NewService() ClientSecret = %v, want %v", service.config.ClientSecret, testClientSecret)
	}
}

func TestPreAuth(t *testing.T) {
	service := NewService(testClientID, testClientSecret, testCallbackURL)
	
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/auth", nil)
	
	service.PreAuth(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("PreAuth() status = %v, want %v", resp.StatusCode, http.StatusTemporaryRedirect)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Errorf("PreAuth() location = %v, want to contain github.com/login/oauth/authorize", location)
	}

	cookies := resp.Cookies()
	var stateCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == stateCookieName {
			stateCookie = cookie
			break
		}
	}

	if stateCookie == nil {
		t.Error("PreAuth() state cookie not set")
	}
}
