package commands_test

import (
	"bytes"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/commands"
)

var _ = Describe("Root Command", func() {
	// TC-U019: Root command registers all subcommands
	Describe("TC-U019: Subcommand registration", func() {
		It("should list policy, catalog, sp, and version subcommands in help output", func() {
			cmd := commands.NewRootCommand()
			out := new(bytes.Buffer)
			cmd.SetOut(out)
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"--help"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			helpOutput := out.String()
			Expect(helpOutput).To(ContainSubstring("policy"))
			Expect(helpOutput).To(ContainSubstring("catalog"))
			Expect(helpOutput).To(ContainSubstring("sp"))
			Expect(helpOutput).To(ContainSubstring("version"))
			Expect(helpOutput).To(ContainSubstring("completion"))
		})
	})

	// TC-U020: Catalog command registers subcommand groups
	Describe("TC-U020: Catalog subcommand registration", func() {
		It("should list service-type, item, and instance subcommands in catalog help", func() {
			cmd := commands.NewRootCommand()
			out := new(bytes.Buffer)
			cmd.SetOut(out)
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"catalog", "--help"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			helpOutput := out.String()
			Expect(helpOutput).To(ContainSubstring("service-type"))
			Expect(helpOutput).To(ContainSubstring("item"))
			Expect(helpOutput).To(ContainSubstring("instance"))
		})
	})

	// TC-U129: SP command registers subcommand groups
	Describe("TC-U129: SP subcommand registration", func() {
		It("should list resource subcommand in sp help", func() {
			cmd := commands.NewRootCommand()
			out := new(bytes.Buffer)
			cmd.SetOut(out)
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"sp", "--help"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			helpOutput := out.String()
			Expect(helpOutput).To(ContainSubstring("resource"))
		})
	})

	// TC-U021: Global flags are registered
	Describe("TC-U021: Global flags", func() {
		It("should list all global flags in help output", func() {
			cmd := commands.NewRootCommand()
			out := new(bytes.Buffer)
			cmd.SetOut(out)
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"--help"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())

			helpOutput := out.String()
			expectedFlags := []string{
				"--api-gateway-url",
				"--output",
				"-o",
				"--timeout",
				"--config",
				"--tls-ca-cert",
				"--tls-client-cert",
				"--tls-client-key",
				"--tls-skip-verify",
			}
			for _, flag := range expectedFlags {
				Expect(helpOutput).To(ContainSubstring(flag),
					"expected flag %s to be listed in help output", flag)
			}
		})
	})

	// TC-U022: Exit code 0 on success
	Describe("TC-U022: Exit code on success", func() {
		It("should return no error when a command completes successfully", func() {
			cmd := commands.NewRootCommand()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"version"})

			err := cmd.Execute()
			Expect(err).NotTo(HaveOccurred())
		})
	})

	// TC-U023: Exit code 2 on usage error
	Describe("TC-U023: Exit code on usage error", func() {
		It("should return a UsageError when a required argument is missing", func() {
			cmd := commands.NewRootCommand()
			cmd.SetOut(new(bytes.Buffer))
			errBuf := new(bytes.Buffer)
			cmd.SetErr(errBuf)
			cmd.SetArgs([]string{"policy", "get"})

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue(),
				"expected error to be a UsageError, got: %v", err)
		})

		It("should return a UsageError when an unknown flag is provided", func() {
			cmd := commands.NewRootCommand()
			cmd.SetOut(new(bytes.Buffer))
			cmd.SetErr(new(bytes.Buffer))
			cmd.SetArgs([]string{"--nonexistent-flag"})

			err := cmd.Execute()
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue(),
				"expected error to be a UsageError, got: %v", err)
		})

		DescribeTable("should return a UsageError for missing positional arguments",
			func(args []string) {
				cmd := commands.NewRootCommand()
				cmd.SetOut(new(bytes.Buffer))
				cmd.SetErr(new(bytes.Buffer))
				cmd.SetArgs(args)

				err := cmd.Execute()
				Expect(err).To(HaveOccurred())

				var usageErr *commands.UsageError
				Expect(errors.As(err, &usageErr)).To(BeTrue(),
					"expected error to be a UsageError for args %v, got: %v",
					strings.Join(args, " "), err)
			},
			Entry("policy get without ID", []string{"policy", "get"}),
			Entry("policy update without ID", []string{"policy", "update"}),
			Entry("policy delete without ID", []string{"policy", "delete"}),
			Entry("catalog service-type get without ID", []string{"catalog", "service-type", "get"}),
			Entry("catalog item get without ID", []string{"catalog", "item", "get"}),
			Entry("catalog item delete without ID", []string{"catalog", "item", "delete"}),
			Entry("catalog instance get without ID", []string{"catalog", "instance", "get"}),
			Entry("catalog instance delete without ID", []string{"catalog", "instance", "delete"}),
			Entry("sp resource get without ID", []string{"sp", "resource", "get"}),
		)
	})
})
