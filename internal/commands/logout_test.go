package commands_test

import (
	"bytes"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zalando/go-keyring"

	"github.com/dcm-project/cli/internal/auth"
	"github.com/dcm-project/cli/internal/commands"
)

var _ = Describe("logout command", func() {
	BeforeEach(func() {
		clearDCMEnvVars()
		keyring.MockInit()
	})

	It("fails with UsageError when issuer-url is not configured", func() {
		cmd := commands.NewRootCommand()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--config", nonexistentConfigPath(), "logout"})

		err := cmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--issuer-url is required (or set DCM_ISSUER_URL)"))
		var usageErr *commands.UsageError
		Expect(errors.As(err, &usageErr)).To(BeTrue())
	})

	It("prints a message when no credentials are stored", func() {
		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", nonexistentConfigPath(),
			"--issuer-url", "http://keycloak.example/realms/dcm",
			"logout",
		})

		err := cmd.Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(errBuf.String()).To(ContainSubstring("No stored credentials found"))
	})

	It("revokes the refresh token and clears stored credentials", func() {
		server := mockOIDCServer(mockOIDCOptions{})
		defer server.Close()

		store, err := auth.NewTokenStore()
		Expect(err).NotTo(HaveOccurred())
		td := &auth.TokenData{
			AccessToken:   makeTestJWT(time.Now().Add(5*time.Minute), "dcm-admin"),
			RefreshToken:  "test-refresh-token",
			IDToken:       "test-id-token",
			Expiry:        time.Now().Add(5 * time.Minute),
			TokenEndpoint: server.URL + "/protocol/openid-connect/token",
		}
		Expect(store.Save(server.URL, td)).To(Succeed())

		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", nonexistentConfigPath(),
			"--issuer-url", server.URL,
			"logout",
		})

		err = cmd.Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(errBuf.String()).To(ContainSubstring("Logged out successfully"))

		loaded, err := store.Load(server.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded).To(BeNil())
	})

	It("clears credentials and warns when revocation fails", func() {
		server := mockOIDCServer(mockOIDCOptions{revokeStatus: 500})
		defer server.Close()

		store, err := auth.NewTokenStore()
		Expect(err).NotTo(HaveOccurred())
		td := &auth.TokenData{
			AccessToken:   makeTestJWT(time.Now().Add(5*time.Minute), "dcm-admin"),
			RefreshToken:  "test-refresh-token",
			IDToken:       "test-id-token",
			Expiry:        time.Now().Add(5 * time.Minute),
			TokenEndpoint: server.URL + "/protocol/openid-connect/token",
		}
		Expect(store.Save(server.URL, td)).To(Succeed())

		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", nonexistentConfigPath(),
			"--issuer-url", server.URL,
			"logout",
		})

		err = cmd.Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(errBuf.String()).To(ContainSubstring("Warning: token revocation failed"))
		Expect(errBuf.String()).To(ContainSubstring("Logged out successfully"))

		loaded, err := store.Load(server.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(loaded).To(BeNil())
	})
})
