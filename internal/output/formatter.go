// Package output provides output formatting for the dcm CLI.
// It supports table, JSON, and YAML output formats for rendering
// single resources, resource lists, status messages, and errors.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"go.yaml.in/yaml/v3"
)

// Format represents an output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// ValidFormats returns the list of supported output format values.
func ValidFormats() []string {
	return []string{string(FormatTable), string(FormatJSON), string(FormatYAML)}
}

// ParseFormat validates and returns a Format from a string value.
// It returns an error if the format is not supported.
func ParseFormat(s string) (Format, error) {
	switch Format(s) {
	case FormatTable, FormatJSON, FormatYAML:
		return Format(s), nil
	default:
		return "", fmt.Errorf("invalid output format %q: must be one of %s", s, strings.Join(ValidFormats(), ", "))
	}
}

// ProblemDetail represents an RFC 7807 Problem Details error.
type ProblemDetail struct {
	Type   string `json:"type"   yaml:"type"`
	Status int    `json:"status" yaml:"status"`
	Title  string `json:"title"  yaml:"title"`
	Detail string `json:"detail" yaml:"detail"`
}

// TableDef defines the table layout for a resource type.
type TableDef struct {
	// Headers are the column header names.
	Headers []string
	// RowFunc extracts column values from a single resource.
	RowFunc func(resource any) []string
}

// Formatter formats CLI output in a specific format.
type Formatter struct {
	format  Format
	out     io.Writer
	errOut  io.Writer
	table   *TableDef
	command string
}

// New creates a Formatter for the given format.
// out is the writer for success output (stdout),
// errOut is the writer for error output (stderr).
func New(format Format, out, errOut io.Writer, table *TableDef, command string) *Formatter {
	return &Formatter{
		format:  format,
		out:     out,
		errOut:  errOut,
		table:   table,
		command: command,
	}
}

// FormatOne formats a single resource.
func (f *Formatter) FormatOne(resource any) error {
	switch f.format {
	case FormatJSON:
		return f.writeJSON(f.out, resource)
	case FormatYAML:
		return f.writeYAML(f.out, resource)
	case FormatTable:
		return f.writeTable([]any{resource})
	default:
		return fmt.Errorf("unsupported format: %s", f.format)
	}
}

// ListResponse wraps a list of resources with an optional pagination token
// for JSON/YAML output.
type ListResponse struct {
	Results       []any  `json:"results"                  yaml:"results"`
	NextPageToken string `json:"next_page_token,omitempty" yaml:"next_page_token,omitempty"`
}

// FormatList formats a list of resources with optional pagination info.
func (f *Formatter) FormatList(resources []any, nextPageToken string) error {
	switch f.format {
	case FormatJSON:
		resp := ListResponse{Results: resources, NextPageToken: nextPageToken}
		if resp.Results == nil {
			resp.Results = []any{}
		}
		return f.writeJSON(f.out, resp)
	case FormatYAML:
		resp := ListResponse{Results: resources, NextPageToken: nextPageToken}
		if resp.Results == nil {
			resp.Results = []any{}
		}
		return f.writeYAML(f.out, resp)
	case FormatTable:
		if err := f.writeTable(resources); err != nil {
			return err
		}
		if nextPageToken != "" {
			_, err := fmt.Fprintf(f.out, "\nNext page: dcm %s --page-token %s\n", f.command, nextPageToken)
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", f.format)
	}
}

// FormatMessage formats a simple status message to stdout.
func (f *Formatter) FormatMessage(msg string) error {
	_, err := fmt.Fprintln(f.out, msg)
	return err
}

// FormatError formats an error to stderr.
func (f *Formatter) FormatError(problem ProblemDetail) error {
	switch f.format {
	case FormatJSON:
		return f.writeJSON(f.errOut, problem)
	case FormatYAML:
		return f.writeYAML(f.errOut, problem)
	case FormatTable:
		_, err := fmt.Fprintf(f.errOut, "Error: %s - %s\n  Status: %d\n  Detail: %s\n",
			problem.Type, problem.Title, problem.Status, problem.Detail)
		return err
	default:
		return fmt.Errorf("unsupported format: %s", f.format)
	}
}

func (f *Formatter) writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func (f *Formatter) writeYAML(w io.Writer, v any) error {
	enc := yaml.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return enc.Close()
}

func (f *Formatter) writeTable(resources []any) error {
	if f.table == nil {
		return fmt.Errorf("table definition not set")
	}
	w := tabwriter.NewWriter(f.out, 0, 0, 2, ' ', 0)
	// Write headers
	_, err := fmt.Fprintln(w, strings.Join(f.table.Headers, "\t"))
	if err != nil {
		return err
	}
	// Write rows
	for _, r := range resources {
		cols := f.table.RowFunc(r)
		_, err = fmt.Fprintln(w, strings.Join(cols, "\t"))
		if err != nil {
			return err
		}
	}
	return w.Flush()
}
