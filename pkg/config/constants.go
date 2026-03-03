package config

const (
	// ServerName is the name of the admin server.
	ServerName = "llm-d-admin"
	// CertsDirectory is the directory for storing all certs it can be changed to /etc/ssl/certs.
	CertsDirectory = "certs"
	// LlmDCAFile is the path to the certificate authority file.
	LlmDCAFile = CertsDirectory + "/llm-d-ca.pem"
	// LlmDKeyFile is the path to the private-key file.
	LlmDKeyFile = CertsDirectory + "/llm-d-key.pem"
)

// BaseDirectory returns the base path of the certificates.
func BaseDirectory() string {
	return CertsDirectory
}
