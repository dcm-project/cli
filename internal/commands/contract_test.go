package commands_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/cli/internal/commands"
)

func docsFixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", "website", name)
}

var _ = Describe("Documentation Contract", func() {
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

	Describe("Catalog Item YAML (small-vm.yaml)", func() {
		// TC-U154: Documented catalog item YAML preserves spec.resources through CLI serialization
		It("TC-U154: should serialize spec.resources with all required fields to the API", func() {
			var receivedBody map[string]any

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-items"))

				Expect(json.NewDecoder(r.Body).Decode(&receivedBody)).To(Succeed())

				writeJSONResponse(w, http.StatusCreated, sampleCatalogItemResponse())
			}))

			err := executeCommand("catalog", "item", "create", "--from-file", docsFixturePath("small-vm.yaml"))
			Expect(err).NotTo(HaveOccurred())

			Expect(receivedBody).To(HaveKey("spec"))
			spec, ok := receivedBody["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "spec should be an object")

			Expect(spec).To(HaveKey("resources"), "spec.resources must not be silently dropped")
			resources, ok := spec["resources"].([]any)
			Expect(ok).To(BeTrue(), "spec.resources should be an array")
			Expect(resources).To(HaveLen(1))

			res0, ok := resources[0].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(res0["name"]).To(Equal("main"))
			Expect(res0["service_type"]).To(Equal("vm"))
			Expect(res0).To(HaveKey("fields"))

			fields, ok := res0["fields"].([]any)
			Expect(ok).To(BeTrue())
			Expect(len(fields)).To(BeNumerically(">=", 5), "all documented fields should be preserved")
		})
	})

	Describe("Catalog Item Instance YAML (my-vm.yaml)", func() {
		// TC-U155: Documented instance YAML preserves user_values[].resource through CLI serialization
		It("TC-U155: should serialize user_values with the resource field to the API", func() {
			var receivedBody map[string]any

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.Method).To(Equal(http.MethodPost))
				Expect(r.URL.Path).To(Equal("/api/v1alpha1/catalog-item-instances"))

				Expect(json.NewDecoder(r.Body).Decode(&receivedBody)).To(Succeed())

				writeJSONResponse(w, http.StatusCreated, sampleInstanceResponse())
			}))

			err := executeCommand("catalog", "instance", "create", "--from-file", docsFixturePath("my-vm.yaml"))
			Expect(err).NotTo(HaveOccurred())

			Expect(receivedBody).To(HaveKey("spec"))
			spec, ok := receivedBody["spec"].(map[string]any)
			Expect(ok).To(BeTrue(), "spec should be an object")

			Expect(spec).To(HaveKey("user_values"), "spec.user_values must not be silently dropped")
			userValues, ok := spec["user_values"].([]any)
			Expect(ok).To(BeTrue(), "spec.user_values should be an array")
			Expect(userValues).To(HaveLen(2))

			for i, uv := range userValues {
				uvMap, ok := uv.(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(uvMap).To(HaveKey("resource"),
					"user_values[%d].resource must not be silently dropped", i)
				Expect(uvMap["resource"]).To(Equal("main"))
			}
		})
	})
})
