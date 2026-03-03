package admin

import (
	"fmt"
	"os"

	"github.com/llm-d-incubation/secure-inference/pkg/config"
)

// CreateAdminCerts creates the admin CA certificate and private key.
func CreateAdminCerts() {
	fmt.Printf("Creating LLM-D Admin CA Cert.\n")
	if err := os.MkdirAll(config.BaseDirectory(), 0o755); err != nil {
		fmt.Printf("Unable to create directory :%v\n", err)
		return
	}
	err := CreateCertificate(&CertificateConfig{
		Name:              config.ServerName,
		IsCA:              true,
		CertOutPath:       config.LlmDCAFile,
		PrivateKeyOutPath: config.LlmDKeyFile,
	})
	if err != nil {
		fmt.Printf("Unable to generate CA certficate :%v\n", err)
		return
	}
}

// GenerateTLSCert creates a TLS certificate signed by the existing CA.
// The CA must have been created first via CreateAdminCerts (llmd-admin init).
func GenerateTLSCert(dnsNames []string) error {
	if err := os.MkdirAll(config.BaseDirectory(), 0o755); err != nil {
		return fmt.Errorf("unable to create directory: %w", err)
	}
	return CreateCertificate(&CertificateConfig{
		Name:              "llm-d-gateway",
		IsServer:          true,
		DNSNames:          dnsNames,
		CAPath:            config.LlmDCAFile,
		CAKeyPath:         config.LlmDKeyFile,
		CertOutPath:       config.CertsDirectory + "/tls-cert.pem",
		PrivateKeyOutPath: config.CertsDirectory + "/tls-key.pem",
	})
}
