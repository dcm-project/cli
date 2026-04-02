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

// sampleInstanceResponse returns a sample catalog item instance JSON response body.
func sampleInstanceResponse() map[string]any {
	return map[string]any{
		"path":         "catalog-item-instances/my-instance",
		"uid":          "c3d4e5f6-a7b8-9012-cdef-123456789012",
		"display_name": "My App Instance",
		"create_time":  "2026-03-09T10:00:00Z",
		"resource_id":  "res-abc123",
		"spec": map[string]any{
			"catalog_item_id": "my-catalog-item",
		},
	}
}

// emptyInstanceListResponse returns a standard empty instance list response body.
func emptyInstanceListResponse() map[string]any {
	return map[string]any{
		"results":         []any{},
		"next_page_token": "",
	}
}

var _ = Describe("Catalog Instance Commands", func() {
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

	Describe("create", func() {
		// TC-U058: Create instance from file
		It("TC-U058: should create an instance from a YAML file", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-item-instances"))

				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body["display_name"]).To(Equal("My App Instance"))

				writeJSONResponse(w, http.StatusCreated, sampleInstanceResponse())
			}))

			yamlFile := writeTempFile("display_name: My App Instance\napi_version: v1alpha1\nspec:\n  catalog_item_id: my-catalog-item\n  user_values: []\n", ".yaml")

			err := executeCommand("catalog", "instance", "create", "--from-file", yamlFile)
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("c3d4e5f6-a7b8-9012-cdef-123456789012"))
			Expect(out).To(ContainSubstring("My App Instance"))
		})

		// TC-U059: Create instance with client-specified ID
		It("TC-U059: should send ?id= query parameter when --id is provided", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-item-instances"))
				Expect(r.URL.Query().Get("id")).To(Equal("my-instance"))

				writeJSONResponse(w, http.StatusCreated, sampleInstanceResponse())
			}))

			yamlFile := writeTempFile("display_name: My App Instance\napi_version: v1alpha1\nspec:\n  catalog_item_id: my-catalog-item\n  user_values: []\n", ".yaml")

			err := executeCommand("catalog", "instance", "create", "--from-file", yamlFile, "--id", "my-instance")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U072: Create instance without --from-file fails
		It("TC-U072: should return a UsageError when --from-file is not provided", func() {
			err := executeCommand("catalog", "instance", "create")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U115: Create instance server error
		It("TC-U115: should display error and exit code 1 on server error", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusInternalServerError, "INTERNAL", "Internal server error", "Something went wrong")
			}))

			yamlFile := writeTempFile("display_name: Test\napi_version: v1alpha1\nspec:\n  catalog_item_id: test\n  user_values: []\n", ".yaml")

			err := executeCommand("catalog", "instance", "create", "--from-file", yamlFile)
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("INTERNAL"))
		})
	})

	Describe("list", func() {
		// TC-U073: List instances
		It("TC-U073: should list instances", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-item-instances"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"results":         []any{sampleInstanceResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("catalog", "instance", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("c3d4e5f6-a7b8-9012-cdef-123456789012"))
			Expect(out).To(ContainSubstring("My App Instance"))
		})

		// List instances with catalog-item-id filter
		It("should pass catalog_item_id query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("catalog_item_id")).To(Equal("my-catalog-item"))

				writeJSONResponse(w, http.StatusOK, emptyInstanceListResponse())
			}))

			err := executeCommand("catalog", "instance", "list", "--catalog-item-id", "my-catalog-item")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U074: List instances with pagination
		It("TC-U074: should pass max_page_size query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("max_page_size")).To(Equal("10"))

				writeJSONResponse(w, http.StatusOK, emptyInstanceListResponse())
			}))

			err := executeCommand("catalog", "instance", "list", "--page-size", "10")
			Expect(err).NotTo(HaveOccurred())
		})

		// List instances with pagination (page-token)
		It("should pass page_token query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("page_token")).To(Equal("abc123"))

				writeJSONResponse(w, http.StatusOK, emptyInstanceListResponse())
			}))

			err := executeCommand("catalog", "instance", "list", "--page-token", "abc123")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U112: List instances returns empty list
		It("TC-U112: should display empty result for empty list", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyInstanceListResponse())
			}))

			err := executeCommand("catalog", "instance", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			// Table output should have headers but no data rows
			Expect(out).To(ContainSubstring("UID"))
			Expect(out).To(ContainSubstring("DISPLAY NAME"))
			Expect(out).NotTo(ContainSubstring("c3d4e5f6"))
		})

		// TC-U112 (JSON variant): Empty list in JSON format
		It("TC-U112: should display empty results array in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyInstanceListResponse())
			}))

			err := executeCommand("--output", "json", "catalog", "instance", "list")
			Expect(err).NotTo(HaveOccurred())

			var result map[string]any
			Expect(json.Unmarshal(outBuf.Bytes(), &result)).To(Succeed())
			Expect(result["results"]).To(BeAssignableToTypeOf([]any{}))
			Expect(result["results"]).To(BeEmpty())
		})
	})

	Describe("get", func() {
		// TC-U075: Get instance
		It("TC-U075: should get an instance by ID", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-item-instances/my-instance"))

				writeJSONResponse(w, http.StatusOK, sampleInstanceResponse())
			}))

			err := executeCommand("catalog", "instance", "get", "my-instance")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("c3d4e5f6-a7b8-9012-cdef-123456789012"))
			Expect(out).To(ContainSubstring("My App Instance"))
		})

		// TC-U076: Get instance without INSTANCE_ID fails
		It("TC-U076: should return a UsageError when INSTANCE_ID is missing", func() {
			err := executeCommand("catalog", "instance", "get")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U113: Get non-existent instance
		It("TC-U113: should display error for non-existent instance", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Catalog item instance "nonexistent" not found.`,
					"The requested catalog item instance resource does not exist.")
			}))

			err := executeCommand("catalog", "instance", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U079: Instance table output columns
		It("TC-U079: should display correct table columns", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, sampleInstanceResponse())
			}))

			err := executeCommand("catalog", "instance", "get", "my-instance")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("UID"))
			Expect(out).To(ContainSubstring("DISPLAY NAME"))
			Expect(out).To(ContainSubstring("CATALOG ITEM"))
			Expect(out).To(ContainSubstring("RESOURCE ID"))
			Expect(out).To(ContainSubstring("CREATED"))
			Expect(out).To(ContainSubstring("c3d4e5f6-a7b8-9012-cdef-123456789012"))
			Expect(out).To(ContainSubstring("My App Instance"))
			Expect(out).To(ContainSubstring("my-catalog-item"))
			Expect(out).To(ContainSubstring("res-abc123"))
			Expect(out).To(ContainSubstring("2026-03-09T10:00:00Z"))
		})
	})

	Describe("rehydrate", func() {
		// TC-U150: Rehydrate instance
		It("TC-U150: should rehydrate an instance and display the result", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-item-instances/my-instance:rehydrate"))

				writeJSONResponse(w, http.StatusOK, sampleInstanceResponse())
			}))

			err := executeCommand("catalog", "instance", "rehydrate", "my-instance")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("c3d4e5f6-a7b8-9012-cdef-123456789012"))
			Expect(out).To(ContainSubstring("My App Instance"))
		})

		// TC-U151: Rehydrate instance without INSTANCE_ID fails
		It("TC-U151: should return a UsageError when INSTANCE_ID is missing", func() {
			err := executeCommand("catalog", "instance", "rehydrate")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U152: Rehydrate non-existent instance
		It("TC-U152: should display error for non-existent instance", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Catalog item instance "nonexistent" not found.`,
					"The requested catalog item instance resource does not exist.")
			}))

			err := executeCommand("catalog", "instance", "rehydrate", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U153: Rehydrate instance server error
		It("TC-U153: should display error and exit code 1 on server error", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusInternalServerError, "INTERNAL", "Internal server error", "Something went wrong")
			}))

			err := executeCommand("catalog", "instance", "rehydrate", "my-instance")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("INTERNAL"))
		})
	})

	Describe("delete", func() {
		// TC-U077: Delete instance
		It("TC-U077: should delete an instance and display success message", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodDelete))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-item-instances/my-instance"))

				w.WriteHeader(http.StatusNoContent)
			}))

			err := executeCommand("catalog", "instance", "delete", "my-instance")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring(`Catalog item instance "my-instance" deleted successfully.`))
		})

		// TC-U078: Delete instance without INSTANCE_ID fails
		It("TC-U078: should return a UsageError when INSTANCE_ID is missing", func() {
			err := executeCommand("catalog", "instance", "delete")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U114: Delete non-existent instance
		It("TC-U114: should display error for non-existent instance", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Catalog item instance "nonexistent" not found.`,
					"The requested catalog item instance resource does not exist.")
			}))

			err := executeCommand("catalog", "instance", "delete", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("NOT_FOUND"))
		})
	})
})
