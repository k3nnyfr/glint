package main

import (
	"strings"

	"git.k3nny.fr/glint/internal/config"
	"git.k3nny.fr/glint/internal/linter"
)

// applyConfig filters and adjusts findings according to the project config
// and inline suppression comments parsed from the pipeline YAML.
//
// Processing order:
//  1. Build a combined ignore set from config.Ignore and any severity entry
//     whose value is "ignore".
//  2. Drop findings whose rule is in the ignore set.
//  3. Drop findings suppressed by an inline "# glint: ignore" comment on the
//     job definition (from p.Suppressions).
//  4. Apply severity overrides ("error" / "warning") from config.Severity.
func applyConfig(findings []linter.Finding, cfg config.Config, suppressions map[string][]string) []linter.Finding {
	// Build the global ignore set (uppercased rule IDs).
	ignoreSet := make(map[string]bool, len(cfg.Ignore))
	for _, r := range cfg.Ignore {
		ignoreSet[strings.ToUpper(r)] = true
	}
	// Severity entries with value "ignore" are equivalent to Ignore entries.
	sevMap := make(map[string]string, len(cfg.Severity)) // upperRule → lowerLevel
	for rule, sev := range cfg.Severity {
		upper := strings.ToUpper(rule)
		lower := strings.ToLower(sev)
		sevMap[upper] = lower
		if lower == "ignore" {
			ignoreSet[upper] = true
		}
	}

	if len(ignoreSet) == 0 && len(sevMap) == 0 && len(suppressions) == 0 {
		return findings
	}

	kept := findings[:0:0] // reuse underlying array but return fresh slice
	for _, f := range findings {
		ruleUpper := strings.ToUpper(f.Rule)

		// Global ignore.
		if ignoreSet[ruleUpper] {
			continue
		}

		// Inline suppression.
		if isSuppressed(f.Job, ruleUpper, suppressions) {
			continue
		}

		// Severity override.
		if level, ok := sevMap[ruleUpper]; ok {
			switch level {
			case "error":
				f.Severity = linter.Error
			case "warning":
				f.Severity = linter.Warning
			}
		}

		kept = append(kept, f)
	}
	return kept
}

// isSuppressed reports whether jobName has a "# glint: ignore" directive that
// covers ruleUpper. The wildcard entry "*" (from "# glint: ignore all")
// suppresses every rule.
func isSuppressed(jobName, ruleUpper string, suppressions map[string][]string) bool {
	if jobName == "" || len(suppressions) == 0 {
		return false
	}
	rules, ok := suppressions[jobName]
	if !ok {
		return false
	}
	for _, r := range rules {
		if r == "*" || r == ruleUpper {
			return true
		}
	}
	return false
}
