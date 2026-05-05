// Package version exposes build-time version info, populated via -ldflags.
package version

import (
	"fmt"
	"runtime"

	"github.com/guysoft/guyide-cli/pkg/schema"
)

// These are overridden at build time via:
//
//	go build -ldflags "-X github.com/guysoft/guyide-cli/internal/version.Version=v0.1.0
//	                   -X github.com/guysoft/guyide-cli/internal/version.Commit=abcdef
//	                   -X github.com/guysoft/guyide-cli/internal/version.BuildDate=2026-05-05"
var (
	Version   = "v0.0.0-dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Info returns a populated VersionInfo struct for JSON output.
func Info() schema.VersionInfo {
	return schema.VersionInfo{
		Envelope:  schema.Envelope{Schema: schema.SchemaVersion, Level: "info"},
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a one-line human-readable version string.
func String() string {
	return fmt.Sprintf("guyide %s (commit %s, %s, %s/%s, schema %s)",
		Version, Commit, runtime.Version(), runtime.GOOS, runtime.GOARCH, schema.SchemaVersion)
}
