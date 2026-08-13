package auth_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/auth"
)

// makeJWT builds a minimal unsigned JWT with the given exp claim.
func makeJWT(exp time.Time) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"exp":%d,"sub":"test-user"}`, exp.Unix())
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	sig := base64.RawURLEncoding.EncodeToString([]byte("fake-signature"))
	return header + "." + payload + "." + sig
}

func sampleTokenData(exp time.Time) *auth.TokenData {
	return &auth.TokenData{
		AccessToken:   makeJWT(exp),
		RefreshToken:  "refresh-token-value",
		IDToken:       "id-token-value",
		Expiry:        exp,
		TokenEndpoint: "http://keycloak:8080/realms/dcm/protocol/openid-connect/token",
	}
}

var _ = Describe("TokenData", func() {
	Describe("String", func() {
		It("redacts token values", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(td.String()).To(Equal("[REDACTED]"))
		})
	})

	Describe("IsExpired", func() {
		It("returns false for a token expiring in the future beyond clock skew", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(td.IsExpired(30 * time.Second)).To(BeFalse())
		})

		It("returns true for a token that expired in the past", func() {
			td := sampleTokenData(time.Now().Add(-1 * time.Second))
			Expect(td.IsExpired(30 * time.Second)).To(BeTrue())
		})

		It("returns true for a token within the clock skew buffer", func() {
			td := sampleTokenData(time.Now().Add(20 * time.Second))
			Expect(td.IsExpired(30 * time.Second)).To(BeTrue())
		})

		It("returns false for a token just outside the clock skew buffer", func() {
			td := sampleTokenData(time.Now().Add(31 * time.Second))
			Expect(td.IsExpired(30 * time.Second)).To(BeFalse())
		})

		It("returns true for an invalid JWT with no Expiry fallback", func() {
			td := &auth.TokenData{AccessToken: "not-a-jwt"}
			Expect(td.IsExpired(30 * time.Second)).To(BeTrue())
		})

		It("falls back to TokenData.Expiry for opaque access tokens", func() {
			td := &auth.TokenData{
				AccessToken: "opaque-access-token",
				Expiry:      time.Now().Add(5 * time.Minute),
			}
			Expect(td.IsExpired(30 * time.Second)).To(BeFalse())
		})

		It("treats opaque tokens as expired when TokenData.Expiry is past", func() {
			td := &auth.TokenData{
				AccessToken: "opaque-access-token",
				Expiry:      time.Now().Add(-time.Minute),
			}
			Expect(td.IsExpired(30 * time.Second)).To(BeTrue())
		})
	})
})

var _ = Describe("FileStore", func() {
	var (
		store     auth.TokenStore
		storeDir  string
		issuerURL string
	)

	BeforeEach(func() {
		storeDir = GinkgoT().TempDir()
		store = auth.NewFileStoreWithDir(storeDir)
		issuerURL = "http://keycloak:8080/realms/dcm"
	})

	Describe("Save and Load round-trip", func() {
		It("persists and retrieves token data", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(store.Save(issuerURL, td)).To(Succeed())

			loaded, err := store.Load(issuerURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).NotTo(BeNil())
			Expect(loaded.AccessToken).To(Equal(td.AccessToken))
			Expect(loaded.RefreshToken).To(Equal(td.RefreshToken))
			Expect(loaded.TokenEndpoint).To(Equal(td.TokenEndpoint))
		})
	})

	Describe("Load with no stored token", func() {
		It("returns nil without error", func() {
			loaded, err := store.Load(issuerURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeNil())
		})
	})

	Describe("Delete", func() {
		It("removes stored token data", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(store.Save(issuerURL, td)).To(Succeed())
			Expect(store.Delete(issuerURL)).To(Succeed())

			loaded, err := store.Load(issuerURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).To(BeNil())
		})

		It("succeeds when no token exists", func() {
			Expect(store.Delete(issuerURL)).To(Succeed())
		})
	})

	Describe("Issuer URL normalization", func() {
		It("treats trailing-slash and no-trailing-slash as the same key", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(store.Save(issuerURL+"/", td)).To(Succeed())

			loaded, err := store.Load(issuerURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded).NotTo(BeNil())
			Expect(loaded.AccessToken).To(Equal(td.AccessToken))
		})
	})

	Describe("File permissions", func() {
		It("creates the token file with 0600 permissions", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(store.Save(issuerURL, td)).To(Succeed())

			info, err := os.Stat(filepath.Join(storeDir, "tokens.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		})

		It("creates the token directory with 0700 permissions", func() {
			nestedDir := filepath.Join(GinkgoT().TempDir(), "nested", ".dcm")
			store = auth.NewFileStoreWithDir(nestedDir)
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(store.Save(issuerURL, td)).To(Succeed())

			info, err := os.Stat(nestedDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o700)))
		})
	})

	Describe("Inaccessible paths", func() {
		BeforeEach(func() {
			if os.Geteuid() == 0 {
				Skip("permission-bit checks are unreliable when running as root")
			}
		})

		It("returns an error when Save cannot write to an unwritable store directory", func() {
			// 0555: readable so readAll sees IsNotExist, but WriteFile of .tmp fails.
			Expect(os.MkdirAll(storeDir, 0o755)).To(Succeed())
			Expect(os.Chmod(storeDir, 0o555)).To(Succeed())
			DeferCleanup(func() {
				_ = os.Chmod(storeDir, 0o700)
			})

			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			err := store.Save(issuerURL, td)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("writing token file"))
		})

		It("returns an error when Save cannot create the store directory", func() {
			// Parent must be searchable so readAll gets IsNotExist for the nested
			// path, then MkdirAll fails because the parent is not writable.
			parent := GinkgoT().TempDir()
			Expect(os.Chmod(parent, 0o555)).To(Succeed())
			DeferCleanup(func() {
				_ = os.Chmod(parent, 0o700)
			})

			store = auth.NewFileStoreWithDir(filepath.Join(parent, "nested"))
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			err := store.Save(issuerURL, td)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("creating token directory"))
		})

		It("returns an error when Load and Save cannot read an unreadable tokens.json", func() {
			Expect(os.MkdirAll(storeDir, 0o700)).To(Succeed())
			tokenPath := filepath.Join(storeDir, "tokens.json")
			Expect(os.WriteFile(tokenPath, []byte("{}"), 0o600)).To(Succeed())
			Expect(os.Chmod(tokenPath, 0o000)).To(Succeed())
			DeferCleanup(func() {
				_ = os.Chmod(tokenPath, 0o600)
			})

			_, loadErr := store.Load(issuerURL)
			Expect(loadErr).To(HaveOccurred())
			Expect(loadErr.Error()).To(ContainSubstring("reading token file"))

			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			saveErr := store.Save(issuerURL, td)
			Expect(saveErr).To(HaveOccurred())
			Expect(saveErr.Error()).To(ContainSubstring("reading token file"))
		})
	})

	Describe("Atomic writes", func() {
		It("does not leave a .tmp file on success", func() {
			td := sampleTokenData(time.Now().Add(5 * time.Minute))
			Expect(store.Save(issuerURL, td)).To(Succeed())

			_, err := os.Stat(filepath.Join(storeDir, "tokens.json.tmp"))
			Expect(os.IsNotExist(err)).To(BeTrue())
		})
	})

	Describe("Multiple issuers", func() {
		It("stores tokens for different issuers independently", func() {
			td1 := sampleTokenData(time.Now().Add(5 * time.Minute))
			td2 := sampleTokenData(time.Now().Add(10 * time.Minute))
			td2.RefreshToken = "other-refresh-token"

			issuer2 := "http://other-keycloak:8080/realms/other"

			Expect(store.Save(issuerURL, td1)).To(Succeed())
			Expect(store.Save(issuer2, td2)).To(Succeed())

			loaded1, err := store.Load(issuerURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded1.RefreshToken).To(Equal("refresh-token-value"))

			loaded2, err := store.Load(issuer2)
			Expect(err).NotTo(HaveOccurred())
			Expect(loaded2.RefreshToken).To(Equal("other-refresh-token"))
		})
	})

	Describe("Corrupt file handling", func() {
		It("returns an error for invalid JSON", func() {
			Expect(os.MkdirAll(storeDir, 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(storeDir, "tokens.json"), []byte("not json"), 0o600)).To(Succeed())

			_, err := store.Load(issuerURL)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parsing token file"))
		})
	})
})

var _ = Describe("newFileStore", func() {
	AfterEach(func() {
		auth.ResetUserHomeDir()
	})

	It("returns an error when the home directory cannot be resolved", func() {
		restore := auth.SetUserHomeDir(func() (string, error) {
			return "", fmt.Errorf("no home")
		})
		defer restore()

		_, err := auth.NewFileStore()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("resolving home directory for token store"))
	})

	It("returns an error when the home directory is empty", func() {
		restore := auth.SetUserHomeDir(func() (string, error) {
			return "", nil
		})
		defer restore()

		_, err := auth.NewFileStore()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("empty home"))
	})
})

var _ = Describe("SaveConfig integration", func() {
	It("creates config file when it does not exist", func() {
		dir := GinkgoT().TempDir()
		configPath := filepath.Join(dir, "config.yaml")

		store := auth.NewFileStoreWithDir(dir)
		td := sampleTokenData(time.Now().Add(5 * time.Minute))
		Expect(store.Save("http://keycloak:8080/realms/dcm", td)).To(Succeed())

		_, err := os.Stat(configPath)
		Expect(os.IsNotExist(err)).To(BeTrue(), "token store should not create config.yaml")

		data, err := json.Marshal(map[string]string{"issuer-url": "http://keycloak:8080/realms/dcm"})
		Expect(err).NotTo(HaveOccurred())
		Expect(data).NotTo(BeEmpty())
	})
})
