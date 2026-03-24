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

// sampleCatalogItemResponse returns a sample catalog item JSON response body.
func sampleCatalogItemResponse() map[string]any {
	return map[string]any{
		"path":         "catalog-items/my-catalog-item",
		"uid":          "b2c3d4e5-f6a7-8901-bcde-f12345678901",
		"display_name": "Small Container",
		"create_time":  "2026-03-09T10:00:00Z",
		"spec": map[string]any{
			"service_type": "container",
		},
	}
}

// emptyCatalogItemListResponse returns a standard empty catalog item list response body.
func emptyCatalogItemListResponse() map[string]any {
	return map[string]any{
		"results":         []any{},
		"next_page_token": "",
	}
}

var _ = Describe("Catalog Item Commands", func() {
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
		// TC-U046: Create catalog item from file
		It("TC-U046: should create a catalog item from a YAML file", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-items"))

				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body["display_name"]).To(Equal("Small Container"))

				writeJSONResponse(w, http.StatusCreated, sampleCatalogItemResponse())
			}))

			yamlFile := writeTempFile("display_name: Small Container\nspec:\n  service_type: container\n", ".yaml")

			err := executeCommand("catalog", "item", "create", "--from-file", yamlFile)
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("b2c3d4e5-f6a7-8901-bcde-f12345678901"))
			Expect(out).To(ContainSubstring("Small Container"))
		})

		// TC-U047: Create catalog item with client-specified ID
		It("TC-U047: should send ?id= query parameter when --id is provided", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-items"))
				Expect(r.URL.Query().Get("id")).To(Equal("my-catalog-item"))

				writeJSONResponse(w, http.StatusCreated, sampleCatalogItemResponse())
			}))

			yamlFile := writeTempFile("display_name: Small Container\n", ".yaml")

			err := executeCommand("catalog", "item", "create", "--from-file", yamlFile, "--id", "my-catalog-item")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U048: Create catalog item without --from-file fails
		It("TC-U048: should return a UsageError when --from-file is not provided", func() {
			err := executeCommand("catalog", "item", "create")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U111: Create catalog item server error
		It("TC-U111: should display error and exit code 1 on server error", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusInternalServerError, "INTERNAL", "Internal server error", "Something went wrong")
			}))

			yamlFile := writeTempFile("display_name: Test\n", ".yaml")

			err := executeCommand("catalog", "item", "create", "--from-file", yamlFile)
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("INTERNAL"))
		})
	})

	Describe("list", func() {
		// TC-U049: List catalog items
		It("TC-U049: should list catalog items", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-items"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"results":         []any{sampleCatalogItemResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("catalog", "item", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("b2c3d4e5-f6a7-8901-bcde-f12345678901"))
			Expect(out).To(ContainSubstring("Small Container"))
		})

		// TC-U050: List catalog items with service-type filter
		It("TC-U050: should pass service_type query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("service_type")).To(Equal("container"))

				writeJSONResponse(w, http.StatusOK, emptyCatalogItemListResponse())
			}))

			err := executeCommand("catalog", "item", "list", "--service-type", "container")
			Expect(err).NotTo(HaveOccurred())
		})

		// List catalog items with pagination (page-size)
		It("should pass max_page_size query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("max_page_size")).To(Equal("10"))

				writeJSONResponse(w, http.StatusOK, emptyCatalogItemListResponse())
			}))

			err := executeCommand("catalog", "item", "list", "--page-size", "10")
			Expect(err).NotTo(HaveOccurred())
		})

		// List catalog items with pagination (page-token)
		It("should pass page_token query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("page_token")).To(Equal("abc123"))

				writeJSONResponse(w, http.StatusOK, emptyCatalogItemListResponse())
			}))

			err := executeCommand("catalog", "item", "list", "--page-token", "abc123")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U107: List catalog items returns empty list
		It("TC-U107: should display empty result for empty list", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyCatalogItemListResponse())
			}))

			err := executeCommand("catalog", "item", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			// Table output should have headers but no data rows
			Expect(out).To(ContainSubstring("UID"))
			Expect(out).To(ContainSubstring("DISPLAY NAME"))
			Expect(out).NotTo(ContainSubstring("b2c3d4e5"))
		})

		// TC-U107 (JSON variant): Empty list in JSON format
		It("TC-U107: should display empty results array in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyCatalogItemListResponse())
			}))

			err := executeCommand("--output", "json", "catalog", "item", "list")
			Expect(err).NotTo(HaveOccurred())

			var result map[string]any
			Expect(json.Unmarshal(outBuf.Bytes(), &result)).To(Succeed())
			Expect(result["results"]).To(BeAssignableToTypeOf([]any{}))
			Expect(result["results"]).To(BeEmpty())
		})
	})

	Describe("get", func() {
		// TC-U051: Get catalog item
		It("TC-U051: should get a catalog item by ID", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-items/my-catalog-item"))

				writeJSONResponse(w, http.StatusOK, sampleCatalogItemResponse())
			}))

			err := executeCommand("catalog", "item", "get", "my-catalog-item")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("b2c3d4e5-f6a7-8901-bcde-f12345678901"))
			Expect(out).To(ContainSubstring("Small Container"))
		})

		// TC-U052: Get catalog item without CATALOG_ITEM_ID fails
		It("TC-U052: should return a UsageError when CATALOG_ITEM_ID is missing", func() {
			err := executeCommand("catalog", "item", "get")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U108: Get non-existent catalog item
		It("TC-U108: should display error for non-existent catalog item", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Catalog item "nonexistent" not found.`,
					"The requested catalog item resource does not exist.")
			}))

			err := executeCommand("catalog", "item", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U057: Catalog item table output columns
		It("TC-U057: should display correct table columns", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, sampleCatalogItemResponse())
			}))

			err := executeCommand("catalog", "item", "get", "my-catalog-item")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("UID"))
			Expect(out).To(ContainSubstring("DISPLAY NAME"))
			Expect(out).To(ContainSubstring("SERVICE TYPE"))
			Expect(out).To(ContainSubstring("CREATED"))
			Expect(out).To(ContainSubstring("b2c3d4e5-f6a7-8901-bcde-f12345678901"))
			Expect(out).To(ContainSubstring("Small Container"))
			Expect(out).To(ContainSubstring("container"))
			Expect(out).To(ContainSubstring("2026-03-09T10:00:00Z"))
		})
	})

	Describe("delete", func() {
		// TC-U055: Delete catalog item
		It("TC-U055: should delete a catalog item and display success message", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodDelete))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-items/my-catalog-item"))

				w.WriteHeader(http.StatusNoContent)
			}))

			err := executeCommand("catalog", "item", "delete", "my-catalog-item")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring(`Catalog item "my-catalog-item" deleted successfully.`))
		})

		// TC-U056: Delete catalog item without CATALOG_ITEM_ID fails
		It("TC-U056: should return a UsageError when CATALOG_ITEM_ID is missing", func() {
			err := executeCommand("catalog", "item", "delete")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U110: Delete non-existent catalog item
		It("TC-U110: should display error for non-existent catalog item", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Catalog item "nonexistent" not found.`,
					"The requested catalog item resource does not exist.")
			}))

			err := executeCommand("catalog", "item", "delete", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("NOT_FOUND"))
		})
	})
})
