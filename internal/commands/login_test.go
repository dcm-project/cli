package commands_test

import (
	"bytes"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/zalando/go-keyring"

	"github.com/dcm-project/cli/internal/auth"
	"github.com/dcm-project/cli/internal/commands"
)

var _ = Describe("login command", func() {
	BeforeEach(func() {
		clearDCMEnvVars()
		keyring.MockInit()
	})

	It("fails with UsageError when issuer-url is not configured", func() {
		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{"--config", nonexistentConfigPath(), "login"})

		err := cmd.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("--issuer-url is required (or set DCM_ISSUER_URL)"))
		var usageErr *commands.UsageError
		Expect(errors.As(err, &usageErr)).To(BeTrue())
	})

	It("completes device login, stores tokens, and persists config", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		configPath := filepath.Join(home, "dcm-config.yaml")

		server := mockOIDCServer(mockOIDCOptions{pollsBeforeSuccess: 0})
		defer server.Close()

		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", configPath,
			"--issuer-url", server.URL,
			"--control-plane-url", "http://cp.example:8080",
			"login",
		})

		err := cmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		Expect(errBuf.String()).To(ContainSubstring("Open "))
		Expect(errBuf.String()).To(ContainSubstring("ABCD-EFGH"))
		Expect(errBuf.String()).To(ContainSubstring("Logged in as dcm-admin"))
		Expect(errBuf.String()).To(ContainSubstring("auto-refresh enabled"))

		store, err := auth.NewTokenStore()
		Expect(err).NotTo(HaveOccurred())
		td, err := store.Load(server.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(td).NotTo(BeNil())
		Expect(td.RefreshToken).To(Equal("test-refresh-token"))
		Expect(td.AccessToken).NotTo(BeEmpty())

		cfgData, err := os.ReadFile(configPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cfgData)).To(ContainSubstring("issuer-url: " + server.URL))
		Expect(string(cfgData)).To(ContainSubstring("control-plane-url: http://cp.example:8080"))
		Expect(string(cfgData)).NotTo(ContainSubstring("token:"))
	})

	It("persists issuer-url but not the default control-plane-url", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		configPath := filepath.Join(home, "dcm-config.yaml")

		server := mockOIDCServer(mockOIDCOptions{pollsBeforeSuccess: 0})
		defer server.Close()

		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", configPath,
			"--issuer-url", server.URL,
			"login",
		})

		err := cmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		cfgData, err := os.ReadFile(configPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cfgData)).To(ContainSubstring("issuer-url: " + server.URL))
		Expect(string(cfgData)).NotTo(ContainSubstring("control-plane-url"))
	})

	It("persists control-plane-url from DCM_CONTROL_PLANE_URL", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DCM_CONTROL_PLANE_URL", "http://env-cp.example:8080")
		configPath := filepath.Join(home, "dcm-config.yaml")

		server := mockOIDCServer(mockOIDCOptions{pollsBeforeSuccess: 0})
		defer server.Close()

		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", configPath,
			"--issuer-url", server.URL,
			"login",
		})

		err := cmd.Execute()
		Expect(err).NotTo(HaveOccurred())

		cfgData, err := os.ReadFile(configPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(cfgData)).To(ContainSubstring("issuer-url: " + server.URL))
		Expect(string(cfgData)).To(ContainSubstring("control-plane-url: http://env-cp.example:8080"))
	})

	It("completes device login when stored credentials are expired and refresh fails", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)

		server := mockOIDCServer(mockOIDCOptions{
			pollsBeforeSuccess: 0,
			rejectRefresh:      true,
		})
		defer server.Close()

		store, err := auth.NewTokenStore()
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Save(server.URL, &auth.TokenData{
			AccessToken:   makeTestJWT(time.Now().Add(-time.Hour), "stale-user"),
			RefreshToken:  "invalid-refresh",
			Expiry:        time.Now().Add(-time.Hour),
			TokenEndpoint: server.URL + "/protocol/openid-connect/token",
		})).To(Succeed())

		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", nonexistentConfigPath(),
			"--issuer-url", server.URL,
			"--control-plane-url", "http://cp.example:8080",
			"login",
		})

		err = cmd.Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(errBuf.String()).To(ContainSubstring("Logged in as dcm-admin"))
		Expect(errBuf.String()).To(ContainSubstring("auto-refresh enabled"))

		td, err := store.Load(server.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(td.RefreshToken).To(Equal("test-refresh-token"))
	})

	It("applies --tls-ca-cert for HTTPS issuer when control-plane URL is HTTP", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)

		ca := newTestCA()
		serverCert, serverKey := ca.issueCert("localhost", []string{"localhost"}, net.IPv4(127, 0, 0, 1))
		tlsCert, err := tls.X509KeyPair(serverCert, serverKey)
		Expect(err).NotTo(HaveOccurred())

		tlsServer := mockOIDCTLSServer(mockOIDCOptions{pollsBeforeSuccess: 0}, &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		})
		defer tlsServer.Close()

		caFile := writePEM(home, "ca.pem", ca.certPEM)

		cmd := commands.NewRootCommand()
		errBuf := new(bytes.Buffer)
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(errBuf)
		cmd.SetArgs([]string{
			"--config", nonexistentConfigPath(),
			"--issuer-url", tlsServer.URL,
			"--control-plane-url", "http://localhost:8080",
			"--tls-ca-cert", caFile,
			"login",
		})

		err = cmd.Execute()
		Expect(err).NotTo(HaveOccurred())
		Expect(errBuf.String()).To(ContainSubstring("Logged in as dcm-admin"))
		Expect(errBuf.String()).To(ContainSubstring("auto-refresh enabled"))
	})
})
