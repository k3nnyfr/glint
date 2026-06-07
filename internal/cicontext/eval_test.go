package cicontext

import "testing"

func TestEvalIf(t *testing.T) {
	vars := func(key string) string {
		m := map[string]string{
			"CI_COMMIT_BRANCH": "develop",
			"CI_COMMIT_TAG":    "",
			"CI_PIPELINE_SOURCE": "push",
			"DEPLOY_ENV":       "staging",
		}
		return m[key]
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		// ── Equality ──────────────────────────────────────────────────────────
		{"eq match", `$CI_COMMIT_BRANCH == "develop"`, true},
		{"eq no match", `$CI_COMMIT_BRANCH == "main"`, false},
		{"eq single-quote", `$CI_COMMIT_BRANCH == 'develop'`, true},
		{"neq match", `$CI_COMMIT_BRANCH != "main"`, true},
		{"neq no match", `$CI_COMMIT_BRANCH != "develop"`, false},

		// ── Null checks ───────────────────────────────────────────────────────
		{"tag undefined eq null", `$CI_COMMIT_TAG == null`, true},
		{"tag undefined neq null", `$CI_COMMIT_TAG != null`, false},
		{"branch defined neq null", `$CI_COMMIT_BRANCH != null`, true},
		{"branch defined eq null", `$CI_COMMIT_BRANCH == null`, false},
		{"null eq null", `null == null`, true},

		// ── Variable truthiness (no operator) ────────────────────────────────
		{"truthy branch", `$CI_COMMIT_BRANCH`, true},
		{"falsy tag", `$CI_COMMIT_TAG`, false},

		// ── Logical NOT ───────────────────────────────────────────────────────
		{"not false", `!$CI_COMMIT_TAG`, true},
		{"not true", `!$CI_COMMIT_BRANCH`, false},
		{"double not", `!!$CI_COMMIT_BRANCH`, true},

		// ── Boolean AND ───────────────────────────────────────────────────────
		{"and both true", `$CI_COMMIT_BRANCH == "develop" && $CI_PIPELINE_SOURCE == "push"`, true},
		{"and left false", `$CI_COMMIT_BRANCH == "main" && $CI_PIPELINE_SOURCE == "push"`, false},
		{"and right false", `$CI_COMMIT_BRANCH == "develop" && $CI_PIPELINE_SOURCE == "schedule"`, false},
		{"and both false", `$CI_COMMIT_BRANCH == "main" && $CI_PIPELINE_SOURCE == "schedule"`, false},

		// ── Boolean OR ────────────────────────────────────────────────────────
		{"or both true", `$CI_COMMIT_BRANCH == "develop" || $CI_COMMIT_BRANCH == "main"`, true},
		{"or left true", `$CI_COMMIT_BRANCH == "develop" || $CI_COMMIT_BRANCH == "nope"`, true},
		{"or right true", `$CI_COMMIT_BRANCH == "nope" || $CI_COMMIT_BRANCH == "develop"`, true},
		{"or both false", `$CI_COMMIT_BRANCH == "main" || $CI_COMMIT_BRANCH == "nope"`, false},

		// ── Parentheses ───────────────────────────────────────────────────────
		{"paren or+and", `($CI_COMMIT_BRANCH == "develop" || $CI_COMMIT_BRANCH == "main") && $CI_COMMIT_TAG == null`, true},
		{"paren or+and false", `($CI_COMMIT_BRANCH == "main" || $CI_COMMIT_BRANCH == "nope") && $CI_COMMIT_TAG == null`, false},

		// ── Regex match ───────────────────────────────────────────────────────
		{"regex match", `$CI_COMMIT_BRANCH =~ /^dev/`, true},
		{"regex no match", `$CI_COMMIT_BRANCH =~ /^main/`, false},
		{"regex not match", `$CI_COMMIT_BRANCH !~ /^main/`, true},
		{"regex not no match", `$CI_COMMIT_BRANCH !~ /^dev/`, false},
		{"regex case sensitive", `$CI_COMMIT_BRANCH =~ /^DEV/`, false},
		{"regex feat branch", `$CI_COMMIT_BRANCH =~ /^feat\//`, false},

		// ── Extra whitespace ──────────────────────────────────────────────────
		{"extra spaces", `  $CI_COMMIT_BRANCH  ==  "develop"  `, true},
		{"tabs", "$CI_COMMIT_BRANCH\t==\t\"develop\"", true},

		// ── Permissive fallback ───────────────────────────────────────────────
		{"unparseable returns true", `this is not valid syntax %%%`, true},
		{"empty expr returns true", ``, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalIf(tc.expr, vars)
			if got != tc.want {
				t.Errorf("EvalIf(%q) = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}
