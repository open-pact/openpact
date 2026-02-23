package version

import _ "embed"

//go:embed VERSION
var version string

func Get() string {
	return version
}
