package output_test

import (
	"bytes"
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.yaml.in/yaml/v3"

	"github.com/dcm-project/cli/internal/output"
)

// testResource represents a sample resource for testing.
type testResource struct {
	ID          string `json:"id"           yaml:"id"`
	DisplayName string `json:"display_name" yaml:"display_name"`
	Type        string `json:"type"         yaml:"type"`
	Priority    int    `json:"priority"     yaml:"priority"`
	Enabled     bool   `json:"enabled"      yaml:"enabled"`
	Created     string `json:"created"      yaml:"created"`
}

func policyTableDef() *output.TableDef {
	return &output.TableDef{
		Headers: []string{"ID", "DISPLAY NAME", "TYPE", "PRIORITY", "ENABLED", "CREATED"},
		RowFunc: func(resource any) []string {
			r, ok := resource.(testResource)
			if !ok {
				return []string{}
			}
			return []string{
				r.ID,
				r.DisplayName,
				r.Type,
				fmt.Sprintf("%d", r.Priority),
				fmt.Sprintf("%t", r.Enabled),
				r.Created,
			}
		},
	}
}

var _ = Describe("Output Formatting", func() {
	var (
		stdout *bytes.Buffer
		stderr *bytes.Buffer
	)

	BeforeEach(func() {
		stdout = new(bytes.Buffer)
		stderr = new(bytes.Buffer)
	})

	sampleResource := testResource{
		ID:          "my-policy",
		DisplayName: "Require CPU Limits",
		Type:        "GLOBAL",
		Priority:    100,
		Enabled:     true,
		Created:     "2026-03-09T10:00:00Z",
	}

	sampleResources := []any{
		testResource{
			ID:          "policy-1",
			DisplayName: "Policy One",
			Type:        "GLOBAL",
			Priority:    100,
			Enabled:     true,
			Created:     "2026-03-09T10:00:00Z",
		},
		testResource{
			ID:          "policy-2",
			DisplayName: "Policy Two",
			Type:        "USER",
			Priority:    200,
			Enabled:     false,
			Created:     "2026-03-08T15:30:00Z",
		},
		testResource{
			ID:          "policy-3",
			DisplayName: "Policy Three",
			Type:        "GLOBAL",
			Priority:    300,
			Enabled:     true,
			Created:     "2026-03-07T12:00:00Z",
		},
	}

	sampleProblem := output.ProblemDetail{
		Type:   "NOT_FOUND",
		Status: 404,
		Title:  `Policy "nonexistent" not found.`,
		Detail: "The requested policy resource does not exist.",
	}

	Describe("Table Output", func() {
		// TC-U009: Table output for single resource
		It("TC-U009: should display a single resource in tabular format with headers", func() {
			f := output.New(output.FormatTable, stdout, stderr, policyTableDef(), "policy list")

			err := f.FormatOne(sampleResource)
			Expect(err).NotTo(HaveOccurred())

			out := stdout.String()
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
			Expect(stderr.String()).To(BeEmpty())
		})

		// TC-U010: Table output for resource list
		It("TC-U010: should display a list of resources in tabular format with no pagination hint", func() {
			f := output.New(output.FormatTable, stdout, stderr, policyTableDef(), "policy list")

			err := f.FormatList(sampleResources, "")
			Expect(err).NotTo(HaveOccurred())

			out := stdout.String()
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("policy-1"))
			Expect(out).To(ContainSubstring("policy-2"))
			Expect(out).To(ContainSubstring("policy-3"))
			Expect(out).NotTo(ContainSubstring("Next page:"))
			Expect(stderr.String()).To(BeEmpty())
		})

		// TC-U013: Pagination hint in table output
		It("TC-U013: should display a pagination hint when nextPageToken is present", func() {
			f := output.New(output.FormatTable, stdout, stderr, policyTableDef(), "policy list --page-size 2")

			err := f.FormatList(sampleResources[:2], "eyJvZmZzZXQiOjJ9")
			Expect(err).NotTo(HaveOccurred())

			out := stdout.String()
			Expect(out).To(ContainSubstring("Next page: dcm policy list --page-size 2 --page-token eyJvZmZzZXQiOjJ9"))
		})

		// TC-U016: No pagination hint when nextPageToken is empty
		It("TC-U016: should not display a pagination hint when nextPageToken is empty", func() {
			f := output.New(output.FormatTable, stdout, stderr, policyTableDef(), "policy list")

			err := f.FormatList(sampleResources, "")
			Expect(err).NotTo(HaveOccurred())

			Expect(stdout.String()).NotTo(ContainSubstring("Next page:"))
		})
	})

	Describe("JSON Output", func() {
		// TC-U011: JSON output produces valid JSON
		It("TC-U011: should produce valid, parseable JSON for a single resource", func() {
			f := output.New(output.FormatJSON, stdout, stderr, nil, "")

			err := f.FormatOne(sampleResource)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = json.Unmarshal(stdout.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["id"]).To(Equal("my-policy"))
			Expect(parsed["display_name"]).To(Equal("Require CPU Limits"))
			Expect(parsed["type"]).To(Equal("GLOBAL"))
			Expect(stderr.String()).To(BeEmpty())
		})

		// TC-U014: Pagination token in JSON output
		It("TC-U014: should include next_page_token in JSON list output", func() {
			f := output.New(output.FormatJSON, stdout, stderr, nil, "")

			err := f.FormatList(sampleResources, "abc123")
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = json.Unmarshal(stdout.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["next_page_token"]).To(Equal("abc123"))
			Expect(parsed["results"]).To(HaveLen(3))
		})
	})

	Describe("YAML Output", func() {
		// TC-U012: YAML output produces valid YAML
		It("TC-U012: should produce valid, parseable YAML for a single resource", func() {
			f := output.New(output.FormatYAML, stdout, stderr, nil, "")

			err := f.FormatOne(sampleResource)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = yaml.Unmarshal(stdout.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["id"]).To(Equal("my-policy"))
			Expect(parsed["display_name"]).To(Equal("Require CPU Limits"))
			Expect(parsed["type"]).To(Equal("GLOBAL"))
			Expect(stderr.String()).To(BeEmpty())
		})

		// TC-U015: Pagination token in YAML output
		It("TC-U015: should include next_page_token in YAML list output", func() {
			f := output.New(output.FormatYAML, stdout, stderr, nil, "")

			err := f.FormatList(sampleResources, "abc123")
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = yaml.Unmarshal(stdout.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["next_page_token"]).To(Equal("abc123"))
			results, ok := parsed["results"].([]any)
			Expect(ok).To(BeTrue())
			Expect(results).To(HaveLen(3))
		})
	})

	Describe("Format Validation", func() {
		// TC-U017: Invalid output format rejected
		It("TC-U017: should return an error for an invalid output format", func() {
			_, err := output.ParseFormat("invalid")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid output format"))
		})

		It("should accept valid formats", func() {
			for _, valid := range []string{"table", "json", "yaml"} {
				f, err := output.ParseFormat(valid)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(f)).To(Equal(valid))
			}
		})
	})

	Describe("FormatMessage", func() {
		// TC-U018: FormatMessage displays status message
		It("TC-U018: should write the message to stdout and nothing to stderr", func() {
			f := output.New(output.FormatTable, stdout, stderr, nil, "")

			err := f.FormatMessage(`Policy "my-policy" deleted successfully.`)
			Expect(err).NotTo(HaveOccurred())

			Expect(stdout.String()).To(ContainSubstring(`Policy "my-policy" deleted successfully.`))
			Expect(stderr.String()).To(BeEmpty())
		})
	})

	Describe("FormatError", func() {
		// TC-U116: Error output written to stderr (formatter-level verification)
		It("TC-U116: should write error output to stderr and not to stdout", func() {
			f := output.New(output.FormatTable, stdout, stderr, nil, "")

			err := f.FormatError(sampleProblem)
			Expect(err).NotTo(HaveOccurred())

			Expect(stdout.String()).To(BeEmpty())
			Expect(stderr.String()).NotTo(BeEmpty())
		})

		// TC-U117: FormatError renders error in table format
		It("TC-U117: should render error in table format to stderr", func() {
			f := output.New(output.FormatTable, stdout, stderr, nil, "")

			err := f.FormatError(sampleProblem)
			Expect(err).NotTo(HaveOccurred())

			errOutput := stderr.String()
			Expect(errOutput).To(ContainSubstring(`Error: NOT_FOUND - Policy "nonexistent" not found.`))
			Expect(errOutput).To(ContainSubstring("Status: 404"))
			Expect(errOutput).To(ContainSubstring("Detail: The requested policy resource does not exist."))
			Expect(stdout.String()).To(BeEmpty())
		})

		// TC-U118: FormatError renders error in JSON format
		It("TC-U118: should render the full Problem Details JSON to stderr", func() {
			f := output.New(output.FormatJSON, stdout, stderr, nil, "")

			err := f.FormatError(sampleProblem)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = json.Unmarshal(stderr.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["type"]).To(Equal("NOT_FOUND"))
			Expect(parsed["status"]).To(BeNumerically("==", 404))
			Expect(parsed["title"]).To(Equal(`Policy "nonexistent" not found.`))
			Expect(parsed["detail"]).To(Equal("The requested policy resource does not exist."))
			Expect(stdout.String()).To(BeEmpty())
		})

		// TC-U119: FormatError renders error in YAML format
		It("TC-U119: should render the full Problem Details YAML to stderr", func() {
			f := output.New(output.FormatYAML, stdout, stderr, nil, "")

			err := f.FormatError(sampleProblem)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = yaml.Unmarshal(stderr.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed["type"]).To(Equal("NOT_FOUND"))
			Expect(parsed["status"]).To(BeNumerically("==", 404))
			Expect(parsed["title"]).To(Equal(`Policy "nonexistent" not found.`))
			Expect(parsed["detail"]).To(Equal("The requested policy resource does not exist."))
			Expect(stdout.String()).To(BeEmpty())
		})
	})

	Describe("Empty List", func() {
		It("should render empty table with headers only", func() {
			f := output.New(output.FormatTable, stdout, stderr, policyTableDef(), "policy list")

			err := f.FormatList([]any{}, "")
			Expect(err).NotTo(HaveOccurred())

			out := stdout.String()
			Expect(out).To(ContainSubstring("ID"))
			Expect(out).To(ContainSubstring("DISPLAY NAME"))
		})

		It("should render empty JSON array", func() {
			f := output.New(output.FormatJSON, stdout, stderr, nil, "")

			err := f.FormatList(nil, "")
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = json.Unmarshal(stdout.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			results, ok := parsed["results"].([]any)
			Expect(ok).To(BeTrue())
			Expect(results).To(BeEmpty())
		})

		It("should render empty YAML list", func() {
			f := output.New(output.FormatYAML, stdout, stderr, nil, "")

			err := f.FormatList(nil, "")
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			err = yaml.Unmarshal(stdout.Bytes(), &parsed)
			Expect(err).NotTo(HaveOccurred())
			results, ok := parsed["results"].([]any)
			Expect(ok).To(BeTrue())
			Expect(results).To(BeEmpty())
		})
	})
})
