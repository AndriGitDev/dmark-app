package buildinfo

import (
	"runtime/debug"
	"strings"
)

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
	info := Info{
		Version:   fallback(Version, "dev"),
		Commit:    fallback(Commit, "unknown"),
		BuildTime: fallback(BuildTime, "unknown"),
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		settings := make([]vcsSetting, 0, len(build.Settings))
		for _, setting := range build.Settings {
			settings = append(settings, vcsSetting{Key: setting.Key, Value: setting.Value})
		}
		info = infoFromVCSSettings(info, settings)
	}
	return info
}

type vcsSetting struct {
	Key   string
	Value string
}

func infoFromVCSSettings(info Info, settings []vcsSetting) Info {
	values := map[string]string{}
	for _, setting := range settings {
		values[setting.Key] = strings.TrimSpace(setting.Value)
	}
	if info.Commit == "unknown" && values["vcs.revision"] != "" {
		info.Commit = values["vcs.revision"]
	}
	if info.BuildTime == "unknown" && values["vcs.time"] != "" {
		info.BuildTime = values["vcs.time"]
	}
	return info
}

func fallback(value string, defaultValue string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}
	return value
}
