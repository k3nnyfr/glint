package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"git.k3nny.fr/glint/internal/linter"
)

func cmdExplain(args []string) {
	if len(args) == 0 {
		printRuleList()
		return
	}
	ruleID := strings.ToUpper(args[0])
	entry, ok := linter.RuleCatalog[ruleID]
	if !ok {
		fmt.Fprintf(os.Stderr, "glint explain: unknown rule %q\n\nRun 'glint explain' to list all rules.\n", ruleID)
		exit(2)
		return
	}
	printRuleEntry(ruleID, entry)
}

func printRuleEntry(id string, e linter.RuleEntry) {
	sev := strings.ToLower(string(e.Severity))
	header := fmt.Sprintf("%s  [%s]  %s", id, sev, e.Title)
	rule := fmt.Sprintf("\n%s\n%s\n\n%s\n",
		header,
		strings.Repeat("─", len(header)),
		e.Description,
	)
	fmt.Print(rule)

	if e.Example != "" {
		fmt.Println("Example:")
		fmt.Println()
		for _, line := range strings.Split(e.Example, "\n") {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println()
	}

	if e.Fix != "" {
		fmt.Println("Fix:")
		fmt.Println()
		for _, line := range strings.Split(e.Fix, "\n") {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println()
	}
}

func printRuleList() {
	ids := make([]string, 0, len(linter.RuleCatalog))
	for id := range linter.RuleCatalog {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	fmt.Printf("glint %s — lint rules\n\n", version)
	for _, id := range ids {
		e := linter.RuleCatalog[id]
		sev := strings.ToLower(string(e.Severity))
		fmt.Printf("  %-6s  [%-7s]  %s\n", id, sev, e.Title)
	}
	fmt.Println("\nUse 'glint explain <RULE>' for details on a specific rule.")
}
