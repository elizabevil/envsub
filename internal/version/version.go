// Package version reports the build information of the running binary.
//
// The Go toolchain embeds the module version, VCS revision, tree state,
// compiler and platform, so those need no build flags. It deliberately does not
// embed a build timestamp, so BuildDate comes from the modification time of the
// executable instead of a build-time -ldflags injection.
//
// The version identifies the build by the git tag or branch it came from plus
// the commit id. The toolchain only stamps the module version, which carries a
// "+dirty" suffix and a pseudo-version instead of the branch name, so the ref
// is injected at build time; a build without the injection falls back to what
// the embedded build info carries.
package version

import (
	"os"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/elizabevil/envsub/internal/jsonx"
	"github.com/elizabevil/envsub/internal/timefmt"
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

// shortCommitLength is the number of characters a shortened commit keeps.
const shortCommitLength = 7

// gitRef is the git tag or branch the binary was built from. The build recipes
// inject it with -ldflags; it is empty in a build that did not.
//
// It is a variable rather than a constant so the linker can set it: the ref is
// the only piece of build information the toolchain does not embed.
var gitRef = ""

// Info is the build information `envsub version` reports. The field set follows
// the conventional Go build-info shape used by Kubernetes and GoReleaser.
type Info struct {
	// GitVersion identifies the build: the git tag or branch it came from
	// joined with the short commit id, for example "v0.1.0-a215676" or
	// "main-a215676". The dirty state is reported separately, so the version
	// string never carries a "+dirty" suffix.
	GitVersion string `json:"gitVersion"`
	// GitCommit is the full VCS revision the binary was built from.
	GitCommit string `json:"gitCommit"`
	// GitTreeState is clean or dirty, depending on uncommitted changes.
	GitTreeState string `json:"gitTreeState"`
	// BuildDate is the build time in the process-wide UTC RFC3339 layout with
	// milliseconds, empty when it cannot be determined.
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
	return applyBuildInfo(info, built, gitRef)
}

// applyBuildInfo copies the values the toolchain embedded over the fallbacks.
// The ref is the tag or branch the build came from; it is empty when the build
// did not inject one.
func applyBuildInfo(info Info, built *debug.BuildInfo, ref string) Info {
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
	info.GitVersion = buildVersion(ref, built.Main.Version, info.GitCommit)
	return info
}

// buildVersion composes the version string from the ref and the commit.
//
// The ref is the injected tag or branch, falling back to the module version
// the toolchain embedded. The commit is appended so the version names one
// exact build; a module version that already carries the commit (a
// pseudo-version) is used as it is.
func buildVersion(ref, moduleVersion, commit string) string {
	if ref == "" {
		// The module version carries a "+dirty" suffix when the tree had
		// uncommitted changes. The dirty state is reported on its own, so the
		// suffix is dropped here.
		ref = strings.TrimSuffix(moduleVersion, "+dirty")
	}
	short := shortCommit(commit)
	switch {
	case ref == "" || ref == develVersion:
		if short == "" {
			return develVersion
		}
		return short
	case short == "":
		return ref
	case strings.Contains(ref, short):
		// A pseudo-version already names the commit.
		return ref
	default:
		return ref + "-" + short
	}
}

// shortCommit shortens a full revision to the customary seven characters.
func shortCommit(commit string) string {
	if len(commit) <= shortCommitLength {
		return commit
	}
	return commit[:shortCommitLength]
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
	return timefmt.Format(info.ModTime())
}

// JSON renders the current build information as an indented JSON object.
func JSON() ([]byte, error) {
	return jsonx.MarshalIndent(Current())
}
