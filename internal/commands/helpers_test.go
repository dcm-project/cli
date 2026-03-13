package commands_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/commands"
)

// testCA holds a self-signed CA and can issue certificates for testing.
type testCA struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
}

func newTestCA() *testCA {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	cert, err := x509.ParseCertificate(certDER)
	Expect(err).NotTo(HaveOccurred())

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	return &testCA{cert: cert, key: key, certPEM: certPEM}
}

// issueCert creates a server or client certificate signed by the CA.
// Returns PEM-encoded cert and key.
func (ca *testCA) issueCert(cn string, dnsNames []string, ipAddrs ...net.IP) (certPEM, keyPEM []byte) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		DNSNames:     dnsNames,
		IPAddresses:  ipAddrs,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	Expect(err).NotTo(HaveOccurred())

	keyDER, err := x509.MarshalECPrivateKey(key)
	Expect(err).NotTo(HaveOccurred())

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM
}

// writePEM writes PEM data to a temp file and returns the path.
func writePEM(dir, name string, data []byte) string {
	path := filepath.Join(dir, name)
	Expect(os.WriteFile(path, data, 0o600)).To(Succeed())
	return path
}

var _ = Describe("buildHTTPClient (via commands)", func() {
	var (
		outBuf *bytes.Buffer
		errBuf *bytes.Buffer
	)

	BeforeEach(func() {
		clearDCMEnvVars()
	})

	executeWithArgs := func(args ...string) error {
		cmd := commands.NewRootCommand()
		outBuf = new(bytes.Buffer)
		errBuf = new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)

		fullArgs := make([]string, 0, 2+len(args))
		fullArgs = append(fullArgs, "--config", nonexistentConfigPath())
		fullArgs = append(fullArgs, args...)
		cmd.SetArgs(fullArgs)

		return cmd.Execute()
	}

	Describe("http:// URL", func() {
		It("should ignore TLS flags and connect successfully", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))
			defer server.Close()

			// TLS flags are present but should be silently ignored for http://
			err := executeWithArgs(
				"--api-gateway-url", server.URL,
				"--tls-skip-verify",
				"policy", "list",
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("https:// URL", func() {
		It("should connect to an HTTPS server with --tls-ca-cert", func() {
			ca := newTestCA()
			serverCert, serverKey := ca.issueCert("localhost", []string{"localhost"}, net.IPv4(127, 0, 0, 1))

			tlsCert, err := tls.X509KeyPair(serverCert, serverKey)
			Expect(err).NotTo(HaveOccurred())

			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))
			server.TLS = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
			server.StartTLS()
			defer server.Close()

			tmpDir := GinkgoT().TempDir()
			caFile := writePEM(tmpDir, "ca.pem", ca.certPEM)

			err = executeWithArgs(
				"--api-gateway-url", server.URL,
				"--tls-ca-cert", caFile,
				"policy", "list",
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should connect with --tls-skip-verify", func() {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))
			defer server.Close()

			err := executeWithArgs(
				"--api-gateway-url", server.URL,
				"--tls-skip-verify",
				"policy", "list",
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail without --tls-ca-cert or --tls-skip-verify against self-signed server", func() {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))
			defer server.Close()

			err := executeWithArgs(
				"--api-gateway-url", server.URL,
				"policy", "list",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to connect"))
		})

		It("should connect with mTLS client certificate", func() {
			ca := newTestCA()
			serverCert, serverKey := ca.issueCert("localhost", []string{"localhost"}, net.IPv4(127, 0, 0, 1))
			clientCert, clientKey := ca.issueCert("client", nil)

			tlsCert, err := tls.X509KeyPair(serverCert, serverKey)
			Expect(err).NotTo(HaveOccurred())

			caPool := x509.NewCertPool()
			caPool.AddCert(ca.cert)

			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))
			server.TLS = &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    caPool,
			}
			server.StartTLS()
			defer server.Close()

			tmpDir := GinkgoT().TempDir()
			caFile := writePEM(tmpDir, "ca.pem", ca.certPEM)
			certFile := writePEM(tmpDir, "client.pem", clientCert)
			keyFile := writePEM(tmpDir, "client-key.pem", clientKey)

			err = executeWithArgs(
				"--api-gateway-url", server.URL,
				"--tls-ca-cert", caFile,
				"--tls-client-cert", certFile,
				"--tls-client-key", keyFile,
				"policy", "list",
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("validation errors", func() {
		It("should return UsageError when only --tls-client-cert is set", func() {
			tmpDir := GinkgoT().TempDir()
			certFile := writePEM(tmpDir, "client.pem", []byte("dummy"))

			err := executeWithArgs(
				"--api-gateway-url", "https://localhost:9999",
				"--tls-client-cert", certFile,
				"--tls-skip-verify",
				"policy", "list",
			)
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("--tls-client-cert and --tls-client-key must be used together"))
		})

		It("should return UsageError when only --tls-client-key is set", func() {
			tmpDir := GinkgoT().TempDir()
			keyFile := writePEM(tmpDir, "client-key.pem", []byte("dummy"))

			err := executeWithArgs(
				"--api-gateway-url", "https://localhost:9999",
				"--tls-client-key", keyFile,
				"--tls-skip-verify",
				"policy", "list",
			)
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("--tls-client-cert and --tls-client-key must be used together"))
		})

		It("should return error when --tls-ca-cert file does not exist", func() {
			err := executeWithArgs(
				"--api-gateway-url", "https://localhost:9999",
				"--tls-ca-cert", "/nonexistent/ca.pem",
				"--tls-skip-verify",
				"policy", "list",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reading CA certificate"))

			// Should be exit code 1, not 2
			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeFalse())
		})

		It("should return error when --tls-ca-cert contains invalid PEM", func() {
			tmpDir := GinkgoT().TempDir()
			caFile := writePEM(tmpDir, "bad-ca.pem", []byte("not a certificate"))

			err := executeWithArgs(
				"--api-gateway-url", "https://localhost:9999",
				"--tls-ca-cert", caFile,
				"--tls-skip-verify",
				"policy", "list",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to parse CA certificate"))
		})

		It("should return error when client cert file does not exist", func() {
			tmpDir := GinkgoT().TempDir()
			keyFile := writePEM(tmpDir, "client-key.pem", []byte("dummy"))

			err := executeWithArgs(
				"--api-gateway-url", "https://localhost:9999",
				"--tls-client-cert", "/nonexistent/client.pem",
				"--tls-client-key", keyFile,
				"--tls-skip-verify",
				"policy", "list",
			)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("loading client certificate"))
		})

		It("should not validate TLS settings for http:// URL", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))
			defer server.Close()

			// Even with mismatched mTLS flags, http:// should succeed
			err := executeWithArgs(
				"--api-gateway-url", server.URL,
				"--tls-client-cert", "/nonexistent/client.pem",
				"policy", "list",
			)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
