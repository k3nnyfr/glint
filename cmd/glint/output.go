package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"git.k3nny.fr/glint/internal/linter"
)

// ANSI escape sequences used for colorized output.
const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiOrange = "\033[33m" // rendered as orange/amber in most terminals
)

// colorEnabled reports whether ANSI color should be used when writing to w.
// Colors are suppressed when NO_COLOR is set or when w is not a terminal.
func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

// writeTextFindings prints findings in a columnized format with optional
// ANSI color. All findings are scanned first to compute column widths so
// that each field aligns across all output lines.
//
// Output columns (space-separated, no borders):
//
//	location   RULE   severity   message
func writeTextFindings(w io.Writer, findings []linter.Finding) {
	if len(findings) == 0 {
		return
	}

	color := colorEnabled(w)

	// Pre-compute locations and maximum location width.
	locs := make([]string, len(findings))
	maxLoc := 0
	for i, f := range findings {
		locs[i] = findingLocation(f)
		if len(locs[i]) > maxLoc {
			maxLoc = len(locs[i])
		}
	}

	// Rule IDs are always 5 chars (GL001–GL999).
	const ruleWidth = 5
	// Severity width: "warning" = 7 chars.
	const sevWidth = 7

	for i, f := range findings {
		loc := locs[i]
		rule := f.Rule
		sev := strings.ToLower(string(f.Severity))

		msg := f.Message
		if f.Job != "" {
			msg = fmt.Sprintf("job %q: %s", f.Job, msg)
		}

		if color {
			var sevSeq string
			if f.Severity == linter.Error {
				sevSeq = ansiRed + ansiBold
			} else {
				sevSeq = ansiOrange + ansiBold
			}
			fmt.Fprintf(w, "%s%-*s%s  %s%-*s%s  %s%-*s%s  %s\n",
				ansiDim, maxLoc, loc, ansiReset,
				ansiBold, ruleWidth, rule, ansiReset,
				sevSeq, sevWidth, sev, ansiReset,
				msg,
			)
		} else {
			fmt.Fprintf(w, "%-*s  %-*s  %-*s  %s\n",
				maxLoc, loc,
				ruleWidth, rule,
				sevWidth, sev,
				msg,
			)
		}
	}
}

// findingLocation formats the file:line:col location string for a finding.
func findingLocation(f linter.Finding) string {
	if f.File == "" {
		return ""
	}
	switch {
	case f.Line > 0 && f.Column > 0:
		return fmt.Sprintf("%s:%d:%d", f.File, f.Line, f.Column)
	case f.Line > 0:
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	default:
		return f.File
	}
}
