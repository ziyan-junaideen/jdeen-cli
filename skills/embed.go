// Package skills exposes the agent skill bundled with the JDeen CLI.
package skills

import "embed"

// JDeenCLI contains the complete jdeen-cli skill directory.
//
//go:embed jdeen-cli
var JDeenCLI embed.FS
