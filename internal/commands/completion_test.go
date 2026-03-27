package commands_test

import (
	"bytes"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/commands"
)

var _ = Describe("Completion Command", func() {
	var (
		outBuf *bytes.Buffer
		errBuf *bytes.Buffer
	)

	executeCompletion := func(args ...string) error {
		cmd := commands.NewRootCommand()
		outBuf = new(bytes.Buffer)
		errBuf = new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)
		cmd.SetArgs(args)
		return cmd.Execute()
	}

	// TC-U132: Generate bash completion script
	Describe("TC-U132: Generate bash completion", func() {
		It("should generate a valid bash completion script to stdout", func() {
			err := executeCompletion("completion", "bash")
			Expect(err).NotTo(HaveOccurred())

			output := outBuf.String()
			Expect(output).NotTo(BeEmpty())
			Expect(output).To(ContainSubstring("bash"))
		})
	})

	// TC-U133: Generate zsh completion script
	Describe("TC-U133: Generate zsh completion", func() {
		It("should generate a valid zsh completion script to stdout", func() {
			err := executeCompletion("completion", "zsh")
			Expect(err).NotTo(HaveOccurred())

			output := outBuf.String()
			Expect(output).NotTo(BeEmpty())
			Expect(output).To(SatisfyAny(
				ContainSubstring("compdef"),
				ContainSubstring("#compdef"),
			))
		})
	})

	// TC-U134: Generate fish completion script
	Describe("TC-U134: Generate fish completion", func() {
		It("should generate a valid fish completion script to stdout", func() {
			err := executeCompletion("completion", "fish")
			Expect(err).NotTo(HaveOccurred())

			output := outBuf.String()
			Expect(output).NotTo(BeEmpty())
			Expect(output).To(ContainSubstring("complete -c dcm"))
		})
	})

	// TC-U135: Generate powershell completion script
	Describe("TC-U135: Generate powershell completion", func() {
		It("should generate a valid powershell completion script to stdout", func() {
			err := executeCompletion("completion", "powershell")
			Expect(err).NotTo(HaveOccurred())

			output := outBuf.String()
			Expect(output).NotTo(BeEmpty())
			Expect(output).To(ContainSubstring("Register-ArgumentCompleter"))
		})
	})

	// TC-U136: Completion without shell argument fails
	Describe("TC-U136: Missing shell argument", func() {
		It("should return a UsageError when no shell argument is provided", func() {
			err := executeCompletion("completion")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue(),
				"expected error to be a UsageError, got: %v", err)
		})
	})

	// TC-U137: Completion with invalid shell argument fails
	Describe("TC-U137: Invalid shell argument", func() {
		It("should return a UsageError when an unsupported shell is provided", func() {
			err := executeCompletion("completion", "invalid-shell")
			Expect(err).To(HaveOccurred())

			var usageErr *commands.UsageError
			Expect(errors.As(err, &usageErr)).To(BeTrue(),
				"expected error to be a UsageError, got: %v", err)
			Expect(err.Error()).To(ContainSubstring("invalid-shell"))
		})
	})

	// TC-U138: Completion help includes usage examples
	Describe("TC-U138: Help includes usage examples", func() {
		It("should include usage examples for all supported shells", func() {
			err := executeCompletion("completion", "--help")
			Expect(err).NotTo(HaveOccurred())

			helpOutput := outBuf.String()
			Expect(helpOutput).To(ContainSubstring("bash"))
			Expect(helpOutput).To(ContainSubstring("zsh"))
			Expect(helpOutput).To(ContainSubstring("fish"))
			Expect(helpOutput).To(ContainSubstring("powershell"))
			Expect(helpOutput).To(SatisfyAny(
				ContainSubstring("PowerShell"),
				ContainSubstring("powershell"),
			))
		})
	})
})
