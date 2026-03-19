package commands_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/commands"
)

// clearDCMEnvVars removes all DCM_* environment variables to isolate tests.
func clearDCMEnvVars() {
	for _, env := range []string{
		"DCM_API_GATEWAY_URL",
		"DCM_OUTPUT_FORMAT",
		"DCM_TIMEOUT",
		"DCM_CONFIG",
		"DCM_TLS_CA_CERT",
		"DCM_TLS_CLIENT_CERT",
		"DCM_TLS_CLIENT_KEY",
		"DCM_TLS_SKIP_VERIFY",
	} {
		Expect(os.Unsetenv(env)).To(Succeed())
	}
}

// nonexistentConfigPath returns a path to a config file that does not exist,
// suitable for isolating tests from the developer's ~/.dcm/config.yaml.
func nonexistentConfigPath() string {
	return filepath.Join(GinkgoT().TempDir(), "nonexistent.yaml")
}

// writeTempFile creates a temporary file with the given content and extension.
func writeTempFile(content, ext string) string {
	f, err := os.CreateTemp(GinkgoT().TempDir(), "test-*"+ext)
	Expect(err).NotTo(HaveOccurred())
	_, err = f.WriteString(content)
	Expect(err).NotTo(HaveOccurred())
	Expect(f.Close()).To(Succeed())
	return f.Name()
}

// samplePolicyResponse returns a sample policy JSON response body.
func samplePolicyResponse() map[string]any {
	return map[string]any{
		"id":           "my-policy",
		"display_name": "Require CPU Limits",
		"policy_type":  "GLOBAL",
		"priority":     float64(100),
		"enabled":      true,
		"create_time":  "2026-03-09T10:00:00Z",
	}
}

// writeJSONResponse writes a JSON response with the given status code and body.
func writeJSONResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	Expect(json.NewEncoder(w).Encode(v)).To(Succeed())
}

