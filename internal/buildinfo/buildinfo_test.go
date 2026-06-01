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
	BuildTime = "\t"

	got := Current()
	if got.Version != "dev" || got.Commit != "unknown" || got.BuildTime != "unknown" {
		t.Fatalf("Current() = %+v, want default build metadata", got)
	}
}
