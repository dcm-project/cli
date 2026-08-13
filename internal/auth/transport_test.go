package auth_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/auth"
)

type failSaveStore struct {
	auth.TokenStore
	err error
}

func (s *failSaveStore) Save(_ string, _ *auth.TokenData) error {
	return s.err
}

// reloadEndpointStore returns staleEndpoint on the first Load and
// currentEndpoint on later Loads, simulating another refresh updating
// the store between the outer RoundTrip load and the locked reload.
type reloadEndpointStore struct {
	auth.TokenStore
	loads           atomic.Int32
	staleEndpoint   string
	currentEndpoint string
	baseTD          *auth.TokenData
}

func (s *reloadEndpointStore) Load(_ string) (*auth.TokenData, error) {
	n := s.loads.Add(1)
	td := *s.baseTD
	if n == 1 {
		td.TokenEndpoint = s.staleEndpoint
	} else {
		td.TokenEndpoint = s.currentEndpoint
	}
	return &td, nil
}

type countingRoundTripper struct {
	base      http.RoundTripper
	hits      *atomic.Int32
	matchHost string
}

func (t *countingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Host == t.matchHost {
		t.hits.Add(1)
	}
	return t.base.RoundTrip(req)
}

var _ = Describe("AuthTransport", func() {
	var (
		backend      *httptest.Server
		storeDir     string
		store        auth.TokenStore
		receivedAuth string
	)

	BeforeEach(func() {
		receivedAuth = ""
		backend = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))

		storeDir = GinkgoT().TempDir()
		store = auth.NewFileStoreWithDir(storeDir)
	})

	AfterEach(func() {
		if backend != nil {
			backend.Close()
		}
	})

	Describe("Static token", func() {
		It("injects the static Bearer token", func() {
			transport := &auth.AuthTransport{
				Base:        http.DefaultTransport,
				StaticToken: "my-static-token",
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(receivedAuth).To(Equal("Bearer my-static-token"))
		})
	})

	Describe("Stored token (valid)", func() {
		It("injects the stored access token", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(store.Save("http://keycloak:8080/realms/dcm", td)).To(Succeed())

			transport := &auth.AuthTransport{
				Base:      http.DefaultTransport,
				Store:     store,
				IssuerURL: "http://keycloak:8080/realms/dcm",
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(receivedAuth).To(HavePrefix("Bearer "))
			Expect(receivedAuth).To(Equal("Bearer " + td.AccessToken))
		})
	})

	Describe("No stored token", func() {
		It("passes through without Authorization header", func() {
			transport := &auth.AuthTransport{
				Base:      http.DefaultTransport,
				Store:     store,
				IssuerURL: "http://keycloak:8080/realms/dcm",
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(receivedAuth).To(BeEmpty())
		})
	})

	Describe("Expired token with refresh", func() {
		It("refreshes and injects the new access token", func() {
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.FormValue("grant_type") == "refresh_token" {
					newExp := time.Now().Add(5 * time.Minute)
					resp := map[string]any{
						"access_token":  makeJWT(newExp),
						"refresh_token": "new-refresh-token",
						"token_type":    "Bearer",
						"expires_in":    300,
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
					return
				}
				http.Error(w, "unexpected request", http.StatusBadRequest)
			}))
			defer tokenServer.Close()

			expiredTD := &auth.TokenData{
				AccessToken:   makeJWT(time.Now().Add(-1 * time.Minute)),
				RefreshToken:  "old-refresh-token",
				Expiry:        time.Now().Add(-1 * time.Minute),
				TokenEndpoint: tokenServer.URL,
			}
			Expect(store.Save("http://keycloak:8080/realms/dcm", expiredTD)).To(Succeed())

			transport := &auth.AuthTransport{
				Base:      http.DefaultTransport,
				Store:     store,
				IssuerURL: "http://keycloak:8080/realms/dcm",
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(receivedAuth).To(HavePrefix("Bearer "))
			Expect(receivedAuth).NotTo(Equal("Bearer " + expiredTD.AccessToken))

			reloaded, err := store.Load("http://keycloak:8080/realms/dcm")
			Expect(err).NotTo(HaveOccurred())
			Expect(reloaded.RefreshToken).To(Equal("new-refresh-token"))
		})

		It("persists TokenEndpoint from the reloaded token under the lock", func() {
			currentEndpoint := ""
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.FormValue("grant_type") == "refresh_token" {
					newExp := time.Now().Add(5 * time.Minute)
					resp := map[string]any{
						"access_token":  makeJWT(newExp),
						"refresh_token": "new-refresh-token",
						"token_type":    "Bearer",
						"expires_in":    300,
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
					return
				}
				http.Error(w, "unexpected request", http.StatusBadRequest)
			}))
			defer tokenServer.Close()
			currentEndpoint = tokenServer.URL

			baseTD := &auth.TokenData{
				AccessToken:  makeJWT(time.Now().Add(-1 * time.Minute)),
				RefreshToken: "old-refresh-token",
				Expiry:       time.Now().Add(-1 * time.Minute),
			}
			wrapped := &reloadEndpointStore{
				TokenStore:      store,
				staleEndpoint:   "http://stale.example/token",
				currentEndpoint: currentEndpoint,
				baseTD:          baseTD,
			}

			transport := &auth.AuthTransport{
				Base:      http.DefaultTransport,
				Store:     wrapped,
				IssuerURL: "http://keycloak:8080/realms/dcm",
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()

			saved, err := store.Load("http://keycloak:8080/realms/dcm")
			Expect(err).NotTo(HaveOccurred())
			Expect(saved).NotTo(BeNil())
			Expect(saved.TokenEndpoint).To(Equal(currentEndpoint))
		})
	})

	Describe("Expired token with failed refresh", func() {
		It("returns an actionable error message", func() {
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			}))
			defer tokenServer.Close()

			expiredTD := &auth.TokenData{
				AccessToken:   makeJWT(time.Now().Add(-1 * time.Minute)),
				RefreshToken:  "expired-refresh-token",
				Expiry:        time.Now().Add(-1 * time.Minute),
				TokenEndpoint: tokenServer.URL,
			}
			Expect(store.Save("http://keycloak:8080/realms/dcm", expiredTD)).To(Succeed())

			transport := &auth.AuthTransport{
				Base:      http.DefaultTransport,
				Store:     store,
				IssuerURL: "http://keycloak:8080/realms/dcm",
			}
			client := &http.Client{Transport: transport}

			_, err := client.Get(backend.URL)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("dcm login"))
		})
	})

	Describe("Refresh uses Base transport", func() {
		It("sends the refresh request through AuthTransport.Base", func() {
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.FormValue("grant_type") == "refresh_token" {
					newExp := time.Now().Add(5 * time.Minute)
					resp := map[string]any{
						"access_token":  makeJWT(newExp),
						"refresh_token": "new-refresh-token",
						"token_type":    "Bearer",
						"expires_in":    300,
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
					return
				}
				http.Error(w, "unexpected request", http.StatusBadRequest)
			}))
			defer tokenServer.Close()

			expiredTD := &auth.TokenData{
				AccessToken:   makeJWT(time.Now().Add(-1 * time.Minute)),
				RefreshToken:  "old-refresh-token",
				Expiry:        time.Now().Add(-1 * time.Minute),
				TokenEndpoint: tokenServer.URL,
			}
			Expect(store.Save("http://keycloak:8080/realms/dcm", expiredTD)).To(Succeed())

			var hits atomic.Int32
			base := &countingRoundTripper{
				base:      http.DefaultTransport,
				hits:      &hits,
				matchHost: strings.TrimPrefix(strings.TrimPrefix(tokenServer.URL, "https://"), "http://"),
			}
			transport := &auth.AuthTransport{
				Base:      base,
				Store:     store,
				IssuerURL: "http://keycloak:8080/realms/dcm",
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(hits.Load()).To(BeNumerically(">=", 1))
		})

		It("prefers RefreshTransport over Base for token refresh", func() {
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.FormValue("grant_type") == "refresh_token" {
					newExp := time.Now().Add(5 * time.Minute)
					resp := map[string]any{
						"access_token":  makeJWT(newExp),
						"refresh_token": "new-refresh-token",
						"token_type":    "Bearer",
						"expires_in":    300,
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
					return
				}
				http.Error(w, "unexpected request", http.StatusBadRequest)
			}))
			defer tokenServer.Close()

			expiredTD := &auth.TokenData{
				AccessToken:   makeJWT(time.Now().Add(-1 * time.Minute)),
				RefreshToken:  "old-refresh-token",
				Expiry:        time.Now().Add(-1 * time.Minute),
				TokenEndpoint: tokenServer.URL,
			}
			Expect(store.Save("http://keycloak:8080/realms/dcm", expiredTD)).To(Succeed())

			tokenHost := strings.TrimPrefix(strings.TrimPrefix(tokenServer.URL, "https://"), "http://")
			var baseHits, refreshHits atomic.Int32
			base := &countingRoundTripper{
				base:      http.DefaultTransport,
				hits:      &baseHits,
				matchHost: tokenHost,
			}
			refresh := &countingRoundTripper{
				base:      http.DefaultTransport,
				hits:      &refreshHits,
				matchHost: tokenHost,
			}
			transport := &auth.AuthTransport{
				Base:             base,
				RefreshTransport: refresh,
				Store:            store,
				IssuerURL:        "http://keycloak:8080/realms/dcm",
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(refreshHits.Load()).To(BeNumerically(">=", 1))
			Expect(baseHits.Load()).To(BeZero())
		})
	})

	Describe("Refresh persist failure", func() {
		It("still injects the refreshed token when Store.Save fails", func() {
			tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost && r.FormValue("grant_type") == "refresh_token" {
					newExp := time.Now().Add(5 * time.Minute)
					resp := map[string]any{
						"access_token":  makeJWT(newExp),
						"refresh_token": "new-refresh-token",
						"token_type":    "Bearer",
						"expires_in":    300,
					}
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(resp)
					return
				}
				http.Error(w, "unexpected request", http.StatusBadRequest)
			}))
			defer tokenServer.Close()

			expiredTD := &auth.TokenData{
				AccessToken:   makeJWT(time.Now().Add(-1 * time.Minute)),
				RefreshToken:  "old-refresh-token",
				Expiry:        time.Now().Add(-1 * time.Minute),
				TokenEndpoint: tokenServer.URL,
			}
			Expect(store.Save("http://keycloak:8080/realms/dcm", expiredTD)).To(Succeed())

			tmpFile, err := os.CreateTemp(GinkgoT().TempDir(), "stderr-*")
			Expect(err).NotTo(HaveOccurred())

			transport := &auth.AuthTransport{
				Base:      http.DefaultTransport,
				Store:     &failSaveStore{TokenStore: store, err: errors.New("disk full")},
				IssuerURL: "http://keycloak:8080/realms/dcm",
				Stderr:    tmpFile,
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(receivedAuth).To(HavePrefix("Bearer "))
			Expect(receivedAuth).NotTo(Equal("Bearer " + expiredTD.AccessToken))

			Expect(tmpFile.Close()).To(Succeed())
			content, err := os.ReadFile(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("could not save refreshed credentials"))
		})
	})

	Describe("HTTP scheme warning", func() {
		It("warns when sending Bearer token over HTTP", func() {
			tmpFile, err := os.CreateTemp(GinkgoT().TempDir(), "stderr-*")
			Expect(err).NotTo(HaveOccurred())

			transport := &auth.AuthTransport{
				Base:        http.DefaultTransport,
				StaticToken: "my-token",
				Stderr:      tmpFile,
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()

			Expect(tmpFile.Close()).To(Succeed())
			content, err := os.ReadFile(tmpFile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("unencrypted HTTP"))
		})
	})

	Describe("No auth configured", func() {
		It("passes through without modification", func() {
			transport := &auth.AuthTransport{
				Base: http.DefaultTransport,
			}
			client := &http.Client{Transport: transport}

			resp, err := client.Get(backend.URL)
			Expect(err).NotTo(HaveOccurred())
			_ = resp.Body.Close()
			Expect(receivedAuth).To(BeEmpty())
		})
	})
})
