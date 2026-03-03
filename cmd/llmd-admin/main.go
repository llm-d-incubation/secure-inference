package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/llm-d-incubation/secure-inference/pkg/admin"
	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

var rootCmd = &cobra.Command{
	Use:   "llmd-admin",
	Short: "llmd-admin manages LLM-D authorization",
	Long:  `llmd-admin is a CLI tool for managing authorization in LLM-D, including certificate creation and JWT token generation`,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize llmd-admin",
	Long:  `Initialize llmd-admin for access control. Creates the necessary key-pair for LLM-D to authorize users`,
	Run: func(cmd *cobra.Command, args []string) {
		admin.CreateAdminCerts()
	},
}

var tlsCertCmd = &cobra.Command{
	Use:   "tls-cert",
	Short: "Generate a TLS certificate signed by the CA",
	Long: `Generates a TLS server certificate signed by the CA created
with 'llmd-admin init'. Requires init to have been run first.`,
	Run: func(cmd *cobra.Command, args []string) {
		dnsNamesStr, _ := cmd.Flags().GetString("dns-names") //nolint:errcheck // cobra flags registered in init
		dnsNames := splitCSV(dnsNamesStr)
		if len(dnsNames) == 0 {
			fmt.Fprintln(os.Stderr, "Error: --dns-names is required")
			os.Exit(1)
		}
		if err := admin.GenerateTLSCert(dnsNames); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating TLS certificate: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("TLS certificate generated:")
		fmt.Println("  cert: certs/tls-cert.pem")
		fmt.Println("  key:  certs/tls-key.pem")
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a JWT token for a user",
	Long:  `Creates a JWT token for a user with the specified attributes`,
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name") //nolint:errcheck // cobra flags registered in init
		role, _ := cmd.Flags().GetString("role") //nolint:errcheck // cobra flags registered in init
		org, _ := cmd.Flags().GetString("org")   //nolint:errcheck // cobra flags registered in init

		privateKeyFile := config.LlmDKeyFile
		token, err := admin.GenerateJWT(name, role, org, privateKeyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating token: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", token)
	},
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(tlsCertCmd)
	createCmd.Flags().String("name", "user1", "Name of the user")
	createCmd.Flags().String("role", "role1", "Role of the user")
	createCmd.Flags().String("org", "org1", "Organization of the user")
	tlsCertCmd.Flags().String("dns-names", "", "Comma-separated DNS names for the TLS certificate (e.g. \"llm-d.com,localhost\")")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
