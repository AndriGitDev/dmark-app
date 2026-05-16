package ops

type Drill struct {
	Name  string
	Steps []string
}

func BackupRestoreDrills() []Drill {
	return []Drill{
		{
			Name: "postgres_restore",
			Steps: []string{
				"create fresh Postgres database",
				"restore latest backup",
				"run migrations to current version",
				"verify users, domains, checks, drift events, reports, and audit logs",
				"run application smoke tests against restored database",
			},
		},
		{
			Name: "clickhouse_restore",
			Steps: []string{
				"create fresh ClickHouse database",
				"apply ClickHouse schema",
				"restore aggregate row backup or replay retained DMARC reports",
				"verify date-range reporter queries and source summaries",
			},
		},
	}
}

func LoadTestScenarios() []Drill {
	return []Drill{
		{Name: "public_checker", Steps: []string{"run representative domain checks", "verify p95 latency", "verify rate-limit behavior", "verify resolver error budget"}},
		{Name: "scan_jobs", Steps: []string{"enqueue scheduled scans", "run workers", "verify queue latency", "verify retry and dead-letter behavior"}},
		{Name: "reporter_queries", Steps: []string{"load aggregate rows", "query high-cardinality sources", "query long date ranges", "verify response time target"}},
	}
}

func DeploymentRollbackDrill() Drill {
	return Drill{
		Name: "deployment_rollback",
		Steps: []string{
			"record current deploy version, database versions, and feature flags",
			"deploy candidate release to the target environment",
			"run smoke tests and service-backed QA gates",
			"trigger rollback to the previous deploy artifact",
			"verify health, public checker, login, worker startup, and Reporter read paths",
			"confirm migrations are backward-compatible or have a documented manual recovery path",
		},
	}
}

func DependencyTriageSteps() []string {
	return []string{
		"identify affected package, version, and advisory",
		"classify reachable runtime surface",
		"prioritize critical and high reachable vulnerabilities",
		"apply update or document compensating control",
		"run automated QA gates after dependency changes",
		"record fix version and deploy date",
	}
}
