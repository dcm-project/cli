package commands

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	catalogclient "github.com/dcm-project/catalog-manager/pkg/client"
	spmclient "github.com/dcm-project/service-provider-manager/pkg/client/provider"
	sprmclient "github.com/dcm-project/service-provider-manager/pkg/client/resource_manager"

	"github.com/dcm-project/cli/internal/config"
	"github.com/dcm-project/cli/internal/output"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func newSPProviderClient(cfg *config.Config) (*spmclient.Client, error) {
	httpClient, err := buildHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return spmclient.NewClient(apiBaseURL(cfg), spmclient.WithHTTPClient(httpClient))
}

func newSPResourceClient(cfg *config.Config) (*sprmclient.Client, error) {
	httpClient, err := buildHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return sprmclient.NewClient(apiBaseURL(cfg), sprmclient.WithHTTPClient(httpClient))
}

func newCatalogClient(cfg *config.Config) (*catalogclient.Client, error) {
	httpClient, err := buildHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return catalogclient.NewClient(apiBaseURL(cfg), catalogclient.WithHTTPClient(httpClient))
}

// FormattedError indicates an error that has already been formatted and
// written to stderr. Execute() should not print it again.
type FormattedError struct{}

func (*FormattedError) Error() string { return "" }

// newFormatter creates a Formatter from the command's resolved configuration.
func newFormatter(cmd *cobra.Command, table *output.TableDef, command string) (*output.Formatter, error) {
	cfg := config.FromCommand(cmd)
	format, err := output.ParseFormat(cfg.OutputFormat)
	if err != nil {
		return nil, &UsageError{Err: err}
	}
	return output.New(format, cmd.OutOrStdout(), cmd.ErrOrStderr(), table, command), nil
}

// buildHTTPClient creates an HTTP client from the resolved configuration.
// When the API Gateway URL uses https://, TLS is configured using the
// TLS-related settings. When it uses http://, TLS settings are ignored.
func buildHTTPClient(cfg *config.Config) (*http.Client, error) {
	if !strings.HasPrefix(cfg.APIGatewayURL, "https://") {
		return &http.Client{}, nil
	}

	// Validate mTLS pair: both or neither must be set.
	if (cfg.TLSClientCert == "") != (cfg.TLSClientKey == "") {
		return nil, &UsageError{Err: fmt.Errorf("--tls-client-cert and --tls-client-key must be used together")}
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.TLSSkipVerify,
	}

	if cfg.TLSCACert != "" {
		caCert, err := os.ReadFile(cfg.TLSCACert)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate %s: %w", cfg.TLSCACert, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate %s", cfg.TLSCACert)
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.TLSClientCert != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSClientCert, cfg.TLSClientKey)
		if err != nil {
			return nil, fmt.Errorf("loading client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
	}, nil
}

// apiBaseURL returns the API base URL with the /api/v1alpha1 suffix.
func apiBaseURL(cfg *config.Config) string {
	return strings.TrimRight(cfg.APIGatewayURL, "/") + "/api/v1alpha1"
}

// parseInputFile reads a YAML or JSON file and returns its content as a map.
// Format detection attempts YAML parsing first (valid JSON is also valid YAML).
func parseInputFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}
	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing file %s: %w", path, err)
	}
	if result == nil {
		return nil, fmt.Errorf("parsing file %s: file does not contain a valid YAML/JSON object", path)
	}
	return result, nil
}

// parseInputFileAs reads a YAML or JSON file and unmarshals its content into
// the specified type. This provides client-side validation that the payload
// matches the expected schema.
func parseInputFileAs[T any](path string) (T, error) {
	var zero T
	data, err := parseInputFile(path)
	if err != nil {
		return zero, err
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return zero, fmt.Errorf("marshalling input: %w", err)
	}
	var result T
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return zero, fmt.Errorf("invalid payload in %s: %w", path, err)
	}
	return result, nil
}

// handleErrorResponse processes a non-2xx HTTP response. For RFC 7807
// responses it formats the error via the Formatter; for other responses
// it returns a plain error with the HTTP status and body.
func handleErrorResponse(resp *http.Response, formatter *output.Formatter) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading error response: %w", err)
	}

	var problem output.ProblemDetail
	if err := json.Unmarshal(body, &problem); err == nil && problem.Type != "" {
		_ = formatter.FormatError(problem)
		return &FormattedError{}
	}

	// Non-RFC-7807 error response
	return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

// requestContext returns a context with the configured timeout.
func requestContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	cfg := config.FromCommand(cmd)
	return context.WithTimeout(cmd.Context(), time.Duration(cfg.Timeout)*time.Second)
}

// connectionError wraps HTTP client errors with a user-friendly message,
// distinguishing timeout errors from connection errors.
func connectionError(err error, cfg *config.Config) error {
	if isTimeoutError(err) {
		return fmt.Errorf("request timed out after %d seconds", cfg.Timeout)
	}
	return fmt.Errorf("failed to connect to API Gateway at %s: %w", cfg.APIGatewayURL, err)
}

// isTimeoutError checks whether an error is a timeout (context deadline or net timeout).
func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr interface{ Timeout() bool }
	return errors.As(err, &netErr) && netErr.Timeout()
}

// stringifyValue extracts a string representation of a map value for table output.
func stringifyValue(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
