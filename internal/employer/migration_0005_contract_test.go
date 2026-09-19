package employer

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestMigration0005RoleAndProfileContract(t *testing.T) {
	contents, err := os.ReadFile("../../sql/schema/0005_add_user_roles_and_employers.sql")
	if err != nil {
		t.Fatalf("read migration 0005: %v", err)
	}
	up := strings.SplitN(string(contents), "-- +goose Down", 2)[0]
	up = regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(up, "")

	addRolePattern := `(?is)(?:^|;)\s*ALTER\s+TABLE\s+users\s+ADD\s+COLUMN\s+role\s+TEXT\s*;`
	backfillPattern := `(?is)(?:^|;)\s*UPDATE\s+users\s+SET\s+role\s*=\s*'employer'\s*;`
	requireRolePattern := `(?is)(?:^|;)\s*ALTER\s+TABLE\s+users\s+ALTER\s+COLUMN\s+role\s+SET\s+NOT\s+NULL\s*;`

	patterns := map[string]string{
		"adds role column":                        addRolePattern,
		"backfills historical users as employers": backfillPattern,
		"limits role domain":                      `(?is)CONSTRAINT\s+users_role_check\s+CHECK\s*\(\s*role\s+IN\s*\(\s*'employee'\s*,\s*'employer'\s*\)\s*\)`,
		"requires role":                           requireRolePattern,
		"runs immutable role trigger":             `(?is)CREATE\s+TRIGGER\s+prevent_user_role_change\s+BEFORE\s+UPDATE\s+OF\s+role\s+ON\s+users\s+FOR\s+EACH\s+ROW\s+EXECUTE\s+FUNCTION\s+prevent_user_role_change\s*\(\s*\)`,
		"keeps one employer per user":             `(?is)CONSTRAINT\s+unique_employer_by_user\s+UNIQUE\s*\(\s*user_id\s*\)`,
		"guards employee profiles":                `(?is)CREATE\s+TRIGGER\s+enforce_exclusive_employee_profile\s+BEFORE\s+INSERT\s+OR\s+UPDATE\s+OF\s+user_id\s+ON\s+employees\s+FOR\s+EACH\s+ROW\s+EXECUTE\s+FUNCTION\s+enforce_exclusive_user_profile\s*\(\s*\)`,
		"guards employer profiles":                `(?is)CREATE\s+TRIGGER\s+enforce_exclusive_employer_profile\s+BEFORE\s+INSERT\s+OR\s+UPDATE\s+OF\s+user_id\s+ON\s+employers\s+FOR\s+EACH\s+ROW\s+EXECUTE\s+FUNCTION\s+enforce_exclusive_user_profile\s*\(\s*\)`,
	}

	for name, pattern := range patterns {
		t.Run(name, func(t *testing.T) {
			if !regexp.MustCompile(pattern).MatchString(up) {
				t.Fatalf("migration 0005 no longer satisfies contract %q", name)
			}
		})
	}
	addRoleAt := regexp.MustCompile(addRolePattern).FindStringIndex(up)
	backfillAt := regexp.MustCompile(backfillPattern).FindStringIndex(up)
	requireRoleAt := regexp.MustCompile(requireRolePattern).FindStringIndex(up)
	if addRoleAt == nil || backfillAt == nil || requireRoleAt == nil || !(addRoleAt[0] < backfillAt[0] && backfillAt[0] < requireRoleAt[0]) {
		t.Fatal("migration 0005 must add role, backfill historical users, then make role required")
	}

	functionBody := func(name string) string {
		t.Helper()
		pattern := regexp.MustCompile(`(?is)CREATE\s+FUNCTION\s+` + regexp.QuoteMeta(name) + `\s*\(\s*\).*?AS\s+\$\$(.*?)\$\$;`)
		matches := pattern.FindStringSubmatch(up)
		if len(matches) != 2 {
			t.Fatalf("migration 0005 does not define function %s with an extractable body", name)
		}
		return matches[1]
	}

	immutableRoleBody := functionBody("prevent_user_role_change")
	if !regexp.MustCompile(`(?is)IF\s+NEW\.role\s+IS\s+DISTINCT\s+FROM\s+OLD\.role\s+THEN.*RAISE\s+EXCEPTION.*ERRCODE\s*=\s*'23514'`).MatchString(immutableRoleBody) {
		t.Fatal("prevent_user_role_change no longer rejects role changes with SQLSTATE 23514")
	}

	exclusiveProfileBody := functionBody("enforce_exclusive_user_profile")
	if !regexp.MustCompile(`(?is)pg_advisory_xact_lock\s*\(\s*1818\s*,\s*NEW\.user_id\s*\)`).MatchString(exclusiveProfileBody) {
		t.Fatal("enforce_exclusive_user_profile no longer uses the shared transactional advisory lock")
	}
	exclusivityBranches := map[string]struct {
		branchPattern string
		guardPattern  string
	}{
		"employee against employer": {
			branchPattern: `(?is)IF\s+TG_TABLE_NAME\s*=\s*'employees'\s+THEN(.*?)ELSIF`,
			guardPattern:  `(?is)FROM\s+employers\s+WHERE\s+user_id\s*=\s*NEW\.user_id.*ERRCODE\s*=\s*'23514'`,
		},
		"employer against employee": {
			branchPattern: `(?is)ELSIF\s+TG_TABLE_NAME\s*=\s*'employers'\s+THEN(.*?)END\s+IF`,
			guardPattern:  `(?is)FROM\s+employees\s+WHERE\s+user_id\s*=\s*NEW\.user_id.*ERRCODE\s*=\s*'23514'`,
		},
	}
	for direction, contract := range exclusivityBranches {
		matches := regexp.MustCompile(contract.branchPattern).FindStringSubmatch(exclusiveProfileBody)
		if len(matches) != 2 || !regexp.MustCompile(contract.guardPattern).MatchString(matches[1]) {
			t.Fatalf("enforce_exclusive_user_profile no longer enforces %s exclusivity", direction)
		}
	}

	if count := strings.Count(up, "EXECUTE FUNCTION enforce_exclusive_user_profile()"); count != 2 {
		t.Fatalf("exclusive profile function is used by %d triggers, want 2 sharing the same advisory lock", count)
	}
}
