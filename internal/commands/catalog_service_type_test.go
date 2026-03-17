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

// sampleServiceTypeResponse returns a sample service type JSON response body.
func sampleServiceTypeResponse() map[string]any {
	return map[string]any{
		"uid":          "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"path":         "service-types/container",
		"service_type": "container",
		"api_version":  "v1alpha1",
		"create_time":  "2026-03-09T10:00:00Z",
		"spec":         map[string]any{},
	}
}

// emptyServiceTypeListResponse returns a standard empty service type list response body.
func emptyServiceTypeListResponse() map[string]any {
	return map[string]any{
		"results":         []any{},
		"next_page_token": "",
	}
}

var _ = Describe("Catalog Service-Type Commands", func() {
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
		// TC-U042: List service types
		It("TC-U042: should list service types", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/service-types"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"results":         []any{sampleServiceTypeResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("catalog", "service-type", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("container"))
			Expect(out).To(ContainSubstring("v1alpha1"))
		})

		// TC-U043: List service types with pagination
		It("TC-U043: should pass max_page_size query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("max_page_size")).To(Equal("5"))

				writeJSONResponse(w, http.StatusOK, emptyServiceTypeListResponse())
			}))

			err := executeCommand("catalog", "service-type", "list", "--page-size", "5")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U043 (page-token variant): List service types with page token
		It("TC-U043: should pass page_token query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("page_token")).To(Equal("abc123"))

				writeJSONResponse(w, http.StatusOK, emptyServiceTypeListResponse())
			}))

			err := executeCommand("catalog", "service-type", "list", "--page-token", "abc123")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U105: List service types returns empty list
		It("TC-U105: should display empty result for empty list", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyServiceTypeListResponse())
			}))

			err := executeCommand("catalog", "service-type", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			// Table output should have headers but no data rows
			Expect(out).To(ContainSubstring("UID"))
			Expect(out).To(ContainSubstring("SERVICE TYPE"))
			Expect(out).NotTo(ContainSubstring("container"))
		})

		// TC-U105 (JSON variant): Empty list in JSON format
		It("TC-U105: should display empty results array in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyServiceTypeListResponse())
			}))

			err := executeCommand("--output", "json", "catalog", "service-type", "list")
			Expect(err).NotTo(HaveOccurred())

			var result map[string]any
			Expect(json.Unmarshal(outBuf.Bytes(), &result)).To(Succeed())
			Expect(result["results"]).To(BeAssignableToTypeOf([]any{}))
			Expect(result["results"]).To(BeEmpty())
		})
	})

	Describe("get", func() {
		// TC-U044: Get service type
		It("TC-U044: should get a service type by ID", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/service-types/my-service-type"))

				writeJSONResponse(w, http.StatusOK, sampleServiceTypeResponse())
			}))

			err := executeCommand("catalog", "service-type", "get", "my-service-type")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("container"))
			Expect(out).To(ContainSubstring("v1alpha1"))
		})

		// TC-U045: Get service type without SERVICE_TYPE_ID fails
		It("TC-U045: should return a UsageError when SERVICE_TYPE_ID is missing", func() {
			err := executeCommand("catalog", "service-type", "get")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U106: Get non-existent service type
		It("TC-U106: should display error for non-existent service type", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Service type "nonexistent" not found.`,
					"The requested service type resource does not exist.")
			}))

			err := executeCommand("catalog", "service-type", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// Table output columns verification (part of TC-U044)
		It("should display correct table columns", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, sampleServiceTypeResponse())
			}))

			err := executeCommand("catalog", "service-type", "get", "my-service-type")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("UID"))
			Expect(out).To(ContainSubstring("SERVICE TYPE"))
			Expect(out).To(ContainSubstring("API VERSION"))
			Expect(out).To(ContainSubstring("CREATED"))
			Expect(out).To(ContainSubstring("a1b2c3d4-e5f6-7890-abcd-ef1234567890"))
			Expect(out).To(ContainSubstring("container"))
			Expect(out).To(ContainSubstring("v1alpha1"))
			Expect(out).To(ContainSubstring("2026-03-09T10:00:00Z"))
		})
	})
})
