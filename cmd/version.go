package cmd

import (
	"runtime/debug"
	"strings"
)

// version is the binary's version. It defaults to "dev" and is overridden at
// build time via:
//
//	-ldflags "-X github.com/totalizator/sshc/cmd.version=v1.2.3"
//
// (the Makefile derives the value from `git describe`).
var version = "dev"

// versionString returns the resolved version. When the binary was not stamped
// at build time, it falls back to Go module/VCS build info so that
// `go install ...@vX` and plain `go build` still report something meaningful.
func versionString() string {
	if version != "dev" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version
	}

	// A version is embedded when installed via `go install pkg@vX.Y.Z`.
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}

	// Otherwise reconstruct from the VCS stamp the Go toolchain embeds.
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		b := strings.Builder{}
		b.WriteString("dev+")
		b.WriteString(rev)
		if dirty {
			b.WriteString("-dirty")
		}
		return b.String()
	}
	return version
}
