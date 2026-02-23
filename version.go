package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var rawVersion string

// Get returns the application version string.
func Get() string {
	return strings.TrimSpace(rawVersion)
}
