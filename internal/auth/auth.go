// Package auth implements OIDC device authorization flow, token storage, and
// authenticated HTTP transport for the DCM CLI.
package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const ClientID = "dcm-cli"

var scopes = []string{oidc.ScopeOpenID, "profile", "email", "offline_access"}

// DeviceLogin performs the OAuth 2.0 Device Authorization Grant (RFC 8628)
// against the given OIDC issuer. It prints the verification URL and user code
// to w, attempts to open a browser, and polls until the user completes
// authentication.
func DeviceLogin(ctx context.Context, issuerURL string, httpClient *http.Client, w io.Writer) (*TokenData, error) {
	oidcCtx := oidc.ClientContext(ctx, httpClient)

	provider, err := oidc.NewProvider(oidcCtx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery failed for %s: %w", issuerURL, err)
	}

	endpoint := provider.Endpoint()
	oauthCfg := &oauth2.Config{
		ClientID: ClientID,
		Endpoint: endpoint,
		Scopes:   scopes,
	}

	devAuth, err := oauthCfg.DeviceAuth(oidcCtx)
	if err != nil {
		return nil, fmt.Errorf("device authorization request failed: %w", err)
	}

	openURL := devAuth.VerificationURI
	if devAuth.VerificationURIComplete != "" {
		openURL = devAuth.VerificationURIComplete
	}

	if _, err := fmt.Fprintf(w, "Open %s in your browser\n", openURL); err != nil {
		return nil, fmt.Errorf("writing login output: %w", err)
	}
	if devAuth.VerificationURIComplete != "" {
		if _, err := fmt.Fprintf(w, "Or visit %s and enter code: %s\n", devAuth.VerificationURI, devAuth.UserCode); err != nil {
			return nil, fmt.Errorf("writing login output: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(w, "Enter code: %s\n", devAuth.UserCode); err != nil {
			return nil, fmt.Errorf("writing login output: %w", err)
		}
	}

	_ = openBrowser(openURL)

	token, err := oauthCfg.DeviceAccessToken(oidcCtx, devAuth)
	if err != nil {
		return nil, fmt.Errorf("device authorization failed: %w", err)
	}

	idToken, _ := token.Extra("id_token").(string)

	return &TokenData{
		AccessToken:   token.AccessToken,
		RefreshToken:  token.RefreshToken,
		IDToken:       idToken,
		Expiry:        token.Expiry,
		TokenEndpoint: endpoint.TokenURL,
	}, nil
}

// RevokeToken revokes the given refresh token at the OIDC provider's
// revocation endpoint.
func RevokeToken(ctx context.Context, issuerURL string, refreshToken string, httpClient *http.Client) error {
	oidcCtx := oidc.ClientContext(ctx, httpClient)

	provider, err := oidc.NewProvider(oidcCtx, issuerURL)
	if err != nil {
		return fmt.Errorf("OIDC discovery failed for %s: %w", issuerURL, err)
	}

	var metadata struct {
		RevocationEndpoint string `json:"revocation_endpoint"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return fmt.Errorf("reading provider metadata: %w", err)
	}
	if metadata.RevocationEndpoint == "" {
		return nil
	}

	data := url.Values{
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
		"client_id":       {ClientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, metadata.RevocationEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("creating revocation request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("revocation request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("token revocation failed with status %d", resp.StatusCode)
	}

	return nil
}

// PreferredUsername extracts the preferred_username claim from the access
// token's JWT payload without signature verification.
func PreferredUsername(accessToken string) string {
	parts := strings.SplitN(accessToken, ".", 3)
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		PreferredUsername string `json:"preferred_username"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return claims.PreferredUsername
}

func openBrowser(rawURL string) error {
	cmd, err := browserCommand(rawURL)
	if err != nil {
		return err
	}
	return cmd.Start()
}

// browserCommand builds the OS-specific command used to open a URL.
// Only http and https schemes are allowed. On Windows it uses rundll32
// instead of cmd.exe to avoid shell metacharacter injection.
func browserCommand(rawURL string) (*exec.Cmd, error) {
	return browserCommandFor(runtime.GOOS, rawURL)
}

func browserCommandFor(goos, rawURL string) (*exec.Cmd, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid browser URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported browser URL scheme %q", u.Scheme)
	}

	switch goos {
	case "darwin":
		return exec.Command("open", rawURL), nil
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL), nil
	default:
		return exec.Command("xdg-open", rawURL), nil
	}
}
