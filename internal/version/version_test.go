package version

import (
	"encoding/json"
	"reflect"
	"runtime/debug"
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
				Main:      debug.Module{Version: "v0.0.0-20261008071740-fe076be703b6"},
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
				GitVersion:   "v0.0.0-20261008071740-fe076be703b6",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := applyBuildInfo(base, tt.built); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("applyBuildInfo() = %+v, want %+v", got, tt.want)
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
