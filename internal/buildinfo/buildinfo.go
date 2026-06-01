package buildinfo

import "strings"

// These variables can be overridden at build time with -ldflags, for example:
//
//	-X github.com/AndriGitDev/meil/internal/buildinfo.Version=2026.06.01
//	-X github.com/AndriGitDev/meil/internal/buildinfo.Commit=<git-sha>
//	-X github.com/AndriGitDev/meil/internal/buildinfo.BuildTime=<rfc3339-time>
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func Current() Info {
	return Info{
		Version:   fallback(Version, "dev"),
		Commit:    fallback(Commit, "unknown"),
		BuildTime: fallback(BuildTime, "unknown"),
	}
}

func fallback(value string, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}
