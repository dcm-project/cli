package commands_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/commands"
)

// sampleSPProviderResponse returns a sample SP provider JSON response body.
func sampleSPProviderResponse() map[string]any {
	return map[string]any{
		"id":            "kubevirt-123",
		"path":          "providers/kubevirt-123",
		"name":          "KubeVirt SP",
		"service_type":  "compute",
		"health_status": "healthy",
		"create_time":   "2026-03-09T10:00:00Z",
	}
}

// emptySPProviderListResponse returns a standard empty SP provider list response body.
func emptySPProviderListResponse() map[string]any {
	return map[string]any{
		"providers":       []any{},
		"next_page_token": "",
	}
}

var _ = Describe("SP Provider Commands", func() {
	var (
		server *httptest.Server
		outBuf *bytes.Buffer
		errBuf *bytes.Buffer
	)

	BeforeEach(func() {
		clearDCMEnvVars()
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
			server = nil
		}
	})

	executeCommand := func(args ...string) error {
		cmd := commands.NewRootCommand()
		outBuf = new(bytes.Buffer)
		errBuf = new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)

		fullArgs := []string{
			"--config", nonexistentConfigPath(),
		}
		if server != nil {
			fullArgs = append(fullArgs, "--api-gateway-url", server.URL)
		}
		fullArgs = append(fullArgs, args...)
		cmd.SetArgs(fullArgs)

		return cmd.Execute()
	}

	Describe("list", func() {
		// TC-U139: List SP providers
		It("TC-U139: should list SP providers", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/providers"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"providers":       []any{sampleSPProviderResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("sp", "provider", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("kubevirt-123"))
			Expect(out).To(ContainSubstring("KubeVirt SP"))
			Expect(out).To(ContainSubstring("compute"))
		})

		// TC-U140: List SP providers with pagination
		It("TC-U140: should pass max_page_size query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("max_page_size")).To(Equal("5"))

				writeJSONResponse(w, http.StatusOK, emptySPProviderListResponse())
			}))

			err := executeCommand("sp", "provider", "list", "--page-size", "5")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U140 (page-token variant): List SP providers with page token
		It("TC-U140: should pass page_token query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("page_token")).To(Equal("abc123"))

				writeJSONResponse(w, http.StatusOK, emptySPProviderListResponse())
			}))

			err := executeCommand("sp", "provider", "list", "--page-token", "abc123")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U141: List SP providers with type filter
		It("TC-U141: should pass type query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("type")).To(Equal("compute"))

				writeJSONResponse(w, http.StatusOK, emptySPProviderListResponse())
			}))

			err := executeCommand("sp", "provider", "list", "--type", "compute")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U144: List SP providers returns empty list
		It("TC-U144: should display empty result for empty list", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptySPProviderListResponse())
			}))

			err := executeCommand("sp", "provider", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			// Table output should have headers but no data rows
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("NAME"))
			Expect(out).To(ContainSubstring("SERVICE TYPE"))
			Expect(out).NotTo(ContainSubstring("kubevirt"))
		})

		// TC-U144 (JSON variant): Empty list in JSON format
		It("TC-U144: should display empty results array in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptySPProviderListResponse())
			}))

			err := executeCommand("--output", "json", "sp", "provider", "list")
			Expect(err).NotTo(HaveOccurred())

			var result map[string]any
			Expect(json.Unmarshal(outBuf.Bytes(), &result)).To(Succeed())
			Expect(result["results"]).To(BeAssignableToTypeOf([]any{}))
			Expect(result["results"]).To(BeEmpty())
		})
	})

	Describe("get", func() {
		// TC-U142: Get SP provider
		It("TC-U142: should get an SP provider by ID", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/providers/kubevirt-123"))

				writeJSONResponse(w, http.StatusOK, sampleSPProviderResponse())
			}))

			err := executeCommand("sp", "provider", "get", "kubevirt-123")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("kubevirt-123"))
			Expect(out).To(ContainSubstring("KubeVirt SP"))
			Expect(out).To(ContainSubstring("compute"))
		})

		// TC-U143: Get SP provider without PROVIDER_ID fails
		It("TC-U143: should return a UsageError when PROVIDER_ID is missing", func() {
			err := executeCommand("sp", "provider", "get")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U145: Get non-existent SP provider
		It("TC-U145: should display error for non-existent SP provider", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`SP provider "nonexistent" not found.`,
					"The requested SP provider does not exist.")
			}))

			err := executeCommand("sp", "provider", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U146: SP provider table output columns
		It("TC-U146: should display correct table columns", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, sampleSPProviderResponse())
			}))

			err := executeCommand("sp", "provider", "get", "kubevirt-123")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("NAME"))
			Expect(out).To(ContainSubstring("SERVICE TYPE"))
			Expect(out).To(ContainSubstring("HEALTH"))
			Expect(out).To(ContainSubstring("CREATED"))
			Expect(out).NotTo(ContainSubstring("STATUS"))
			Expect(out).To(ContainSubstring("kubevirt-123"))
			Expect(out).To(ContainSubstring("KubeVirt SP"))
			Expect(out).To(ContainSubstring("compute"))
			Expect(out).To(ContainSubstring("healthy"))
			Expect(out).To(ContainSubstring("2026-03-09T10:00:00Z"))
		})
	})
})
