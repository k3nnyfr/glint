package linter

import (
	"fmt"
	"strings"

	"git.k3nny.fr/glint/internal/model"
)

// checkInsecureRemoteInclude warns when an include: remote: entry uses plain
// HTTP (GL045). The file is still fetched and linted; the finding is a warning
// so pipelines that use HTTP for internal infra are not blocked.
func checkInsecureRemoteInclude(p *model.Pipeline) []Finding {
	var findings []Finding
	for _, inc := range p.Include {
		m, ok := inc.(map[string]any)
		if !ok {
			continue
		}
		remote, _ := m["remote"].(string)
		if strings.HasPrefix(remote, "http://") {
			findings = append(findings, Finding{
				Severity: Warning,
				Rule:     RuleInsecureRemoteInclude,
				File:     p.SourceFile,
				Message:  fmt.Sprintf("remote include %q uses plain HTTP; CI templates are fetched unencrypted — prefer HTTPS", remote),
			})
		}
	}
	return findings
}
