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

// sampleSPResourceResponse returns a sample SP resource (service type instance) JSON response body.
func sampleSPResourceResponse() map[string]any {
	return map[string]any{
		"id":            "my-instance",
		"path":          "service-type-instances/my-instance",
		"provider_name": "kubevirt-123",
		"status":        "READY",
		"create_time":   "2026-03-09T10:00:00Z",
		"spec":          map[string]any{},
	}
}

// sampleDeletedSPResourceResponse returns a sample soft-deleted SP resource JSON response body.
func sampleDeletedSPResourceResponse() map[string]any {
	return map[string]any{
		"id":              "deleted-instance",
		"path":            "service-type-instances/deleted-instance",
		"provider_name":   "kubevirt-123",
		"status":          "DELETED",
		"deletion_status": "PENDING",
		"create_time":     "2026-03-09T10:00:00Z",
		"spec":            map[string]any{},
	}
}

// emptySPResourceListResponse returns a standard empty SP resource list response body.
func emptySPResourceListResponse() map[string]any {
	return map[string]any{
		"instances":       []any{},
		"next_page_token": "",
	}
}

var _ = Describe("SP Resource Commands", func() {
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
		// TC-U121: List SP resources
		It("TC-U121: should list SP resources", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/service-type-instances"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"instances":       []any{sampleSPResourceResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("sp", "resource", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("my-instance"))
			Expect(out).To(ContainSubstring("kubevirt-123"))
			Expect(out).To(ContainSubstring("READY"))
		})

		// TC-U122: List SP resources with pagination
		It("TC-U122: should pass max_page_size query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("max_page_size")).To(Equal("5"))

				writeJSONResponse(w, http.StatusOK, emptySPResourceListResponse())
			}))

			err := executeCommand("sp", "resource", "list", "--page-size", "5")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U122 (page-token variant): List SP resources with page token
		It("TC-U122: should pass page_token query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("page_token")).To(Equal("abc123"))

				writeJSONResponse(w, http.StatusOK, emptySPResourceListResponse())
			}))

			err := executeCommand("sp", "resource", "list", "--page-token", "abc123")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U123: List SP resources with provider filter
		It("TC-U123: should pass provider query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("provider")).To(Equal("kubevirt-123"))

				writeJSONResponse(w, http.StatusOK, emptySPResourceListResponse())
			}))

			err := executeCommand("sp", "resource", "list", "--provider", "kubevirt-123")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass show_deleted query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("show_deleted")).To(Equal("true"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"instances":       []any{sampleSPResourceResponse(), sampleDeletedSPResourceResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("sp", "resource", "list", "--show-deleted")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("DELETION STATUS"))
			Expect(out).To(ContainSubstring("my-instance"))
			Expect(out).To(ContainSubstring("deleted-instance"))
			Expect(out).To(ContainSubstring("PENDING"))
		})

		It("should not include deletion status column without --show-deleted", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("show_deleted")).To(BeEmpty())

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"instances":       []any{sampleSPResourceResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("sp", "resource", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).NotTo(ContainSubstring("DELETION STATUS"))
			Expect(out).To(ContainSubstring("my-instance"))
			Expect(out).ToNot(ContainSubstring("deleted-instance"))
		})

		// TC-U126: List SP resources returns empty list
		It("TC-U126: should display empty result for empty list", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptySPResourceListResponse())
			}))

			err := executeCommand("sp", "resource", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			// Table output should have headers but no data rows
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("PROVIDER"))
			Expect(out).NotTo(ContainSubstring("kubevirt"))
		})

		// TC-U126 (JSON variant): Empty list in JSON format
		It("TC-U126: should display empty results array in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptySPResourceListResponse())
			}))

			err := executeCommand("--output", "json", "sp", "resource", "list")
			Expect(err).NotTo(HaveOccurred())

			var result map[string]any
			Expect(json.Unmarshal(outBuf.Bytes(), &result)).To(Succeed())
			Expect(result["results"]).To(BeAssignableToTypeOf([]any{}))
			Expect(result["results"]).To(BeEmpty())
		})
	})

	Describe("get", func() {
		// TC-U124: Get SP resource
		It("TC-U124: should get an SP resource by ID", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/service-type-instances/my-instance"))

				writeJSONResponse(w, http.StatusOK, sampleSPResourceResponse())
			}))

			err := executeCommand("sp", "resource", "get", "my-instance")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("my-instance"))
			Expect(out).To(ContainSubstring("kubevirt-123"))
			Expect(out).To(ContainSubstring("READY"))
		})

		It("should pass show_deleted query parameter on get", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("show_deleted")).To(Equal("true"))

				writeJSONResponse(w, http.StatusOK, sampleDeletedSPResourceResponse())
			}))

			err := executeCommand("sp", "resource", "get", "deleted-instance", "--show-deleted")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("DELETION STATUS"))
			Expect(out).To(ContainSubstring("deleted-instance"))
			Expect(out).To(ContainSubstring("PENDING"))
		})

		It("should not include deletion status column on get without --show-deleted", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("show_deleted")).To(BeEmpty())

				writeJSONResponse(w, http.StatusOK, sampleSPResourceResponse())
			}))

			err := executeCommand("sp", "resource", "get", "my-instance")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).NotTo(ContainSubstring("DELETION STATUS"))
			Expect(out).To(ContainSubstring("my-instance"))
		})

		// TC-U125: Get SP resource without INSTANCE_ID fails
		It("TC-U125: should return a UsageError when INSTANCE_ID is missing", func() {
			err := executeCommand("sp", "resource", "get")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U127: Get non-existent SP resource
		It("TC-U127: should display error for non-existent SP resource", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`SP resource "nonexistent" not found.`,
					"The requested SP resource does not exist.")
			}))

			err := executeCommand("sp", "resource", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U128: SP resource table output columns
		It("TC-U128: should display correct table columns", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, sampleSPResourceResponse())
			}))

			err := executeCommand("sp", "resource", "get", "my-instance")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("PROVIDER"))
			Expect(out).To(ContainSubstring("STATUS"))
			Expect(out).To(ContainSubstring("CREATED"))
			Expect(out).To(ContainSubstring("my-instance"))
			Expect(out).To(ContainSubstring("kubevirt-123"))
			Expect(out).To(ContainSubstring("READY"))
			Expect(out).To(ContainSubstring("2026-03-09T10:00:00Z"))
		})
	})
})
