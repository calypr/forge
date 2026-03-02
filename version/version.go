// Package version reports the Forge version.
package version

import "fmt"

// Build and version details
var (
	GitCommit   = ""
	GitBranch   = ""
	GitUpstream = ""
	BuildDate   = "03/2026"
	Version     = "0.1.0"
)

var tpl = `git commit: %s
git branch: %s
git upstream: %s
build date: %s
version: %s`

// String formats a string with version details.
func String() string {
	return fmt.Sprintf(tpl, GitCommit, GitBranch, GitUpstream, BuildDate, Version)
}

// LogFields logs build and version information to the given logger.
func LogFields() []any {
	return []any{
		"GitCommit", GitCommit,
		"GitBranch", GitBranch,
		"GitUpstream", GitUpstream,
		"BuildDate", BuildDate,
		"Version", Version,
	}
}
