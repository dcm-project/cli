package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

const clockSkew = 30 * time.Second

// AuthTransport is an http.RoundTripper that injects Bearer tokens into
// outgoing requests. It supports two modes:
//   - Static token: injected directly from DCM_TOKEN / --token with no
//     refresh logic.
//   - Stored token: loaded from a TokenStore, with automatic refresh when
//     the access token expires.
type AuthTransport struct {
	Base http.RoundTripper
	// RefreshTransport is used for OIDC token-endpoint calls during refresh.
	// When nil, Base is used. Set this when the issuer URL needs different
	// TLS settings than the control-plane URL (e.g. HTTP CP + HTTPS issuer).
	RefreshTransport http.RoundTripper
	Store            TokenStore
	IssuerURL        string
	StaticToken      string
	Stderr           *os.File
	mu               sync.Mutex
	warnOnce         sync.Once
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.StaticToken != "" {
		t.warnHTTP(req)
		req = cloneRequest(req)
		req.Header.Set("Authorization", "Bearer "+t.StaticToken)
		return t.base().RoundTrip(req)
	}

	if t.Store == nil || t.IssuerURL == "" {
		return t.base().RoundTrip(req)
	}

	tokenData, err := t.Store.Load(t.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("loading stored credentials: %w", err)
	}
	if tokenData == nil {
		return t.base().RoundTrip(req)
	}

	if !tokenData.IsExpired(clockSkew) {
		t.warnHTTP(req)
		req = cloneRequest(req)
		req.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
		return t.base().RoundTrip(req)
	}

	refreshed, err := t.refreshToken(req.Context(), tokenData)
	if err != nil {
		return nil, fmt.Errorf("authentication expired, run 'dcm login' to re-authenticate: %w", err)
	}

	t.warnHTTP(req)
	req = cloneRequest(req)
	req.Header.Set("Authorization", "Bearer "+refreshed.AccessToken)
	return t.base().RoundTrip(req)
}

func (t *AuthTransport) refreshToken(ctx context.Context, tokenData *TokenData) (*TokenData, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	reloaded, err := t.Store.Load(t.IssuerURL)
	if err != nil {
		return nil, err
	}
	if reloaded != nil && !reloaded.IsExpired(clockSkew) {
		return reloaded, nil
	}

	current := tokenData
	if reloaded != nil {
		current = reloaded
	}

	if current.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	oauthCfg := &oauth2.Config{
		ClientID: ClientID,
		Endpoint: oauth2.Endpoint{
			TokenURL: current.TokenEndpoint,
		},
	}

	oldToken := &oauth2.Token{
		RefreshToken: current.RefreshToken,
	}

	// Prefer RefreshTransport when set so issuer TLS (custom CA, mTLS) is
	// used even if Base was built for an HTTP control-plane URL. Do not wrap
	// with AuthTransport — that would re-enter RoundTrip while holding t.mu.
	refreshClient := &http.Client{Transport: t.refreshBase()}
	refreshCtx := context.WithValue(ctx, oauth2.HTTPClient, refreshClient)

	newToken, err := oauthCfg.TokenSource(refreshCtx, oldToken).Token()
	if err != nil {
		return nil, err
	}

	idToken, _ := newToken.Extra("id_token").(string)
	refreshed := &TokenData{
		AccessToken:   newToken.AccessToken,
		RefreshToken:  newToken.RefreshToken,
		IDToken:       idToken,
		Expiry:        newToken.Expiry,
		TokenEndpoint: current.TokenEndpoint,
	}

	// Prefer returning the refreshed token even if persist fails. With refresh
	// token rotation the IdP may have invalidated the old refresh token, so
	// discarding the new tokens would leave the session unrecoverable.
	if err := t.Store.Save(t.IssuerURL, refreshed); err != nil {
		t.warnPersist(err)
	}

	return refreshed, nil
}

func (t *AuthTransport) warnPersist(err error) {
	stderr := t.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	_, _ = fmt.Fprintf(stderr, "Warning: could not save refreshed credentials: %v\n", err)
}

func (t *AuthTransport) warnHTTP(req *http.Request) {
	if req.URL.Scheme != "http" {
		return
	}
	t.warnOnce.Do(func() {
		stderr := t.Stderr
		if stderr == nil {
			stderr = os.Stderr
		}
		_, _ = fmt.Fprintln(stderr, "Warning: sending Bearer token over unencrypted HTTP connection")
	})
}

func (t *AuthTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *AuthTransport) refreshBase() http.RoundTripper {
	if t.RefreshTransport != nil {
		return t.RefreshTransport
	}
	return t.base()
}

func cloneRequest(req *http.Request) *http.Request {
	r2 := req.Clone(req.Context())
	return r2
}
