// Package buildinfo reports the identity of the running converter binary —
// the producer name, release version, and VCS revision recorded in a .tbook's
// provenance metadata (spec §3.4, manifest.meta.runs[].producer).
package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Name is the producer id written to meta.runs[].producer.name. It is a stable
// short identifier, not a display name: consumers may key on it.
const Name = "tbook-converter"

// URL is the producer's home, so a file's provenance points at the tool that
// made it.
const URL = "https://github.com/adubovskoy/tbook_converter"

// Version is the release version, "dev" for an unstamped build. Stamp it at
// build time with:
//
//	go build -ldflags "-X github.com/dimando/reader/converter/internal/buildinfo.Version=1.5.1"
var Version = "dev"

// commitLen truncates the revision to a readable-but-unambiguous prefix.
const commitLen = 12

// Commit returns the VCS revision Go embeds when the binary is built from a
// checkout, truncated to commitLen and suffixed "-dirty" when the working tree
// carried uncommitted changes. It returns "" when no revision is available
// (`go run`, a module-cache build, or -buildvcs=false).
func Commit() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return ""
	}
	if len(rev) > commitLen {
		rev = rev[:commitLen]
	}
	if dirty {
		rev += "-dirty"
	}
	return strings.ToLower(rev)
}
