#!/usr/bin/env python3
"""Tests for the backend service-backed RC gate helper."""

from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "rc_backend_services.sh"
MAKEFILE = ROOT / "Makefile"


class BackendServiceRCGateTests(unittest.TestCase):
    def test_script_runs_required_service_backed_gates_without_silent_skips(self):
        body = SCRIPT.read_text()
        required_snippets = [
            "MEIL_DB_INTEGRATION=1",
            "go test ./internal/store/pg -count=1",
            "go run ./cmd/meil migrate postgres up",
            "go run ./cmd/meil migrate clickhouse up",
            "CLICKHOUSE_URL",
            "DATABASE_URL",
            "redact",
            "require_command",
        ]
        for snippet in required_snippets:
            with self.subTest(snippet=snippet):
                self.assertIn(snippet, body)

    def test_script_refuses_to_report_service_gate_success_without_real_services(self):
        body = SCRIPT.read_text()
        self.assertIn("RC_BACKEND_SERVICES_NO_COMPOSE", body)
        self.assertRegex(body, r"exit 1")
        self.assertIn("Missing required command", body)
        self.assertIn("using external services", body)

    def test_redaction_masks_database_passwords_without_hiding_usernames(self):
        body = SCRIPT.read_text()
        self.assertIn(r"(postgres://[^:/@]+:)[^@]+@", body)
        self.assertIn(r"\1[REDACTED]@", body)
        self.assertNotIn("postgres://[^:]+@", body)

    def test_makefile_exposes_rc_backend_services_target(self):
        makefile = MAKEFILE.read_text()
        self.assertRegex(makefile, r"\.PHONY:.*rc-backend-services")
        self.assertRegex(makefile, r"(?m)^rc-backend-services:\n\t./scripts/rc_backend_services\.sh\s*$")
    def test_makefile_allows_lint_runner_override_for_ci_equivalent_hosts(self):
        makefile = MAKEFILE.read_text()
        self.assertIn("GOLANGCI_LINT ?= golangci-lint", makefile)
        self.assertIn("$(GOLANGCI_LINT) run", makefile)
        self.assertIn("GOVULNCHECK ?= govulncheck", makefile)
        self.assertIn("$(GOVULNCHECK) ./...", makefile)
        self.assertIn("SQLC ?= sqlc", makefile)
        self.assertIn("$(SQLC) generate", makefile)


if __name__ == "__main__":
    unittest.main()
