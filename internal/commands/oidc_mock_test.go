package commands_test

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

// mockOIDCOptions configures optional revoke behavior for the test OIDC server.
type mockOIDCOptions struct {
	pollsBeforeSuccess int
	revokeStatus       int // 0 means 200 OK; non-zero uses that status
	omitRevokeEndpoint bool
	rejectRefresh      bool
}

func mockOIDCServer(opts mockOIDCOptions) *httptest.Server {
	return httptest.NewServer(mockOIDCHandler(opts))
}

func mockOIDCTLSServer(opts mockOIDCOptions, tlsCfg *tls.Config) *httptest.Server {
	server := httptest.NewUnstartedServer(mockOIDCHandler(opts))
	server.TLS = tlsCfg
	server.StartTLS()
	return server
}

func mockOIDCHandler(opts mockOIDCOptions) http.Handler {
	var pollCount atomic.Int32
	revokeStatus := opts.revokeStatus
	if revokeStatus == 0 {
		revokeStatus = http.StatusOK
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		baseURL := requestBaseURL(r)
		discovery := map[string]any{
			"issuer":                                baseURL,
			"authorization_endpoint":                baseURL + "/protocol/openid-connect/auth",
			"token_endpoint":                        baseURL + "/protocol/openid-connect/token",
			"device_authorization_endpoint":         baseURL + "/protocol/openid-connect/auth/device",
			"jwks_uri":                              baseURL + "/protocol/openid-connect/certs",
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		if !opts.omitRevokeEndpoint {
			discovery["revocation_endpoint"] = baseURL + "/protocol/openid-connect/revoke"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	})

	mux.HandleFunc("/protocol/openid-connect/auth/device", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		baseURL := requestBaseURL(r)
		resp := map[string]any{
			"device_code":               "test-device-code",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          baseURL + "/device",
			"verification_uri_complete": baseURL + "/device?user_code=ABCD-EFGH",
			"expires_in":                600,
			"interval":                  0,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.FormValue("grant_type") == "refresh_token" {
			if opts.rejectRefresh {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			exp := time.Now().Add(5 * time.Minute)
			resp := map[string]any{
				"access_token":  makeTestJWT(exp, "dcm-admin"),
				"refresh_token": "new-refresh-token",
				"token_type":    "Bearer",
				"expires_in":    300,
				"id_token":      "new-id-token",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		count := int(pollCount.Add(1))
		if count <= opts.pollsBeforeSuccess {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "authorization_pending",
			})
			return
		}

		exp := time.Now().Add(5 * time.Minute)
		resp := map[string]any{
			"access_token":  makeTestJWT(exp, "dcm-admin"),
			"refresh_token": "test-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    300,
			"id_token":      "test-id-token",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/protocol/openid-connect/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(revokeStatus)
	})

	mux.HandleFunc("/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	})

	return mux
}

func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func makeTestJWT(exp time.Time, username string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"exp":%d,"sub":"test-user","preferred_username":"%s"}`, exp.Unix(), username)
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	return header + "." + payload + "." + sig
}
