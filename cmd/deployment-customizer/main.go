// deployment-customizer customizes llm-d YAML configuration files for use with secure-inference.
//
// Usage:
//
//	deployment-customizer gateway <yaml-file>       Customize gateway config for HTTPS
//	deployment-customizer model-service <yaml-file> Customize model service values for LoRA
package main

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	filePath := os.Args[2]

	var err error
	switch command {
	case "gateway":
		err = customizeGateway(filePath)
	case "model-service":
		err = customizeModelService(filePath)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  deployment-customizer gateway <yaml-file>        Customize gateway config for HTTPS")
	fmt.Fprintln(os.Stderr, "  deployment-customizer model-service <yaml-file>  Customize model service values for LoRA")
}

// customizeGateway replaces the gateway key with HTTPS listener configuration.
func customizeGateway(filePath string) error {
	data, err := readYAML(filePath)
	if err != nil {
		return err
	}

	if _, ok := data["gateway"]; !ok {
		return fmt.Errorf("gateway key not found in %s", filePath)
	}

	data["gateway"] = map[string]any{
		"gatewayClassName": "istio",
		"listeners": []any{
			map[string]any{
				"name":     "https",
				"port":     443,
				"protocol": "HTTPS",
				"allowedRoutes": map[string]any{
					"namespaces": map[string]any{
						"from": "All",
					},
				},
				"tls": map[string]any{
					"hostname": "llm-d.com",
					"mode":     "Terminate",
					"certificateRefs": []any{
						map[string]any{"name": "llm-d-gateway-https-cert-secret"},
					},
				},
			},
		},
	}

	return writeYAML(filePath, data)
}

// customizeModelService updates modelArtifacts and adds LoRA args to vllm containers.
func customizeModelService(filePath string) error {
	data, err := readYAML(filePath)
	if err != nil {
		return err
	}

	if ma, ok := data["modelArtifacts"].(map[string]any); ok {
		ma["uri"] = "hf://meta-llama/Llama-3.2-1B-Instruct"
		ma["name"] = "meta-llama/Llama-3.2-1B-Instruct"
		fmt.Println("Updated modelArtifacts to meta-llama/Llama-3.2-1B-Instruct")
	}

	loraArgs := []any{
		"--max-loras",
		"3",
		"--lora-modules",
		`{"name": "ibm_z17_technical_technical_introduction"}`,
		`{"name": "ansible_automation_ibm_power_env"}`,
		`{"name": "best_practices_ibm_storage_flash_system"}`,
	}

	customizeVLLMContainer(data, "decode", loraArgs)
	customizeVLLMContainer(data, "prefill", loraArgs)

	return writeYAML(filePath, data)
}

func customizeVLLMContainer(data map[string]any, section string, loraArgs []any) {
	sec, ok := data[section].(map[string]any)
	if !ok {
		return
	}
	containers, ok := sec["containers"].([]any)
	if !ok {
		return
	}
	for _, c := range containers {
		container, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if container["name"] == "vllm" {
			args := make([]any, len(loraArgs))
			copy(args, loraArgs)
			container["args"] = args
			fmt.Printf("Customized %s container with LoRA args\n", section)
			return
		}
	}
}

func readYAML(filePath string) (map[string]any, error) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}
	var data map[string]any
	if err := yaml.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", filePath, err)
	}
	return data, nil
}

func writeYAML(filePath string, data map[string]any) error {
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}
	if err := os.WriteFile(filePath, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", filePath, err)
	}
	fmt.Printf("Successfully customized %s\n", filePath)
	return nil
}
