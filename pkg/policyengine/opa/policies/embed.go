package policies

import _ "embed"

// AccessPolicy contains the Rego policy for access control decisions.
// This policy is embedded at compile time and loaded into OPA at runtime.
//
//go:embed access.rego
var AccessPolicy string
