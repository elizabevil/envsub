package version

import (
	"encoding/json"
	"reflect"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

func TestApplyBuildInfo(t *testing.T) {
	t.Parallel()
	const revision = "50ebe6d88e9d1a2b3c4d5e6f7a8b9c0d1e2f3a4b"
	base := Info{
		GitVersion:   develVersion,
		GitTreeState: TreeStateClean,
		GoVersion:    "go1.26",
		Compiler:     "gc",
		Platform:     "linux/amd64",
	}

	tests := []struct {
		name  string
		ref   string
		built *debug.BuildInfo
		want  Info
	}{
		{
			name: "tagged install without vcs settings",
			built: &debug.BuildInfo{
				Main:      debug.Module{Version: "v1.2.3"},
				GoVersion: "go1.27.1",
			},
			want: Info{
				GitVersion:   "v1.2.3",
				GitTreeState: TreeStateClean,
				GoVersion:    "go1.27.1",
				Compiler:     "gc",
				Platform:     "linux/amd64",
			},
		},
		{
			name: "checkout build reports the revision, dirty state and settings",
			built: &debug.BuildInfo{
				Main:      debug.Module{Version: "v0.0.0-20261008071740-50ebe6d88e9d"},
				GoVersion: "go1.27.1",
				Settings: []debug.BuildSetting{
					{Key: settingRevision, Value: revision},
					{Key: settingModified, Value: "true"},
					{Key: settingCompiler, Value: "gc"},
					{Key: settingGOOS, Value: "darwin"},
					{Key: settingGOARCH, Value: "arm64"},
				},
			},
			want: Info{
				GitVersion:   "v0.0.0-20261008071740-50ebe6d88e9d",
				GitCommit:    revision,
				GitTreeState: TreeStateDirty,
				GoVersion:    "go1.27.1",
				Compiler:     "gc",
				Platform:     "darwin/arm64",
			},
		},
		{
			name:  "clean tree keeps the clean state",
			built: &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: settingModified, Value: "false"}}},
			want: Info{
				GitVersion:   develVersion,
				GitTreeState: TreeStateClean,
				GoVersion:    "go1.26",
				Compiler:     "gc",
				Platform:     "linux/amd64",
			},
		},
		{
			name: "an injected tag names the tag and the commit",
			ref:  "v0.1.0",
			built: &debug.BuildInfo{
				Main: debug.Module{Version: "v0.1.0"},
				Settings: []debug.BuildSetting{
					{Key: settingRevision, Value: revision},
				},
			},
			want: Info{
				GitVersion:   "v0.1.0-50ebe6d",
				GitCommit:    revision,
				GitTreeState: TreeStateClean,
				GoVersion:    "go1.26",
				Compiler:     "gc",
				Platform:     "linux/amd64",
			},
		},
		{
			name: "an injected branch names the branch and the commit",
			ref:  "main",
			built: &debug.BuildInfo{
				Main: debug.Module{Version: "v0.1.1-0.20261022111730-50ebe6d88e9d"},
				Settings: []debug.BuildSetting{
					{Key: settingRevision, Value: revision},
				},
			},
			want: Info{
				GitVersion:   "main-50ebe6d",
				GitCommit:    revision,
				GitTreeState: TreeStateClean,
				GoVersion:    "go1.26",
				Compiler:     "gc",
				Platform:     "linux/amd64",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := applyBuildInfo(base, tt.built, tt.ref); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("applyBuildInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestBuildVersion pins how the version string is composed: the ref and the
// commit, never a "+dirty" suffix.
func TestBuildVersion(t *testing.T) {
	t.Parallel()
	const commit = "50ebe6d88e9d1a2b3c4d5e6f7a8b9c0d1e2f3a4b"
	tests := []struct {
		name          string
		ref           string
		moduleVersion string
		commit        string
		want          string
	}{
		{name: "tag and commit", ref: "v0.1.0", commit: commit, want: "v0.1.0-50ebe6d"},
		{name: "branch and commit", ref: "main", commit: commit, want: "main-50ebe6d"},
		{name: "ref without a commit", ref: "v0.1.0", want: "v0.1.0"},
		{
			name:          "the dirty suffix is dropped from the module version",
			moduleVersion: "v0.1.0+dirty",
			commit:        commit,
			want:          "v0.1.0-50ebe6d",
		},
		{
			name:          "a pseudo-version already names the commit",
			moduleVersion: "v0.1.1-0.20261022111730-50ebe6d88e9d",
			commit:        commit,
			want:          "v0.1.1-0.20261022111730-50ebe6d88e9d",
		},
		{name: "nothing is known", want: develVersion},
		{name: "only a commit is known", commit: commit, want: "50ebe6d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := buildVersion(tt.ref, tt.moduleVersion, tt.commit); got != tt.want {
				t.Fatalf("buildVersion(%q, %q, %q) = %q, want %q",
					tt.ref, tt.moduleVersion, tt.commit, got, tt.want)
			}
		})
	}
}

func TestCurrentIsComplete(t *testing.T) {
	t.Parallel()
	info := Current()
	for name, value := range map[string]string{
		"GitVersion": info.GitVersion,
		"GoVersion":  info.GoVersion,
		"Compiler":   info.Compiler,
		"Platform":   info.Platform,
	} {
		if value == "" {
			t.Fatalf("Current().%s is empty", name)
		}
	}
	if info.GitTreeState != TreeStateClean && info.GitTreeState != TreeStateDirty {
		t.Fatalf("GitTreeState = %q, want clean or dirty", info.GitTreeState)
	}
	if info.BuildDate == "" {
		t.Skip("no build date available in this environment")
	}
	if _, err := time.Parse(time.RFC3339, info.BuildDate); err != nil {
		t.Fatalf("BuildDate = %q, want RFC3339: %v", info.BuildDate, err)
	}
}

// TestCurrentVersionHasNoDirtySuffix pins that the version string identifies
// the build by ref and commit, and leaves the dirty state to its own field.
func TestCurrentVersionHasNoDirtySuffix(t *testing.T) {
	t.Parallel()
	info := Current()
	if strings.Contains(info.GitVersion, "+dirty") {
		t.Fatalf("GitVersion = %q, want no dirty suffix", info.GitVersion)
	}
}

func TestJSONReportsTheGitVersionField(t *testing.T) {
	t.Parallel()
	data, err := JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", data, err)
	}
	version, _ := decoded["gitVersion"].(string)
	if version == "" {
		t.Fatalf("JSON() = %s, want a non-empty gitVersion field", data)
	}
}
