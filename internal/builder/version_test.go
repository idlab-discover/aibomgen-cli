package builder

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
)

// helper writes a small executable "git" script into dir with the provided content.
func writeFakeGit(t *testing.T, dir, content string) {
	t.Helper()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte(content), 0755); err != nil {
		t.Fatalf("writeFakeGit: %v", err)
	}
}

func TestGetAIBoMGenVersion(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origPath := os.Getenv("PATH")
	defer func() {
		Version = origVersion
		Commit = origCommit
		_ = os.Setenv("PATH", origPath)
	}()

	tests := []struct {
		name    string
		version string
		commit  string
		noGit   bool // when true, point PATH to an empty temp dir
		want    string
	}{
		{name: "ldflags priority", version: "1.2.3-ldflags", commit: "", want: "1.2.3-ldflags"},
		{name: "commit fallback (no git)", version: "", commit: "deadbeef", noGit: true, want: "commit-deadbeef"},
		{name: "devel fallback (no version/commit/git)", version: "", commit: "", noGit: true, want: "devel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// reset globals.
			Version = ""
			Commit = ""
			Version = tt.version
			Commit = tt.commit

			if tt.noGit {
				tmp, err := os.MkdirTemp("", "no-git")
				if err != nil {
					t.Fatalf("tempdir: %v", err)
				}
				t.Cleanup(func() { _ = os.RemoveAll(tmp) })
				_ = os.Setenv("PATH", tmp)
			}

			if got := GetAIBoMGenVersion(); got != tt.want {
				t.Errorf("GetAIBoMGenVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAIBoMGenVersion_DevUsesGitDescribe(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", origPath) }()

	// put a fake git that returns a tag for describe.
	tmp, err := os.MkdirTemp("", "fake-git")
	if err != nil {
		t.Fatalf("tempdir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	script := "#!/bin/sh\nif [ \"$1\" = \"describe\" ]; then echo vX.Y.Z; exit 0; fi\nexit 1\n"
	writeFakeGit(t, tmp, script)
	_ = os.Setenv("PATH", tmp+string(os.PathListSeparator)+origPath)

	origVersion := Version
	origCommit := Commit
	defer func() { Version = origVersion; Commit = origCommit }()
	Version = "dev"
	Commit = ""

	if got := GetAIBoMGenVersion(); got != "vX.Y.Z" {
		t.Errorf("GetAIBoMGenVersion() = %v, want %v", got, "vX.Y.Z")
	}
}

func TestGetAIBoMGenVersion_ReadBuildInfo(t *testing.T) {
	orig := readBuildInfo
	defer func() { readBuildInfo = orig }()

	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Version: "v9.9.0"}}, true
	}
	origVersion := Version
	origCommit := Commit
	defer func() { Version = origVersion; Commit = origCommit }()
	Version = "dev"
	Commit = ""

	if got := GetAIBoMGenVersion(); got != "v9.9.0" {
		t.Errorf("GetAIBoMGenVersion() = %v, want %v", got, "v9.9.0")
	}
}

func Test_gitDescribe(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", origPath) }()

	tests := []struct {
		name          string
		fakeGitScript string // if non-empty, writes this script as git into a temp dir and prepends to PATH
		noGit         bool   // if true, PATH is set to an empty temp dir
		want          string
	}{
		{name: "describe succeeds", fakeGitScript: "#!/bin/sh\nif [ \"$1\" = \"describe\" ]; then echo v9.9.9; exit 0; fi\nexit 1\n", want: "v9.9.9"},
		{name: "describe fails rev-parse succeeds", fakeGitScript: "#!/bin/sh\nif [ \"$1\" = \"describe\" ]; then exit 1; fi\nif [ \"$1\" = \"rev-parse\" ]; then echo abc123; exit 0; fi\nexit 1\n", want: "abc123"},
		{name: "git missing", noGit: true, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.fakeGitScript != "" {
				tmp, err := os.MkdirTemp("", "fake-git")
				if err != nil {
					t.Fatalf("tempdir: %v", err)
				}
				t.Cleanup(func() { _ = os.RemoveAll(tmp) })
				writeFakeGit(t, tmp, tt.fakeGitScript)
				_ = os.Setenv("PATH", tmp+string(os.PathListSeparator)+origPath)
			} else if tt.noGit {
				tmp, err := os.MkdirTemp("", "no-git")
				if err != nil {
					t.Fatalf("tempdir: %v", err)
				}
				t.Cleanup(func() { _ = os.RemoveAll(tmp) })
				_ = os.Setenv("PATH", tmp)
			} else {
				_ = os.Setenv("PATH", origPath)
			}

			if got := gitDescribe(); got != tt.want {
				t.Errorf("gitDescribe() = %v, want %v", got, tt.want)
			}
		})
	}
}
