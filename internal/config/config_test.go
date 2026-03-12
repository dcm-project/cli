package config_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/commands"
	"github.com/dcm-project/cli/internal/config"
)

// clearDCMEnvVars removes all DCM_* environment variables to isolate tests.
func clearDCMEnvVars() {
	envVars := []string{
		"DCM_API_GATEWAY_URL",
		"DCM_OUTPUT_FORMAT",
		"DCM_TIMEOUT",
		"DCM_CONFIG",
		"DCM_TLS_CA_CERT",
		"DCM_TLS_CLIENT_CERT",
		"DCM_TLS_CLIENT_KEY",
		"DCM_TLS_SKIP_VERIFY",
	}
	for _, env := range envVars {
		Expect(os.Unsetenv(env)).To(Succeed())
	}
}

// writeConfigFile creates a temporary config file with the given YAML content.
func writeConfigFile(content string) string {
	dir := GinkgoT().TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(content), 0o600)
	Expect(err).NotTo(HaveOccurred())
	return path
}

var _ = Describe("Configuration", func() {
	BeforeEach(func() {
		clearDCMEnvVars()
	})

	// TC-U001: Load configuration from config file
	Describe("TC-U001: Config file loading", func() {
		It("should load api-gateway-url from config file", func() {
			cfgPath := writeConfigFile("api-gateway-url: http://custom:9080\n")
			cmd := commands.NewRootCommand()
			cmd.SetArgs([]string{"--config", cfgPath, "version"})
			cmd.SetOut(GinkgoWriter)
			cmd.SetErr(GinkgoWriter)
			_ = cmd.Execute()

			cfg, err := config.Load(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.APIGatewayURL).To(Equal("http://custom:9080"))
		})
	})

	// TC-U002: Environment variable overrides config file
	Describe("TC-U002: Env var overrides config file", func() {
		It("should use environment variable over config file value", func() {
			cfgPath := writeConfigFile("api-gateway-url: http://file:9080\n")
			GinkgoT().Setenv("DCM_API_GATEWAY_URL", "http://env:9080")

			cmd := commands.NewRootCommand()
			cmd.SetArgs([]string{"--config", cfgPath, "version"})
			cmd.SetOut(GinkgoWriter)
			cmd.SetErr(GinkgoWriter)
			_ = cmd.Execute()

			cfg, err := config.Load(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.APIGatewayURL).To(Equal("http://env:9080"))
		})
	})

	// TC-U003: CLI flag overrides environment variable and config file
	Describe("TC-U003: CLI flag overrides env var and config file", func() {
		It("should use CLI flag over environment variable and config file", func() {
			cfgPath := writeConfigFile("api-gateway-url: http://file:9080\n")
			GinkgoT().Setenv("DCM_API_GATEWAY_URL", "http://env:9080")

			cmd := commands.NewRootCommand()
			cmd.SetArgs([]string{
				"--config", cfgPath,
				"--api-gateway-url", "http://flag:9080",
				"version",
			})
			cmd.SetOut(GinkgoWriter)
			cmd.SetErr(GinkgoWriter)
			_ = cmd.Execute()

			cfg, err := config.Load(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.APIGatewayURL).To(Equal("http://flag:9080"))
		})
	})

	// TC-U004: Default values applied when no config specified
	Describe("TC-U004: Built-in defaults", func() {
		It("should apply default values when no config file, env vars, or flags are set", func() {
			cfgPath := filepath.Join(GinkgoT().TempDir(), "nonexistent.yaml")
			cmd := commands.NewRootCommand()
			cmd.SetArgs([]string{"--config", cfgPath, "version"})
			cmd.SetOut(GinkgoWriter)
			cmd.SetErr(GinkgoWriter)
			_ = cmd.Execute()

			cfg, err := config.Load(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.APIGatewayURL).To(Equal("http://localhost:9080"))
			Expect(cfg.OutputFormat).To(Equal("table"))
			Expect(cfg.Timeout).To(Equal(30))
			Expect(cfg.TLSCACert).To(BeEmpty())
			Expect(cfg.TLSClientCert).To(BeEmpty())
			Expect(cfg.TLSClientKey).To(BeEmpty())
			Expect(cfg.TLSSkipVerify).To(BeFalse())
		})
	})

	// TC-U005: Missing config file does not cause failure
	Describe("TC-U005: Missing config file", func() {
		It("should not fail when the config file does not exist", func() {
			cfgPath := filepath.Join(GinkgoT().TempDir(), "does-not-exist.yaml")
			cmd := commands.NewRootCommand()
			cmd.SetArgs([]string{"--config", cfgPath, "version"})
			cmd.SetOut(GinkgoWriter)
			cmd.SetErr(GinkgoWriter)
			_ = cmd.Execute()

			cfg, err := config.Load(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.APIGatewayURL).To(Equal("http://localhost:9080"))
		})
	})

	// TC-U006: Custom config file path via --config flag
	Describe("TC-U006: Custom config file via --config", func() {
		It("should load configuration from a custom file path", func() {
			cfgPath := writeConfigFile("timeout: 60\n")
			cmd := commands.NewRootCommand()
			cmd.SetArgs([]string{"--config", cfgPath, "version"})
			cmd.SetOut(GinkgoWriter)
			cmd.SetErr(GinkgoWriter)
			_ = cmd.Execute()

			cfg, err := config.Load(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Timeout).To(Equal(60))
		})
	})

	// TC-U007: Custom config file path via DCM_CONFIG environment variable
	Describe("TC-U007: Custom config file via DCM_CONFIG", func() {
		It("should load configuration from DCM_CONFIG path", func() {
			cfgPath := writeConfigFile("timeout: 45\n")
			GinkgoT().Setenv("DCM_CONFIG", cfgPath)

			cmd := commands.NewRootCommand()
			cmd.SetArgs([]string{"version"})
			cmd.SetOut(GinkgoWriter)
			cmd.SetErr(GinkgoWriter)
			_ = cmd.Execute()

			cfg, err := config.Load(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.Timeout).To(Equal(45))
		})
	})

	// TC-U008: All environment variables are supported
	Describe("TC-U008: All environment variables", func() {
		DescribeTable("should load configuration from each environment variable",
			func(envVar, envValue, configField string, expected any) {
				cfgPath := filepath.Join(GinkgoT().TempDir(), "nonexistent.yaml")
				GinkgoT().Setenv(envVar, envValue)

				cmd := commands.NewRootCommand()
				cmd.SetArgs([]string{"--config", cfgPath, "version"})
				cmd.SetOut(GinkgoWriter)
				cmd.SetErr(GinkgoWriter)
				_ = cmd.Execute()

				cfg, err := config.Load(cmd)
				Expect(err).NotTo(HaveOccurred())

				switch configField {
				case "APIGatewayURL":
					Expect(cfg.APIGatewayURL).To(Equal(expected))
				case "OutputFormat":
					Expect(cfg.OutputFormat).To(Equal(expected))
				case "Timeout":
					Expect(cfg.Timeout).To(Equal(expected))
				case "TLSCACert":
					Expect(cfg.TLSCACert).To(Equal(expected))
				case "TLSClientCert":
					Expect(cfg.TLSClientCert).To(Equal(expected))
				case "TLSClientKey":
					Expect(cfg.TLSClientKey).To(Equal(expected))
				case "TLSSkipVerify":
					Expect(cfg.TLSSkipVerify).To(Equal(expected))
				}
			},
			Entry("DCM_API_GATEWAY_URL", "DCM_API_GATEWAY_URL", "http://e:9080", "APIGatewayURL", "http://e:9080"),
			Entry("DCM_OUTPUT_FORMAT", "DCM_OUTPUT_FORMAT", "json", "OutputFormat", "json"),
			Entry("DCM_TIMEOUT", "DCM_TIMEOUT", "60", "Timeout", 60),
			Entry("DCM_TLS_CA_CERT", "DCM_TLS_CA_CERT", "/path/ca.pem", "TLSCACert", "/path/ca.pem"),
			Entry("DCM_TLS_CLIENT_CERT", "DCM_TLS_CLIENT_CERT", "/path/cert.pem", "TLSClientCert", "/path/cert.pem"),
			Entry("DCM_TLS_CLIENT_KEY", "DCM_TLS_CLIENT_KEY", "/path/key.pem", "TLSClientKey", "/path/key.pem"),
			Entry("DCM_TLS_SKIP_VERIFY", "DCM_TLS_SKIP_VERIFY", "true", "TLSSkipVerify", true),
		)
	})
})
