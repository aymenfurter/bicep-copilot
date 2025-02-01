package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

const (
	stateCookieName = "oauth_state"
	pkceCodeName    = "pkce_code"
)

type Service struct {
	config     *oauth2.Config
	states     sync.Map
	pkceCodes  sync.Map
	maxAge     time.Duration
	cleanupMtx sync.Mutex
}

func NewService(clientID, clientSecret, callbackURL string) *Service {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Scopes:       []string{"read:user"},
		Endpoint:     github.Endpoint,
	}

	s := &Service{
		config: config,
		maxAge: 10 * time.Minute,
	}

	go s.cleanupRoutine()

	return s
}

func (s *Service) PreAuth(w http.ResponseWriter, r *http.Request) {
	state, err := generateRandomString(32)
	if err != nil {
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	codeVerifier, err := generateRandomString(64)
	if err != nil {
		http.Error(w, "Failed to generate code verifier", http.StatusInternalServerError)
		return
	}

	s.states.Store(state, time.Now())
	s.pkceCodes.Store(state, codeVerifier)

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   int(s.maxAge.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	hash := sha256.Sum256([]byte(codeVerifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}

	authURL := s.config.AuthCodeURL(state, opts...)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

func (s *Service) PostAuth(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")

	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		http.Error(w, "State cookie not found", http.StatusBadRequest)
		return
	}

	if cookie.Value != state {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	codeVerifierI, ok := s.pkceCodes.LoadAndDelete(state)
	if !ok {
		http.Error(w, "Invalid or expired session", http.StatusBadRequest)
		return
	}
	codeVerifier := codeVerifierI.(string)

	ctx := context.Background()
	_, err = s.config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to exchange token: %v", err), http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	s.states.Delete(state)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `
		<!DOCTYPE html>
		<html>
			<head>
				<title>Authorization Complete</title>
				<style>
					body { font-family: system-ui, -apple-system, sans-serif; text-align: center; padding-top: 2rem; }
				</style>
			</head>
			<body>
				<h1>Authorization Complete</h1>
				<p>You can now close this window and return to using the Copilot extension.</p>
				<script>
					setTimeout(() => window.close(), 3000);
				</script>
			</body>
		</html>
	`)
}

func (s *Service) cleanupRoutine() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.cleanupMtx.Lock()
		now := time.Now()

		s.states.Range(func(k, v interface{}) bool {
			if timestamp, ok := v.(time.Time); ok {
				if now.Sub(timestamp) > s.maxAge {
					s.states.Delete(k)
					s.pkceCodes.Delete(k)
				}
			}
			return true
		})

		s.cleanupMtx.Unlock()
	}
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes)[:length], nil
}