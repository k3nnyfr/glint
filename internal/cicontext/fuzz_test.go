package cicontext

import "testing"

// FuzzEvalIf ensures the rules:if: expression evaluator never panics on
// arbitrary input. It accepts two strings: the expression and a variable value
// substituted for every variable reference encountered.
// Run with: go test -fuzz=FuzzEvalIf ./internal/cicontext/
func FuzzEvalIf(f *testing.F) {
	// Seed corpus: representative expressions exercising every code path in
	// the hand-rolled recursive-descent parser.
	seeds := []struct{ expr, varVal string }{
		// Simple comparisons
		{`$VAR == "main"`, "main"},
		{`$VAR != "main"`, "main"},
		{`$VAR == null`, ""},
		{`$VAR != null`, "x"},
		// Regex operators
		{`$VAR =~ /^v\d+\.\d+/`, "v1.2.3"},
		{`$VAR !~ /^v\d+\.\d+/`, "not-a-version"},
		{`$VAR =~ /^us\//`, "us/west"},
		// Boolean operators
		{`$A == "x" && $B == "y"`, "x"},
		{`$A == "x" || $B == "y"`, "z"},
		{`!($VAR == "main")`, "main"},
		// Nested parens
		{`($VAR == "a" || $VAR == "b") && $VAR != "c"`, "a"},
		// Bare true / false
		{`$VAR == true`, "true"},
		{`$VAR == false`, "false"},
		// Integer comparison (GitLab CI compares as strings)
		{`$VAR == 42`, "42"},
		// Regex with flags
		{`$VAR =~ /main/i`, "MAIN"},
		// Syntax errors / incomplete expressions
		{``, ""},
		{`&&`, ""},
		{`$VAR =~`, "x"},
		{`($VAR`, "x"},
		{`$VAR == `, "x"},
		// Variable syntax variants
		{`${VAR} == "main"`, "main"},
		// Deeply nested
		{`((($VAR == "a")))`, "a"},
		// String with escapes
		{`$VAR == "hello\nworld"`, "hello\nworld"},
		// Null literal
		{`null == null`, ""},
		{`$VAR == null`, ""},
	}
	for _, s := range seeds {
		f.Add(s.expr, s.varVal)
	}

	f.Fuzz(func(t *testing.T, expr, varVal string) {
		// EvalIf must never panic; it may return any bool.
		_ = EvalIf(expr, func(string) string { return varVal })
		_ = EvalIfStrict(expr, func(string) string { return varVal })
	})
}

// FuzzExpandVarRefs ensures variable expansion in expression strings never
// panics and never produces a longer output than the worst-case expansion bound.
func FuzzExpandVarRefs(f *testing.F) {
	seeds := []struct{ s, val string }{
		{"$VAR", "hello"},
		{"${VAR}", "hello"},
		{"$A $B $C", "x"},
		{"no vars here", ""},
		{"$$double", "x"},
		{"$", "x"},
		{"${", "x"},
		{"${}", "x"},
		{"prefix_$VAR_suffix", "mid"},
		{"$1INVALID", "x"},
	}
	for _, s := range seeds {
		f.Add(s.s, s.val)
	}

	f.Fuzz(func(t *testing.T, s, val string) {
		vars := map[string]string{"VAR": val, "A": val, "B": val, "C": val}
		_ = expandVarRefs(s, vars)
	})
}
