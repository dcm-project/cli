package auth_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/auth"
)

func mockOIDCServer(pollsBeforeSuccess int) *httptest.Server {
	var pollCount atomic.Int32

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		baseURL := "http://" + r.Host
		discovery := map[string]any{
			"issuer":                                baseURL,
			"authorization_endpoint":                baseURL + "/protocol/openid-connect/auth",
			"token_endpoint":                        baseURL + "/protocol/openid-connect/token",
			"device_authorization_endpoint":         baseURL + "/protocol/openid-connect/auth/device",
			"revocation_endpoint":                   baseURL + "/protocol/openid-connect/revoke",
			"jwks_uri":                              baseURL + "/protocol/openid-connect/certs",
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(discovery)
	})

	mux.HandleFunc("/protocol/openid-connect/auth/device", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		Expect(r.FormValue("client_id")).To(Equal(auth.ClientID))
		resp := map[string]any{
			"device_code":               "test-device-code",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          "http://" + r.Host + "/device",
			"verification_uri_complete": "http://" + r.Host + "/device?user_code=ABCD-EFGH",
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

		grantType := r.FormValue("grant_type")

		if grantType == "refresh_token" {
			exp := time.Now().Add(5 * time.Minute)
			resp := map[string]any{
				"access_token":  makeJWT(exp),
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
		if count <= pollsBeforeSuccess {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "authorization_pending",
			})
			return
		}

		exp := time.Now().Add(5 * time.Minute)
		accessToken := makeJWTWithUsername(exp, "dcm-admin")
		resp := map[string]any{
			"access_token":  accessToken,
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
		Expect(r.FormValue("client_id")).To(Equal(auth.ClientID))
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	})

	return httptest.NewServer(mux)
}

func makeJWTWithUsername(exp time.Time, username string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"exp":%d,"sub":"test-user","preferred_username":"%s"}`, exp.Unix(), username)
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	return header + "." + payload + "." + sig
}

var _ = Describe("DeviceLogin", func() {
	var (
		server *httptest.Server
		output *bytes.Buffer
	)

	AfterEach(func() {
		if server != nil {
			server.Close()
			server = nil
		}
	})

	It("completes the device flow and returns token data", func() {
		server = mockOIDCServer(1)
		output = new(bytes.Buffer)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		td, err := auth.DeviceLogin(ctx, server.URL, server.Client(), output)
		Expect(err).NotTo(HaveOccurred())
		Expect(td).NotTo(BeNil())
		Expect(td.AccessToken).NotTo(BeEmpty())
		Expect(td.RefreshToken).To(Equal("test-refresh-token"))
		Expect(td.IDToken).To(Equal("test-id-token"))
		Expect(td.TokenEndpoint).To(ContainSubstring("/protocol/openid-connect/token"))

		Expect(output.String()).To(ContainSubstring("ABCD-EFGH"))
		Expect(output.String()).To(ContainSubstring("/device"))
	})

	It("fails when OIDC discovery fails", func() {
		badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer badServer.Close()

		output = new(bytes.Buffer)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := auth.DeviceLogin(ctx, badServer.URL, badServer.Client(), output)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("OIDC discovery failed"))
	})
})

var _ = Describe("ClientID", func() {
	It("is the hardcoded public client dcm-cli", func() {
		Expect(auth.ClientID).To(Equal("dcm-cli"))
	})
})

var _ = Describe("browserCommand", func() {
	It("accepts http and https URLs", func() {
		for _, raw := range []string{
			"https://keycloak.example.com/device",
			"http://localhost:8080/device?user_code=ABCD",
		} {
			cmd, err := auth.BrowserCommand(raw)
			Expect(err).NotTo(HaveOccurred(), raw)
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Args).To(ContainElement(raw))
		}
	})

	It("rejects non-http schemes", func() {
		for _, raw := range []string{
			"file:///etc/passwd",
			"javascript:alert(1)",
			"cmd://calc",
		} {
			_, err := auth.BrowserCommand(raw)
			Expect(err).To(HaveOccurred(), raw)
			Expect(err.Error()).To(ContainSubstring("unsupported browser URL scheme"))
		}
	})

	It("rejects URLs without an http(s) scheme", func() {
		_, err := auth.BrowserCommand("not a url")
		Expect(err).To(HaveOccurred())
	})

	It("uses rundll32 on Windows so cmd.exe does not parse the URL", func() {
		raw := `https://evil.example/x&calc`
		cmd, err := auth.BrowserCommandFor("windows", raw)
		Expect(err).NotTo(HaveOccurred())
		Expect(cmd.Args).To(Equal([]string{
			"rundll32",
			"url.dll,FileProtocolHandler",
			raw,
		}))
	})
})

var _ = Describe("PreferredUsername", func() {
	It("extracts preferred_username from a valid JWT", func() {
		token := makeJWTWithUsername(time.Now().Add(5*time.Minute), "dcm-admin")
		Expect(auth.PreferredUsername(token)).To(Equal("dcm-admin"))
	})

	It("returns empty string for invalid JWT", func() {
		Expect(auth.PreferredUsername("not-a-jwt")).To(BeEmpty())
	})
})

var _ = Describe("RevokeToken", func() {
	It("sends revocation request to the provider", func() {
		server := mockOIDCServer(0)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := auth.RevokeToken(ctx, server.URL, "test-refresh-token", server.Client())
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns nil when the provider has no revocation_endpoint", func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
			baseURL := "http://" + r.Host
			discovery := map[string]any{
				"issuer":                                baseURL,
				"authorization_endpoint":                baseURL + "/protocol/openid-connect/auth",
				"token_endpoint":                        baseURL + "/protocol/openid-connect/token",
				"jwks_uri":                              baseURL + "/protocol/openid-connect/certs",
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(discovery)
		})
		mux.HandleFunc("/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := auth.RevokeToken(ctx, server.URL, "test-refresh-token", server.Client())
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns an error when the revocation endpoint responds with 4xx", func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
			baseURL := "http://" + r.Host
			discovery := map[string]any{
				"issuer":                                baseURL,
				"authorization_endpoint":                baseURL + "/protocol/openid-connect/auth",
				"token_endpoint":                        baseURL + "/protocol/openid-connect/token",
				"revocation_endpoint":                   baseURL + "/protocol/openid-connect/revoke",
				"jwks_uri":                              baseURL + "/protocol/openid-connect/certs",
				"subject_types_supported":               []string{"public"},
				"id_token_signing_alg_values_supported": []string{"RS256"},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(discovery)
		})
		mux.HandleFunc("/protocol/openid-connect/revoke", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		})
		mux.HandleFunc("/protocol/openid-connect/certs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
		})
		server := httptest.NewServer(mux)
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := auth.RevokeToken(ctx, server.URL, "test-refresh-token", server.Client())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("token revocation failed with status 400"))
	})
})