// writeRFC7807 writes an RFC 7807 error response.
func writeRFC7807(w http.ResponseWriter, status int, typ, title, detail string) {
	body := map[string]any{
		"type":   typ,
		"status": status,
		"title":  title,
		"detail": detail,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	Expect(json.NewEncoder(w).Encode(body)).To(Succeed())
}

// emptyListResponse returns a standard empty list response body.
func emptyListResponse() map[string]any {
	return map[string]any{
		"policies":        []any{},
		"next_page_token": "",
	}
}

var _ = Describe("Policy Commands", func() {
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

	// executeCommand creates a root command, sets up output capture, and executes
	// with the given args prepended by --api-gateway-url and --config.
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

	// --- Section 5: Policy Commands ---

	Describe("create", func() {
		// TC-U026: Create policy from YAML file
		It("TC-U026: should create a policy from a YAML file", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/policies"))

				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body["display_name"]).To(Equal("Test Policy"))

				writeJSONResponse(w, http.StatusCreated, samplePolicyResponse())
			}))

			yamlFile := writeTempFile("display_name: Test Policy\npolicy_type: GLOBAL\npriority: 100\nenabled: true\n", ".yaml")

			err := executeCommand("policy", "create", "--from-file", yamlFile)
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("my-policy"))
			Expect(out).To(ContainSubstring("Require CPU Limits"))
		})

		// TC-U027: Create policy from JSON file
		It("TC-U027: should create a policy from a JSON file", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/policies"))

				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body["display_name"]).To(Equal("JSON Policy"))

				writeJSONResponse(w, http.StatusCreated, samplePolicyResponse())
			}))

			jsonFile := writeTempFile(`{"display_name":"JSON Policy","policy_type":"GLOBAL"}`, ".json")

			err := executeCommand("policy", "create", "--from-file", jsonFile)
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U028: Create policy with client-specified ID
		It("TC-U028: should send ?id= query parameter when --id is provided", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/policies"))
				Expect(r.URL.Query().Get("id")).To(Equal("my-policy"))

				writeJSONResponse(w, http.StatusCreated, samplePolicyResponse())
			}))

			yamlFile := writeTempFile("display_name: Test\n", ".yaml")

			err := executeCommand("policy", "create", "--from-file", yamlFile, "--id", "my-policy")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U029: Create policy without --from-file fails
		It("TC-U029: should return a UsageError when --from-file is not provided", func() {
			err := executeCommand("policy", "create")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U062: Invalid file produces error (transitively via TC-U026)
		It("TC-U062: should return an error when the input file does not exist", func() {
			err := executeCommand("policy", "create", "--from-file", "/nonexistent/policy.yaml")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("reading file"))
		})

		// TC-U063: Unreadable file content produces error (transitively via TC-U026)
		It("TC-U063: should return an error when the input file contains non-object content", func() {
			invalidFile := writeTempFile("- item1\n- item2\n", ".yaml")

			err := executeCommand("policy", "create", "--from-file", invalidFile)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("parsing file"))
		})

		// TC-U104: Create policy server error
		It("TC-U104: should display error and exit code 1 on server error", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusInternalServerError, "INTERNAL", "Internal server error", "Something went wrong")
			}))

			yamlFile := writeTempFile("display_name: Test\n", ".yaml")

			err := executeCommand("policy", "create", "--from-file", yamlFile)
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("INTERNAL"))
		})
	})

	Describe("list", func() {
		// TC-U030: List policies
		It("TC-U030: should list policies", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/policies"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"policies":        []any{samplePolicyResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("policy", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("my-policy"))
			Expect(out).To(ContainSubstring("Require CPU Limits"))
		})

		// TC-U031: List policies with filter
		It("TC-U031: should pass filter query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("filter")).To(Equal("policy_type='GLOBAL'"))

				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))

			err := executeCommand("policy", "list", "--filter", "policy_type='GLOBAL'")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U032: List policies with order-by
		It("TC-U032: should pass order_by query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("order_by")).To(Equal("priority asc"))

				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))

			err := executeCommand("policy", "list", "--order-by", "priority asc")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U033: List policies with pagination
		It("TC-U033: should pass max_page_size and page_token query parameters", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("max_page_size")).To(Equal("10"))
				Expect(r.URL.Query().Get("page_token")).To(Equal("abc123"))

				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))

			err := executeCommand("policy", "list", "--page-size", "10", "--page-token", "abc123")
			Expect(err).NotTo(HaveOccurred())
		})

		// TC-U100: List policies returns empty list
		It("TC-U100: should display empty result for empty list", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))

			err := executeCommand("policy", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			// Table output should have headers but no data rows
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("DISPLAY NAME"))
			Expect(out).NotTo(ContainSubstring("my-policy"))
		})

		// TC-U100 (JSON variant): Empty list in JSON format
		It("TC-U100: should display empty results array in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyListResponse())
			}))

			err := executeCommand("--output", "json", "policy", "list")
			Expect(err).NotTo(HaveOccurred())

			var result map[string]any
			Expect(json.Unmarshal(outBuf.Bytes(), &result)).To(Succeed())
			Expect(result["results"]).To(BeAssignableToTypeOf([]any{}))
			Expect(result["results"]).To(BeEmpty())
		})
	})

	Describe("get", func() {
		// TC-U034: Get policy
		It("TC-U034: should get a policy by ID", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/policies/my-policy"))

				writeJSONResponse(w, http.StatusOK, samplePolicyResponse())
			}))

			err := executeCommand("policy", "get", "my-policy")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("my-policy"))
			Expect(out).To(ContainSubstring("Require CPU Limits"))
		})

		// TC-U035: Get policy without POLICY_ID fails
		It("TC-U035: should return a UsageError when POLICY_ID is missing", func() {
			err := executeCommand("policy", "get")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U101: Get non-existent policy
		It("TC-U101: should display error for non-existent policy", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Policy "nonexistent" not found.`,
					"The requested policy resource does not exist.")
			}))

			err := executeCommand("policy", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U041: Policy table output columns
		It("TC-U041: should display correct table columns", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, samplePolicyResponse())
			}))

			err := executeCommand("policy", "get", "my-policy")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("DISPLAY NAME"))
			Expect(out).To(ContainSubstring("TYPE"))
			Expect(out).To(ContainSubstring("PRIORITY"))
			Expect(out).To(ContainSubstring("ENABLED"))
			Expect(out).To(ContainSubstring("CREATED"))
			Expect(out).To(ContainSubstring("my-policy"))
			Expect(out).To(ContainSubstring("Require CPU Limits"))
			Expect(out).To(ContainSubstring("GLOBAL"))
			Expect(out).To(ContainSubstring("100"))
			Expect(out).To(ContainSubstring("true"))
			Expect(out).To(ContainSubstring("2026-03-09T10:00:00Z"))
		})
	})

	Describe("update", func() {
		// TC-U036: Update policy
		It("TC-U036: should update a policy with a patch file", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPatch))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/policies/my-policy"))

				var body map[string]any
				Expect(json.NewDecoder(r.Body).Decode(&body)).To(Succeed())
				Expect(body["priority"]).To(Equal(float64(50)))

				resp := samplePolicyResponse()
				resp["priority"] = float64(50)
				writeJSONResponse(w, http.StatusOK, resp)
			}))

			patchFile := writeTempFile("priority: 50\n", ".yaml")

			err := executeCommand("policy", "update", "my-policy", "--from-file", patchFile)
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("my-policy"))
			Expect(out).To(ContainSubstring("50"))
		})

		// TC-U037: Update policy without --from-file fails
		It("TC-U037: should return a UsageError when --from-file is not provided", func() {
			err := executeCommand("policy", "update", "my-policy")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U038: Update policy without POLICY_ID fails
		It("TC-U038: should return a UsageError when POLICY_ID is missing", func() {
			err := executeCommand("policy", "update")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U102: Update non-existent policy
		It("TC-U102: should display error for non-existent policy", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Policy "nonexistent" not found.`,
					"The requested policy resource does not exist.")
			}))

			patchFile := writeTempFile("priority: 50\n", ".yaml")

			err := executeCommand("policy", "update", "nonexistent", "--from-file", patchFile)
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("NOT_FOUND"))
		})
	})

	Describe("delete", func() {
		// TC-U039: Delete policy
		It("TC-U039: should delete a policy and display success message", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodDelete))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/policies/my-policy"))

				w.WriteHeader(http.StatusNoContent)
			}))

			err := executeCommand("policy", "delete", "my-policy")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring(`Policy "my-policy" deleted successfully.`))
		})

		// TC-U040: Delete policy without POLICY_ID fails
		It("TC-U040: should return a UsageError when POLICY_ID is missing", func() {
			err := executeCommand("policy", "delete")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		// TC-U103: Delete non-existent policy
		It("TC-U103: should display error for non-existent policy", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Policy "nonexistent" not found.`,
					"The requested policy resource does not exist.")
			}))

			err := executeCommand("policy", "delete", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())
			Expect(errBuf.String()).To(ContainSubstring("NOT_FOUND"))
		})
	})

	// --- Section 10: Error Handling ---

	Describe("Error Handling", func() {
		// TC-U080: API error displayed in table format
		It("TC-U080: should display RFC 7807 error in table format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Policy "nonexistent" not found.`,
					"The requested policy resource does not exist.")
			}))

			err := executeCommand("policy", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring(`Error: NOT_FOUND - Policy "nonexistent" not found.`))
			Expect(errOut).To(ContainSubstring("Status: 404"))
			Expect(errOut).To(ContainSubstring("Detail: The requested policy resource does not exist."))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U081: API error displayed in JSON format
		It("TC-U081: should display RFC 7807 error in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Policy "nonexistent" not found.`,
					"The requested policy resource does not exist.")
			}))

			err := executeCommand("--output", "json", "policy", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			// stderr should contain valid JSON with Problem Details
			var problem map[string]any
			Expect(json.Unmarshal(errBuf.Bytes(), &problem)).To(Succeed())
			Expect(problem["type"]).To(Equal("NOT_FOUND"))
			Expect(problem["status"]).To(BeNumerically("==", 404))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U082: API error displayed in YAML format
		It("TC-U082: should display RFC 7807 error in YAML format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Policy "nonexistent" not found.`,
					"The requested policy resource does not exist.")
			}))

			err := executeCommand("--output", "yaml", "policy", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("type: NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("status: 404"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U083: Connection error displays clear message
		It("TC-U083: should display a connection error when API Gateway is unreachable", func() {
			// Use a server that is immediately closed to simulate unreachable
			closedServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
			closedURL := closedServer.URL
			closedServer.Close()

			cmd := commands.NewRootCommand()
			outBuf = new(bytes.Buffer)
			errBuf = new(bytes.Buffer)
			cmd.SetOut(outBuf)
			cmd.SetErr(errBuf)
			cmd.SetArgs([]string{
				"--config", nonexistentConfigPath(),
				"--api-gateway-url", closedURL,
				"policy", "list",
			})

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to connect"))
		})

		// TC-U084: Timeout error displays clear message
		It("TC-U084: should display a timeout error when request exceeds timeout", func() {
			server = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				// Delay longer than the configured timeout
				time.Sleep(3 * time.Second)
			}))

			err := executeCommand("--timeout", "1", "policy", "list")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("timed out"))
		})

		// TC-U085: Exit code 1 on runtime error
		It("TC-U085: should return a non-UsageError for server errors", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusInternalServerError, "INTERNAL", "Internal error", "Something broke")
			}))

			err := executeCommand("policy", "list")
			Expect(err).To(HaveOccurred())

			// Should NOT be a UsageError (exit code 1, not 2)
			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeFalse())
		})

		// TC-U116: Error output written to stderr (command-level verification)
		It("TC-U116: should write error output to stderr and nothing to stdout", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND", "Not found", "Resource not found")
			}))

			err := executeCommand("policy", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			Expect(errBuf.String()).NotTo(BeEmpty())
			Expect(outBuf.String()).To(BeEmpty())
		})

		// TC-U120: Non-RFC-7807 error response
		It("TC-U120: should display HTTP status and body for non-RFC-7807 errors", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadGateway)
				_, err := fmt.Fprint(w, "Bad Gateway")
				Expect(err).NotTo(HaveOccurred())
			}))

			err := executeCommand("policy", "list")
			Expect(err).To(HaveOccurred())

			// Should NOT be a FormattedError (plain error message)
			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeFalse())

			Expect(err.Error()).To(ContainSubstring("502"))
			Expect(err.Error()).To(ContainSubstring("Bad Gateway"))
		})
	})
})
