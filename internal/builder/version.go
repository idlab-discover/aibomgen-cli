package builder

import (
	"bytes"
	"context"
	"os/exec"
	"runtime/debug"
	"strings"
)

const aibomgenModulePath = "github.com/idlab-discover/aibomgen-cli"

var (
	// Set these at build time with -ldflags "-X 'github.com/idlab-discover/aibomgen-cli/internal/builder.Version=...' -X '...Commit=...'".
	Version = ""
	Commit  = ""
)

var readBuildInfo = debug.ReadBuildInfo

func GetAIBoMGenVersion() string {
	// 1) prefer explicit ldflags.
	if Version != "" && Version != "dev" {
		return Version
	}

	// 2) build info lookup.
	info, ok := readBuildInfo()
	if ok && info != nil {
		// When running as the CLI binary itself.
		if v := cleanVersion(info.Main.Version); v != "" {
			return v
		}

		// When running embedded as a dependency (library mode).
		if v := moduleVersionFromBuildInfo(info, aibomgenModulePath); v != "" {
			return v
		}

		// Only use git fallback when this module is the main module.
		if info.Main.Path == aibomgenModulePath {
			if d := gitDescribe(); d != "" {
				return d
			}
		}
	} else {
		// Keep old behavior if build info is unavailable.
		if d := gitDescribe(); d != "" {
			return d
		}
	}

	// 3) commit fallback.
	if Commit != "" {
		return "commit-" + Commit
	}
	return "devel"
}

func moduleVersionFromBuildInfo(info *debug.BuildInfo, modulePath string) string {
	for _, dep := range info.Deps {
		if dep == nil || dep.Path != modulePath {
			continue
		}
		if dep.Replace != nil {
			if v := cleanVersion(dep.Replace.Version); v != "" {
				return v
			}
		}
		if v := cleanVersion(dep.Version); v != "" {
			return v
		}
	}
	return ""
}

func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "(devel)" {
		return ""
	}
	return v
}

func gitDescribe() string {
	cmd := exec.CommandContext(context.Background(), "git", "describe", "--tags", "--always", "--dirty")
	out, err := cmd.Output()
	if err != nil {
		cmd2 := exec.CommandContext(context.Background(), "git", "rev-parse", "--short", "HEAD")
		if out2, err2 := cmd2.Output(); err2 == nil {
			return strings.TrimSpace(string(out2))
		}
		return ""
	}
	return strings.TrimSpace(string(bytes.TrimSpace(out)))
}
