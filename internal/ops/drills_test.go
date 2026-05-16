package ops

import "testing"

func TestBackupRestoreDrills(t *testing.T) {
	t.Parallel()

	drills := BackupRestoreDrills()
	if len(drills) != 2 {
		t.Fatalf("len(BackupRestoreDrills()) = %d, want 2", len(drills))
	}
	for _, drill := range drills {
		if drill.Name == "" || len(drill.Steps) == 0 {
			t.Fatalf("invalid drill: %+v", drill)
		}
	}
}

func TestLoadTestScenarios(t *testing.T) {
	t.Parallel()

	scenarios := LoadTestScenarios()
	if len(scenarios) < 3 {
		t.Fatalf("len(LoadTestScenarios()) = %d, want at least 3", len(scenarios))
	}
}

func TestDeploymentRollbackDrill(t *testing.T) {
	t.Parallel()

	drill := DeploymentRollbackDrill()
	if drill.Name != "deployment_rollback" {
		t.Fatalf("Name = %q, want deployment_rollback", drill.Name)
	}
	if len(drill.Steps) < 5 {
		t.Fatalf("rollback drill has %d steps, want actionable checklist", len(drill.Steps))
	}
}

func TestDependencyTriageSteps(t *testing.T) {
	t.Parallel()

	steps := DependencyTriageSteps()
	if len(steps) < 5 {
		t.Fatalf("len(DependencyTriageSteps()) = %d, want actionable checklist", len(steps))
	}
}
