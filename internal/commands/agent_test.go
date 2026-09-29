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

// sampleAgentResponse returns a sample environment agent JSON response body.
func sampleAgentResponse() map[string]any {
	return map[string]any{
		"agent_id":      "agent-123",
		"name":          "kubevirt-east",
		"environment":   "production",
		"health_status": "ready",
		"create_time":   "2026-03-09T10:00:00Z",
	}
}

// emptyAgentListResponse returns a standard empty agent list response body.
func emptyAgentListResponse() map[string]any {
	return map[string]any{
		"agents":          []any{},
		"next_page_token": "",
	}
}

var _ = Describe("Agent Commands", func() {
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
			fullArgs = append(fullArgs, "--control-plane-url", server.URL)
		}
		fullArgs = append(fullArgs, args...)
		cmd.SetArgs(fullArgs)

		return cmd.Execute()
	}

	Describe("list", func() {
		It("should list environment agents", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/agents"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"agents":          []any{sampleAgentResponse()},
					"next_page_token": "",
				})
			}))

			err := executeCommand("agent", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("agent-123"))
			Expect(out).To(ContainSubstring("kubevirt-east"))
			Expect(out).To(ContainSubstring("production"))
		})

		It("should pass max_page_size query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("max_page_size")).To(Equal("5"))

				writeJSONResponse(w, http.StatusOK, emptyAgentListResponse())
			}))

			err := executeCommand("agent", "list", "--page-size", "5")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass page_token query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("page_token")).To(Equal("abc123"))

				writeJSONResponse(w, http.StatusOK, emptyAgentListResponse())
			}))

			err := executeCommand("agent", "list", "--page-token", "abc123")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should pass health_status query parameter", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("health_status")).To(Equal("ready"))

				writeJSONResponse(w, http.StatusOK, emptyAgentListResponse())
			}))

			err := executeCommand("agent", "list", "--health-status", "ready")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include --health-status in the next-page command", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Query().Get("health_status")).To(Equal("ready"))

				writeJSONResponse(w, http.StatusOK, map[string]any{
					"agents":          []any{sampleAgentResponse()},
					"next_page_token": "page-2",
				})
			}))

			err := executeCommand("agent", "list", "--health-status", "ready")
			Expect(err).NotTo(HaveOccurred())
			Expect(outBuf.String()).To(ContainSubstring("Next page: dcm agent list --health-status ready --page-token page-2"))
		})

		It("should display empty result for empty list", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyAgentListResponse())
			}))

			err := executeCommand("agent", "list")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("NAME"))
			Expect(out).To(ContainSubstring("ENVIRONMENT"))
			Expect(out).NotTo(ContainSubstring("agent-123"))
		})

		It("should display empty results array in JSON format", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, emptyAgentListResponse())
			}))

			err := executeCommand("--output", "json", "agent", "list")
			Expect(err).NotTo(HaveOccurred())

			var result map[string]any
			Expect(json.Unmarshal(outBuf.Bytes(), &result)).To(Succeed())
			Expect(result["results"]).To(BeAssignableToTypeOf([]any{}))
			Expect(result["results"]).To(BeEmpty())
		})
	})

	Describe("get", func() {
		It("should get an environment agent by ID", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/agents/agent-123"))

				writeJSONResponse(w, http.StatusOK, sampleAgentResponse())
			}))

			err := executeCommand("agent", "get", "agent-123")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("agent-123"))
			Expect(out).To(ContainSubstring("kubevirt-east"))
			Expect(out).To(ContainSubstring("production"))
		})

		It("should return a UsageError when AGENT_ID is missing", func() {
			err := executeCommand("agent", "get")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue())
		})

		It("should display error for non-existent agent", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRFC7807(w, http.StatusNotFound, "NOT_FOUND",
					`Agent "nonexistent" not found.`,
					"The requested agent does not exist.")
			}))

			err := executeCommand("agent", "get", "nonexistent")
			Expect(err).To(HaveOccurred())

			var fmtErr *commands.FormattedError
			Expect(errors.As(err, &fmtErr)).To(BeTrue())

			errOut := errBuf.String()
			Expect(errOut).To(ContainSubstring("NOT_FOUND"))
			Expect(errOut).To(ContainSubstring("not found"))
			Expect(outBuf.String()).To(BeEmpty())
		})

		It("should display correct table columns", func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeJSONResponse(w, http.StatusOK, sampleAgentResponse())
			}))

			err := executeCommand("agent", "get", "agent-123")
			Expect(err).NotTo(HaveOccurred())

			out := outBuf.String()
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("NAME"))
			Expect(out).To(ContainSubstring("ENVIRONMENT"))
			Expect(out).To(ContainSubstring("HEALTH"))
			Expect(out).To(ContainSubstring("CREATED"))
			Expect(out).To(ContainSubstring("agent-123"))
			Expect(out).To(ContainSubstring("kubevirt-east"))
			Expect(out).To(ContainSubstring("production"))
			Expect(out).To(ContainSubstring("ready"))
			Expect(out).To(ContainSubstring("2026-03-09T10:00:00Z"))
		})
	})
})
