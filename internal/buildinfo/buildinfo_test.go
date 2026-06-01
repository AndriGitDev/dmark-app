package buildinfo

import "testing"

func TestCurrentReturnsConfiguredBuildMetadata(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() {
		Version, Commit, BuildTime = oldVersion, oldCommit, oldBuildTime
	})

	Version = "2026.06.01"
	Commit = "abcdef123"
	BuildTime = "2026-06-01T00:00:00Z"

	got := Current()
	if got.Version != Version || got.Commit != Commit || got.BuildTime != BuildTime {
		t.Fatalf("Current() = %+v, want configured build metadata", got)
	}
}

func TestCurrentFallsBackWhenBuildMetadataIsBlank(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	t.Cleanup(func() {
		Version, Commit, BuildTime = oldVersion, oldCommit, oldBuildTime
	})

	Version = " "
	Commit = ""
	BuildTime = "	"

	got := Current()
	if got.Version != "dev" || got.Commit != "unknown" || got.BuildTime != "unknown" {
		t.Fatalf("Current() = %+v, want default build metadata", got)
	}
}

func TestInfoFromVCSSettingsFillsUnknownBuildMetadata(t *testing.T) {
	got := infoFromVCSSettings(Info{Version: "dev", Commit: "unknown", BuildTime: "unknown"}, []vcsSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
		{Key: "vcs.time", Value: "2026-06-01T04:05:06Z"},
	})

	if got.Version != "dev" || got.Commit != "0123456789abcdef0123456789abcdef01234567" || got.BuildTime != "2026-06-01T04:05:06Z" {
		t.Fatalf("infoFromVCSSettings() = %+v, want VCS commit and build time fallback", got)
	}
}

func TestInfoFromVCSSettingsDoesNotOverrideExplicitLdflags(t *testing.T) {
	got := infoFromVCSSettings(Info{Version: "2026.06.01", Commit: "explicit", BuildTime: "explicit-time"}, []vcsSetting{
		{Key: "vcs.revision", Value: "0123456789abcdef0123456789abcdef01234567"},
		{Key: "vcs.time", Value: "2026-06-01T04:05:06Z"},
	})

	if got.Commit != "explicit" || got.BuildTime != "explicit-time" {
		t.Fatalf("infoFromVCSSettings() = %+v, want explicit ldflags preserved", got)
	}
}
