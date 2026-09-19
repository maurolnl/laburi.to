package jobposition

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func readMigrationUp(t *testing.T, path string) string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", path, err)
	}

	return strings.SplitN(string(contents), "-- +goose Down", 2)[0]
}

func TestMigration0006JobPositionContract(t *testing.T) {
	up := readMigrationUp(t, "../../sql/schema/0006_job_positions.sql")

	patterns := map[string]string{
		"belongs to an employer and cascades": `(?is)employer_id\s+INTEGER\s+NOT\s+NULL\s+REFERENCES\s+employers\(id\)\s+ON\s+DELETE\s+CASCADE`,
		"limits required experience":          `(?is)CONSTRAINT\s+job_positions_required_experience_check\s+CHECK\s*\(\s*required_experience\s+IN\s*\(\s*'less_1y'\s*,\s*'1y'\s*,\s*'2_to_5y'\s*,\s*'5_to_10y'\s*,\s*'more_10y'\s*\)\s*\)`,
		"limits required education level":     `(?is)CONSTRAINT\s+job_positions_required_education_level_check\s+CHECK\s*\(\s*required_education_level\s+IN\s*\(\s*'university'\s*,\s*'postgraduate'\s*,\s*'high-school-orientation'\s*,\s*'tertiary'\s*\)\s*\)`,
		"limits available hours per day":      `(?is)CONSTRAINT\s+job_positions_available_hours_per_day_check\s+CHECK\s*\(\s*available_hours_per_day\s+BETWEEN\s+1\s+AND\s+8\s*\)`,
		"defaults technical resources":        `(?is)technical_resources\s+TEXT\[\]\s+NOT\s+NULL\s+DEFAULT\s+'\{\}'`,
		"keeps deleted_at nullable":           `(?is)deleted_at\s+TIMESTAMPTZ\s*,`,
	}

	for name, pattern := range patterns {
		t.Run(name, func(t *testing.T) {
			if !regexp.MustCompile(pattern).MatchString(up) {
				t.Fatalf("migration 0006 no longer satisfies contract %q", name)
			}
		})
	}

	if regexp.MustCompile(`(?is)deleted_at\s+TIMESTAMPTZ\s+NOT\s+NULL`).MatchString(up) {
		t.Fatal("deleted_at must stay nullable: a NOT NULL column cannot represent an active job position")
	}
}

func TestMigration0006ExcludesDeletedFromQueries(t *testing.T) {
	up := readMigrationUp(t, "../../sql/schema/0006_job_positions.sql")

	indexes := []string{
		"job_positions_employer_id_idx",
		"job_positions_required_experience_idx",
		"job_positions_required_education_level_idx",
		"job_positions_timezone_idx",
	}

	for _, index := range indexes {
		t.Run(index+" is partial", func(t *testing.T) {
			pattern := `(?is)CREATE\s+INDEX\s+` + index + `\s+ON\s+job_positions\([a-z_]+\)\s+WHERE\s+deleted_at\s+IS\s+NULL`
			if !regexp.MustCompile(pattern).MatchString(up) {
				t.Fatalf("index %s must exist and be partial on deleted_at IS NULL", index)
			}
		})
	}

	if count := strings.Count(up, "WHERE deleted_at IS NULL"); count != len(indexes) {
		t.Fatalf("%d partial clauses found, want %d: every index must exclude deleted job positions", count, len(indexes))
	}

	for _, forbidden := range []string{"status", "published", "state"} {
		t.Run("has no "+forbidden+" column", func(t *testing.T) {
			pattern := `(?im)^\s+` + forbidden + `\s+[A-Z]`
			if regexp.MustCompile(pattern).MatchString(up) {
				t.Fatalf("job positions are published on creation: column %q reintroduces a publication state", forbidden)
			}
		})
	}
}

func TestMigration0006SharesEmployeeDomains(t *testing.T) {
	jobPositions := readMigrationUp(t, "../../sql/schema/0006_job_positions.sql")
	employees := readMigrationUp(t, "../../sql/schema/0001_employees.sql")

	domains := map[string]string{
		"years of experience": `'less_1y', '1y', '2_to_5y', '5_to_10y', 'more_10y'`,
		"education level":     `'university', 'postgraduate', 'high-school-orientation', 'tertiary'`,
	}

	for name, domain := range domains {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(employees, domain) {
				t.Fatalf("employee schema no longer declares the %s domain %q", name, domain)
			}
			if !strings.Contains(jobPositions, domain) {
				t.Fatalf("job position schema diverged from the employee %s domain %q", name, domain)
			}
		})
	}
}

func TestJobPositionQueriesExcludeDeleted(t *testing.T) {
	contents, err := os.ReadFile("../../sql/queries/job_positions.sql")
	if err != nil {
		t.Fatalf("read job position queries: %v", err)
	}

	blocks := map[string]string{}
	for _, block := range strings.Split(string(contents), "-- name: ")[1:] {
		name := strings.Fields(block)[0]
		blocks[name] = block
	}

	for _, name := range []string{
		"GetActiveJobPositionByID",
		"ListActiveJobPositionsByEmployer",
		"UpdateActiveJobPosition",
		"SoftDeleteJobPosition",
	} {
		t.Run(name+" filters deleted rows", func(t *testing.T) {
			block, ok := blocks[name]
			if !ok {
				t.Fatalf("query %s is missing", name)
			}
			if !strings.Contains(block, "deleted_at IS NULL") {
				t.Fatalf("query %s must exclude soft deleted job positions", name)
			}
		})
	}

	t.Run("soft delete is idempotent", func(t *testing.T) {
		block := blocks["SoftDeleteJobPosition"]
		pattern := `(?is)SET\s+deleted_at\s*=\s*now\(\).*WHERE\s+id\s*=\s*\$1\s+AND\s+deleted_at\s+IS\s+NULL`
		if !regexp.MustCompile(pattern).MatchString(block) {
			t.Fatal("soft delete must only stamp rows that are still active, so a second delete affects no rows")
		}
	})

	if strings.Contains(blocks["CreateJobPosition"], "deleted_at IS NULL") {
		t.Fatal("create must not filter on deleted_at")
	}
}
