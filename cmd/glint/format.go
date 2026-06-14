package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"git.k3nny.fr/glint/internal/linter"
)

// --- JSON ---

type jsonReport struct {
	SchemaVersion int           `json:"schema_version"`
	GlintVersion  string        `json:"glint_version"`
	Pipeline      string        `json:"pipeline"`
	Findings      []jsonFinding `json:"findings"`
	Summary       jsonSummary   `json:"summary"`
}

type jsonFinding struct {
	Rule     string `json:"rule,omitempty"`
	Severity string `json:"severity"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Job      string `json:"job,omitempty"`
	Message  string `json:"message"`
}

type jsonSummary struct {
	Total    int `json:"total"`
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
}

func writeJSON(w io.Writer, findings []linter.Finding, pipeline string) {
	errs, warns := countSeverities(findings)
	jf := make([]jsonFinding, 0, len(findings))
	for _, f := range findings {
		jf = append(jf, jsonFinding{
			Rule:     f.Rule,
			Severity: strings.ToLower(string(f.Severity)),
			File:     f.File,
			Line:     f.Line,
			Job:      f.Job,
			Message:  f.Message,
		})
	}
	report := jsonReport{
		SchemaVersion: 1,
		GlintVersion:  version,
		Pipeline:      pipeline,
		Findings:      jf,
		Summary:       jsonSummary{Total: len(findings), Errors: errs, Warnings: warns},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}

// --- SARIF 2.1.0 ---

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string       `json:"id"`
	ShortDescription sarifMessage `json:"shortDescription"`
	HelpURI          string       `json:"helpUri,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId,omitempty"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysLoc `json:"physicalLocation"`
}

type sarifPhysLoc struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           *sarifRegion  `json:"region,omitempty"`
}

type sarifArtifact struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

func writeSARIF(w io.Writer, findings []linter.Finding, _ string) {
	// Collect unique rule IDs (preserving first-seen order).
	seenRules := map[string]bool{}
	var rules []sarifRule
	for _, f := range findings {
		if f.Rule != "" && !seenRules[f.Rule] {
			seenRules[f.Rule] = true
			rules = append(rules, sarifRule{
				ID:               f.Rule,
				ShortDescription: sarifMessage{Text: "glint rule " + f.Rule},
				HelpURI:          "https://git.k3nny.fr/glint",
			})
		}
	}
	if rules == nil {
		rules = []sarifRule{}
	}

	var results []sarifResult
	for _, f := range findings {
		level := "warning"
		if f.Severity == linter.Error {
			level = "error"
		}
		msg := f.Message
		if f.Job != "" {
			msg = fmt.Sprintf("job %q: %s", f.Job, msg)
		}
		result := sarifResult{
			RuleID:  f.Rule,
			Level:   level,
			Message: sarifMessage{Text: msg},
		}
		if f.File != "" {
			loc := sarifLocation{
				PhysicalLocation: sarifPhysLoc{
					ArtifactLocation: sarifArtifact{URI: f.File, URIBaseID: "%SRCROOT%"},
				},
			}
			if f.Line > 0 {
				loc.PhysicalLocation.Region = &sarifRegion{StartLine: f.Line}
			}
			result.Locations = []sarifLocation{loc}
		}
		results = append(results, result)
	}
	if results == nil {
		results = []sarifResult{}
	}

	log := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "glint",
				Version:        strings.TrimPrefix(version, "v"),
				InformationURI: "https://git.k3nny.fr/glint",
				Rules:          rules,
			}},
			Results: results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(log)
}

// --- JUnit XML ---

type junitTestsuites struct {
	XMLName  xml.Name         `xml:"testsuites"`
	Name     string           `xml:"name,attr"`
	Tests    int              `xml:"tests,attr"`
	Failures int              `xml:"failures,attr"`
	Time     string           `xml:"time,attr"`
	Suites   []junitTestsuite `xml:"testsuite"`
}

type junitTestsuite struct {
	Name     string          `xml:"name,attr"`
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Time     string          `xml:"time,attr"`
	Cases    []junitTestcase `xml:"testcase"`
}

type junitTestcase struct {
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Failure   *junitFailure `xml:"failure,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

func writeJUnit(w io.Writer, findings []linter.Finding, pipeline string) {
	_, fails := countSeverities(findings)
	_ = fails // re-derive below to count both errors and warnings as failures

	var cases []junitTestcase
	if len(findings) == 0 {
		cases = []junitTestcase{{Name: "no issues found", Classname: pipeline}}
	} else {
		for _, f := range findings {
			name := f.Rule
			if name == "" {
				name = "lint"
			}
			classname := f.File
			if classname == "" {
				classname = pipeline
			}
			if f.Job != "" {
				classname += "#" + f.Job
			}
			msg := f.Message
			if f.Job != "" {
				msg = fmt.Sprintf("job %q: %s", f.Job, msg)
			}
			cases = append(cases, junitTestcase{
				Name:      name,
				Classname: classname,
				Failure: &junitFailure{
					Message: msg,
					Type:    strings.ToLower(string(f.Severity)),
					Body:    f.String(),
				},
			})
		}
	}

	suite := junitTestsuite{
		Name:     pipeline,
		Tests:    len(cases),
		Failures: len(findings), // all findings are failures
		Time:     "0",
		Cases:    cases,
	}
	suites := junitTestsuites{
		Name:     "glint",
		Tests:    len(cases),
		Failures: len(findings),
		Time:     "0",
		Suites:   []junitTestsuite{suite},
	}

	fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(w)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(suites)
	_ = enc.Flush()
	fmt.Fprintln(w)
}

// --- GitHub Actions annotations ---

// writeGitHub emits GitHub Actions workflow command annotation lines.
// Each finding becomes an ::error:: or ::warning:: line that GitHub CI
// renders as an inline comment on the relevant file in pull requests.
func writeGitHub(w io.Writer, findings []linter.Finding) {
	for _, f := range findings {
		level := "warning"
		if f.Severity == linter.Error {
			level = "error"
		}
		msg := f.Message
		if f.Job != "" {
			msg = fmt.Sprintf("job %q: %s", f.Job, msg)
		}
		// GitHub annotation messages must not contain raw newlines, percent signs,
		// carriage returns, or colons in the parameter block.
		msg = strings.ReplaceAll(msg, "%", "%25")
		msg = strings.ReplaceAll(msg, "\r", "%0D")
		msg = strings.ReplaceAll(msg, "\n", "%0A")

		var params []string
		if f.File != "" {
			params = append(params, "file="+f.File)
			if f.Line > 0 {
				params = append(params, fmt.Sprintf("line=%d", f.Line))
			}
		}
		if f.Rule != "" {
			params = append(params, "title="+f.Rule)
		}

		paramStr := ""
		if len(params) > 0 {
			paramStr = " " + strings.Join(params, ",")
		}
		fmt.Fprintf(w, "::%s%s::%s\n", level, paramStr, msg)
	}
}

// --- shared helpers ---

func countSeverities(findings []linter.Finding) (errors, warnings int) {
	for _, f := range findings {
		if f.Severity == linter.Error {
			errors++
		} else {
			warnings++
		}
	}
	return
}
