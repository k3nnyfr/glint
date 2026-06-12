package cicontext

import "testing"

func TestEvalIf(t *testing.T) {
	vars := func(key string) string {
		m := map[string]string{
			"CI_COMMIT_BRANCH":  "develop",
			"CI_COMMIT_TAG":     "",
			"CI_PIPELINE_SOURCE": "push",
			"DEPLOY_ENV":        "staging",
			"BRANCH_PATTERN":    "/^dev/",
			"BRANCH_PATTERN_CI": "/^DEV/i",
			"EMPTY_PATTERN":     "",
			"PLAIN_PATTERN":     "develop",
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

		// ── Multi-line expressions (newlines between tokens) ──────────────────
		{"multiline or true", "$CI_COMMIT_BRANCH == \"develop\" ||\n$CI_COMMIT_TAG != null", true},
		{"multiline or false", "$CI_COMMIT_BRANCH == \"main\" ||\n$CI_COMMIT_TAG != null", false},
		{"multiline and true", "$CI_COMMIT_BRANCH == \"develop\" &&\n$CI_PIPELINE_SOURCE == \"push\"", true},
		{"multiline and false", "$CI_COMMIT_BRANCH == \"main\" &&\n$CI_PIPELINE_SOURCE == \"push\"", false},
		{"multiline with crlf", "$CI_COMMIT_BRANCH == \"develop\" ||\r\n$CI_COMMIT_TAG != null", true},

		// ── ${VAR} curly-brace syntax ─────────────────────────────────────────
		{"curly var eq match", `${CI_COMMIT_BRANCH} == "develop"`, true},
		{"curly var eq no match", `${CI_COMMIT_BRANCH} == "main"`, false},
		{"curly var truthiness", `${CI_COMMIT_BRANCH}`, true},
		{"curly var falsy", `${CI_COMMIT_TAG}`, false},
		{"curly var neq null", `${CI_COMMIT_BRANCH} != null`, true},
		{"curly mixed", `${CI_COMMIT_BRANCH} == "develop" && $CI_PIPELINE_SOURCE == "push"`, true},

		// ── Regex flags (/pattern/i etc.) ─────────────────────────────────────
		{"regex flag i match", `$CI_COMMIT_BRANCH =~ /^DEV/i`, true},
		{"regex flag i no match", `$CI_COMMIT_BRANCH =~ /^MAIN/i`, false},
		{"regex flag i not match", `$CI_COMMIT_BRANCH !~ /^MAIN/i`, true},
		{"regex no flag case sensitive", `$CI_COMMIT_BRANCH =~ /^DEV/`, false},
		{"regex flag i version tag", `$CI_PIPELINE_SOURCE =~ /^PUSH$/i`, true},

		// ── Variable on right side of =~ ──────────────────────────────────────
		{"var regex rhs match", `$CI_COMMIT_BRANCH =~ $BRANCH_PATTERN`, true},
		{"var regex rhs no match", `$CI_PIPELINE_SOURCE =~ $BRANCH_PATTERN`, false},
		{"var regex rhs ci flag match", `$CI_COMMIT_BRANCH =~ $BRANCH_PATTERN_CI`, true},
		{"var regex rhs empty permissive", `$CI_COMMIT_BRANCH =~ $EMPTY_PATTERN`, true},
		{"var regex rhs plain permissive", `$CI_COMMIT_BRANCH =~ $PLAIN_PATTERN`, true},
		{"var regex rhs not match", `$CI_COMMIT_BRANCH !~ $BRANCH_PATTERN`, false},

		// ── Bare true/false keywords ─────────────────────────────────────────
		// GitLab CI treats true/false as the string values "true"/"false".
		{"bare true match", `$CI_PIPELINE_SOURCE == true`, false},       // "push" != "true"
		{"bare false match", `$CI_COMMIT_TAG == false`, false},           // "" != "false"
		{"bare true var set to true", `$DEPLOY_ENV == true`, false},      // "staging" != "true"
		{"bare false neq", `$CI_COMMIT_BRANCH != false`, true},           // "develop" != "false"
		{"bare true in compound", `$CI_COMMIT_BRANCH != null && $CI_COMMIT_TAG == false`, false},

		// ── Integer literals ──────────────────────────────────────────────────
		// Compared as decimal strings (GitLab CI converts integers to strings).
		{"int eq match", `$CI_PIPELINE_SOURCE != 0`, true},  // "push" != "0"
		{"int eq no match", `$CI_COMMIT_TAG == 0`, false},    // "" != "0"
		{"int in compound", `$CI_COMMIT_BRANCH != null && $CI_COMMIT_BRANCH != 0`, true},

		// ── Permissive fallback ───────────────────────────────────────────────
		{"unparseable returns true", `this is not valid syntax %%%`, true},
		{"empty expr returns true", ``, true},

		// ── Single = as alias for == ──────────────────────────────────────────
		{"single eq match", `$CI_COMMIT_BRANCH = "develop"`, true},
		{"single eq no match", `$CI_COMMIT_BRANCH = "main"`, false},
		{"single eq in compound", `$CI_COMMIT_BRANCH = "develop" && $CI_PIPELINE_SOURCE = "push"`, true},
		{"single eq compound false", `$CI_COMMIT_BRANCH = "main" && $CI_PIPELINE_SOURCE = "push"`, false},
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

func TestEvalIfStrict(t *testing.T) {
	vars := func(key string) string {
		m := map[string]string{
			"CI_COMMIT_BRANCH":   "develop",
			"CI_PIPELINE_SOURCE": "push",
			"WORKFLOW":           "",
		}
		return m[key]
	}

	tests := []struct {
		name string
		expr string
		want bool
	}{
		// Parseable expressions behave identically to EvalIf.
		{"parseable match", `$CI_COMMIT_BRANCH == "develop"`, true},
		{"parseable no match", `$CI_COMMIT_BRANCH == "main"`, false},
		{"single eq match", `$CI_COMMIT_BRANCH = "develop"`, true},
		// Empty expression: ruleIfMatchesStrict handles the empty→true case
		// before calling EvalIfStrict, so empty falls through to false here.
		{"empty expr", ``, false},

		// Unparseable expressions return false (strict) instead of true (permissive).
		{"unparseable returns false", `this is not valid syntax %%%`, false},

		// The key workflow-rule scenario: a complex condition with an
		// unevaluable sub-expression should not match (strict=false) so that
		// later workflow rules can be evaluated.
		{"workflow rule complex no match", `$WORKFLOW = "gitflow" && $CI_PIPELINE_SOURCE == /(push|web)/`, false},

		// Compound with a bad second operand: strict returns false.
		{"and with bad rhs strict false", `$CI_COMMIT_BRANCH == "develop" && !(((`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EvalIfStrict(tc.expr, vars)
			if got != tc.want {
				t.Errorf("EvalIfStrict(%q) = %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
}
