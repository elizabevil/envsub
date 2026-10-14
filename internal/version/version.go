// Package version reports the build information of the running binary.
//
// The Go toolchain embeds the module version, VCS revision, tree state,
// compiler and platform, so those need no build flags. It deliberately does not
// embed a build timestamp, so BuildDate comes from the modification time of the
// executable instead of a build-time -ldflags injection.
package version

import (
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/elizabevil/envsub/internal/jsonx"
)

// Build settings read from the embedded build info.
const (
	settingRevision = "vcs.revision"
	settingModified = "vcs.modified"
	settingCompiler = "-compiler"
	settingGOOS     = "GOOS"
	settingGOARCH   = "GOARCH"
)

// Tree states reported in Info.GitTreeState.
const (
	TreeStateClean = "clean"
	TreeStateDirty = "dirty"
)

// develVersion is the toolchain placeholder for a build without a module
// version, for example a plain `go build` from a checkout.
const develVersion = "(devel)"

// Info is the build information `envsub version` reports. The field set follows
// the conventional Go build-info shape used by Kubernetes and GoReleaser.
type Info struct {
	// GitVersion is the module version; it carries the commit for a checkout
	// build and is "(devel)" when no version is embedded.
	GitVersion string `json:"gitVersion"`
	// GitCommit is the full VCS revision the binary was built from.
	GitCommit string `json:"gitCommit"`
	// GitTreeState is clean or dirty, depending on uncommitted changes.
	GitTreeState string `json:"gitTreeState"`
	// BuildDate is the RFC3339 build time, empty when it cannot be determined.
	BuildDate string `json:"buildDate"`
	// GoVersion is the toolchain that produced the binary.
	GoVersion string `json:"goVersion"`
	// Compiler is the toolchain compiler, usually gc.
	Compiler string `json:"compiler"`
	// Platform is GOOS/GOARCH.
	Platform string `json:"platform"`
}

// Current reports the build information of the running binary.
func Current() Info {
	info := Info{
		GitVersion:   develVersion,
		GitTreeState: TreeStateClean,
		BuildDate:    currentBuildDate(),
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
	}
	built, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	return applyBuildInfo(info, built)
}

// applyBuildInfo copies the values the toolchain embedded over the fallbacks.
func applyBuildInfo(info Info, built *debug.BuildInfo) Info {
	if built.Main.Version != "" {
		info.GitVersion = built.Main.Version
	}
	if built.GoVersion != "" {
		info.GoVersion = built.GoVersion
	}
	goos, goarch := runtime.GOOS, runtime.GOARCH
	for _, setting := range built.Settings {
		switch setting.Key {
		case settingRevision:
			info.GitCommit = setting.Value
		case settingModified:
			if setting.Value == "true" {
				info.GitTreeState = TreeStateDirty
			}
		case settingCompiler:
			if setting.Value != "" {
				info.Compiler = setting.Value
			}
		case settingGOOS:
			if setting.Value != "" {
				goos = setting.Value
			}
		case settingGOARCH:
			if setting.Value != "" {
				goarch = setting.Value
			}
		}
	}
	info.Platform = goos + "/" + goarch
	return info
}

// currentBuildDate returns the modification time of the running binary, the
// only build timestamp available without injecting one at build time.
func currentBuildDate() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	info, err := os.Stat(executable)
	if err != nil {
		return ""
	}
	return info.ModTime().UTC().Format(time.RFC3339)
}

// JSON renders the current build information as an indented JSON object.
func JSON() ([]byte, error) {
	return jsonx.MarshalIndent(Current())
}
