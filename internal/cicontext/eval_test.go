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

// TestEvalIfParserEdgeCases exercises low-level parser branches not reached
// by the main table-driven tests above.
func TestEvalIfParserEdgeCases(t *testing.T) {
	vars := func(key string) string {
		switch key {
		case "BRANCH":
			return "develop"
		case "UNTERMINATED_RE":
			return "/no-end"
		case "ESCAPED_RE":
			return `/^dev\./`
		}
		return ""
	}

	// parseOr: right side of || fails to parse.
	if !EvalIf(`$BRANCH || !(`, vars) {
		t.Error("parseOr bad right: expect permissive-true")
	}
	if EvalIfStrict(`$BRANCH || !(`, vars) {
		t.Error("parseOr bad right: expect strict-false")
	}

	// parsePrimary: inner parseOr succeeds but no closing ')'.
	if !EvalIf(`($BRANCH == "develop"`, vars) {
		t.Error("unmatched paren: expect permissive-true")
	}
	if EvalIfStrict(`($BRANCH == "develop"`, vars) {
		t.Error("unmatched paren: expect strict-false")
	}

	// parseComparison ==: right-hand parseValue fails (! is not a valid value start).
	if !EvalIf(`$BRANCH == !invalid`, vars) {
		t.Error("== bad rhs: expect permissive-true")
	}

	// parseComparison !=: right-hand parseValue fails.
	if !EvalIf(`$BRANCH != ${`, vars) {
		t.Error("!= bad rhs: expect permissive-true")
	}

	// parseRegexRHS: peek is neither '/' nor '$' → return "", false, false.
	// parseComparison =~: patOk=false, permissive=false → return false, false.
	if !EvalIf(`$BRANCH =~ "literal"`, vars) {
		t.Error("=~ string literal rhs: expect permissive-true")
	}
	if EvalIfStrict(`$BRANCH =~ "literal"`, vars) {
		t.Error("=~ string literal rhs: expect strict-false")
	}

	// parseComparison =~: bad regex pattern → compile error → return true, true.
	if !EvalIf(`$BRANCH =~ /[unclosed/`, vars) {
		t.Error("=~ bad regex: expect permissive-true")
	}

	// parseComparison !~: patOk=false → return false, false.
	if !EvalIf(`$BRANCH !~ "literal"`, vars) {
		t.Error("!~ string literal rhs: expect permissive-true")
	}
	if EvalIfStrict(`$BRANCH !~ "literal"`, vars) {
		t.Error("!~ string literal rhs: expect strict-false")
	}

	// parseComparison !~: bad regex pattern → compile error → return true, true.
	if !EvalIf(`$BRANCH !~ /[unclosed/`, vars) {
		t.Error("!~ bad regex: expect permissive-true")
	}

	// parseComparison single =: RHS parseValue fails.
	if !EvalIf(`$BRANCH = !invalid`, vars) {
		t.Error("single = bad rhs: expect permissive-true")
	}

	// parseValue: '$' not followed by a valid identifier.
	if !EvalIf(`$} == "develop"`, vars) {
		t.Error("$ bad ident: expect permissive-true")
	}

	// parseValue: '${' with no closing '}' or empty name.
	if !EvalIf(`${} == "develop"`, vars) {
		t.Error("${} empty name: expect permissive-true")
	}
	if !EvalIf(`${BRANCH == "develop"`, vars) {
		t.Error("${BRANCH no close brace: expect permissive-true")
	}

	// parseStringLiteral: escape sequence — \v is consumed and the next byte
	// is written literally, so "de\velop" → "develop" which matches BRANCH.
	if !EvalIf(`$BRANCH == "de\velop"`, vars) {
		t.Error(`string escape: "de\velop" should decode to "develop"`)
	}

	// parseStringLiteral: unterminated string literal.
	if !EvalIf(`$BRANCH == "no-end`, vars) {
		t.Error("unterminated string: expect permissive-true")
	}
	if EvalIfStrict(`$BRANCH == "no-end`, vars) {
		t.Error("unterminated string: expect strict-false")
	}

	// parseRegexLiteral: escape sequence in regex (backslash preserved).
	// /^d\evelop/ → pattern "^d\evelop" — \e is invalid in Go regexp
	// → compile error → permissive true.
	if !EvalIf(`$BRANCH =~ /^d\evelop/`, vars) {
		t.Error("regex escape (bad compile): expect permissive-true")
	}

	// parseRegexLiteral: unterminated regex (no closing '/').
	if !EvalIf(`$BRANCH =~ /no-end`, vars) {
		t.Error("unterminated regex: expect permissive-true")
	}
	if EvalIfStrict(`$BRANCH =~ /no-end`, vars) {
		t.Error("unterminated regex: expect strict-false")
	}

	// applyRegexFlags: 'm' and 's' flags (exercises two additional switch cases).
	if !EvalIf(`$BRANCH =~ /^DEV/ims`, vars) {
		t.Error("regex /ims flags: case-insensitive should match 'develop'")
	}

	// extractRegexFromString: unterminated regex in variable value.
	// UNTERMINATED_RE = "/no-end" (no closing '/') → permissive.
	if !EvalIf(`$BRANCH =~ $UNTERMINATED_RE`, vars) {
		t.Error("unterminated re in var: expect permissive-true")
	}

	// extractRegexFromString: escape sequence in variable value.
	// ESCAPED_RE = /^dev\./  → pattern = "^dev\." (literal dot) → no match for "develop".
	if EvalIf(`$BRANCH =~ $ESCAPED_RE`, vars) {
		t.Error("ESCAPED_RE=/^dev\\./ requires a literal dot; 'develop' has no dot")
	}

	// !~ permissive path (line 203-205): var value is a plain string, not /regex/ → permissive.
	// BRANCH = "develop" which does not start with '/' → extractRegexFromString returns ok=false
	// → parseRegexRHS returns permissive=true → !~ case returns (true, true).
	if !EvalIf(`$BRANCH !~ $BRANCH`, vars) {
		t.Error("!~ plain-var rhs: plain variable value triggers permissive-true")
	}

	// parseRegexRHS: '$' in rhs but parseValue fails (line 244-246).
	// '$}' — '$' followed by '}' which is not a valid identifier start → varOk=false.
	if !EvalIf(`$BRANCH =~ $}`, vars) {
		t.Error("=~ $}: parseValue fails → permissive-true (EvalIf overall)")
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
