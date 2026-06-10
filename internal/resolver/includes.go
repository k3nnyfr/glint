package resolver

import (
	"fmt"
	"strings"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/model"
)

// IncludeWarning describes a remote include entry that could not be resolved.
// These are surfaced to the user as [WARNING] lines before the lint findings.
type IncludeWarning struct {
	// Label is a short human-readable identifier shown in the warning message,
	// e.g. "project my-group/templates:/ci.yml@main" or
	// "component gitlab.com/components/golang/build@v1.0".
	Label   string
	Err     error // nil when Skipped is true
	Skipped bool  // no token available — skipped without attempting a network call
}

func (w IncludeWarning) String() string {
	if w.Skipped {
		return fmt.Sprintf(
			"%s: skipped — no GitLab token found (set GITLAB_TOKEN, CI_JOB_TOKEN, or GITLAB_PRIVATE_TOKEN); findings may be incomplete",
			w.Label)
	}
	return fmt.Sprintf(
		"%s: could not be fetched — %v; findings may be incomplete or contain false positives",
		w.Label, w.Err)
}

// ResolveIncludes processes the pipeline's include: block.
//
// Project includes (include: project: ...) require authentication and are
// skipped with a warning when no token is configured.
//
// Component includes (include: component: ...) attempt the fetch
// unauthenticated first, so public CI/CD catalog components work without a
// token. A warning is emitted when the fetch fails.
//
// Non-resolvable include forms (local, remote, template) are silently skipped.
func ResolveIncludes(p *model.Pipeline, cfg fetcher.GitLabConfig) []IncludeWarning {
	var warnings []IncludeWarning
	for _, inc := range p.Include {
		entry, ok := normaliseInclude(inc)
		if !ok {
			continue
		}

		if project, _ := entry["project"].(string); project != "" {
			warnings = append(warnings, resolveProjectInclude(p, entry, project, cfg)...)
			continue
		}

		if compRef, _ := entry["component"].(string); compRef != "" {
			if w, ok := resolveComponentInclude(p, compRef, cfg); ok {
				warnings = append(warnings, w)
			}
			continue
		}

		// local, remote, template — resolved by GitLab at runtime, skip silently.
	}
	return warnings
}

// resolveProjectInclude fetches all files listed under a single project: entry
// and merges them into p.
func resolveProjectInclude(p *model.Pipeline, entry map[string]any, project string, cfg fetcher.GitLabConfig) []IncludeWarning {
	ref, _ := entry["ref"].(string)
	var warnings []IncludeWarning

	for _, filePath := range includeFiles(entry) {
		label := fmt.Sprintf("project %s:%s", project, filePath)
		if ref != "" {
			label += "@" + ref
		}

		if !cfg.HasToken() {
			warnings = append(warnings, IncludeWarning{Label: label, Skipped: true})
			continue
		}

		data, err := cfg.FetchFile(project, filePath, ref)
		if err != nil {
			warnings = append(warnings, IncludeWarning{Label: label, Err: err})
			continue
		}

		included, err := model.ParseBytes(data)
		if err != nil {
			warnings = append(warnings, IncludeWarning{Label: label, Err: fmt.Errorf("parsing YAML: %w", err)})
			continue
		}

		mergeIncluded(p, included)
	}
	return warnings
}

// resolveComponentInclude fetches a CI/CD catalog component and merges it into p.
// Returns (warning, true) if something went wrong; (zero, false) on success.
func resolveComponentInclude(p *model.Pipeline, ref string, cfg fetcher.GitLabConfig) (IncludeWarning, bool) {
	label := "component " + ref

	// Variable interpolation (e.g. $CI_SERVER_FQDN) cannot be resolved locally.
	if strings.ContainsRune(ref, '$') {
		return IncludeWarning{
			Label: label,
			Err:   fmt.Errorf("component reference contains a CI variable that cannot be resolved at lint time"),
		}, true
	}

	host, project, component, version, err := parseComponentRef(ref)
	if err != nil {
		return IncludeWarning{Label: label, Err: err}, true
	}

	hostCfg := cfg.ForHost(host)
	data, err := fetchComponentFile(hostCfg, project, component, version)
	if err != nil {
		return IncludeWarning{Label: label, Err: err}, true
	}

	included, err := model.ParseBytes(data)
	if err != nil {
		return IncludeWarning{Label: label, Err: fmt.Errorf("parsing component YAML: %w", err)}, true
	}

	mergeIncluded(p, included)
	return IncludeWarning{}, false
}

// parseComponentRef parses a CI/CD component reference of the form:
//
//	<host>/<project-path>/<component-name>@<version>
//
// The host is the GitLab instance FQDN. The last path segment before @ is the
// component name; everything between host and component name is the project path.
//
// Examples:
//
//	gitlab.com/components/golang/build@v1.0.0
//	gitlab.example.com/my-org/ci-templates/lint@main
//	gitlab.com/components/secret-detection/secret-detection@~latest
func parseComponentRef(ref string) (host, project, component, version string, err error) {
	atIdx := strings.LastIndex(ref, "@")
	if atIdx < 0 || atIdx == len(ref)-1 {
		err = fmt.Errorf("component reference %q must include a version (e.g. @v1.0 or @main)", ref)
		return
	}
	version = ref[atIdx+1:]
	path := ref[:atIdx]

	parts := strings.Split(path, "/")
	// Minimum: host + at least one project segment + component = 3 parts.
	if len(parts) < 3 {
		err = fmt.Errorf("component reference %q must be <host>/<project>/<component>@<version>", ref)
		return
	}

	host = parts[0]
	component = parts[len(parts)-1]
	project = strings.Join(parts[1:len(parts)-1], "/")
	return
}

// fetchComponentFile fetches a component's template YAML from a GitLab project.
// GitLab supports two layouts; both are attempted:
//   - templates/<component>.yml  (single-file component)
//   - templates/<component>/template.yml  (directory component)
func fetchComponentFile(cfg fetcher.GitLabConfig, project, component, version string) ([]byte, error) {
	primary := "templates/" + component + ".yml"
	data, err := cfg.FetchFile(project, primary, version)
	if err == nil {
		return data, nil
	}

	fallback := "templates/" + component + "/template.yml"
	data2, err2 := cfg.FetchFile(project, fallback, version)
	if err2 != nil {
		// Return the primary error — it refers to the canonical path.
		return nil, fmt.Errorf("%v (also tried %s: %v)", err, fallback, err2)
	}
	return data2, nil
}

// normaliseInclude converts a raw include: list element to a string-keyed map.
// An include: value can be a plain string (shorthand local) or a map.
func normaliseInclude(raw any) (map[string]any, bool) {
	switch v := raw.(type) {
	case map[string]any:
		return v, true
	case string:
		return map[string]any{"local": v}, true
	}
	return nil, false
}

// includeFiles returns the list of file paths from an include entry.
// The file: key may be a single string or a list of strings.
func includeFiles(entry map[string]any) []string {
	raw, ok := entry["file"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case string:
		return []string{v}
	case []any:
		var paths []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				paths = append(paths, s)
			}
		}
		return paths
	}
	return nil
}

// mergeIncluded copies jobs and stages from src into dst.
// dst (the main pipeline) always wins when a key already exists.
func mergeIncluded(dst, src *model.Pipeline) {
	stageSet := make(map[string]bool, len(dst.Stages))
	for _, s := range dst.Stages {
		stageSet[s] = true
	}
	for _, s := range src.Stages {
		if !stageSet[s] {
			dst.Stages = append(dst.Stages, s)
			stageSet[s] = true
		}
	}

	for name, job := range src.Jobs {
		if _, exists := dst.Jobs[name]; !exists {
			dst.Jobs[name] = job
			if raw, ok := src.RawJobs[name]; ok {
				dst.RawJobs[name] = raw
			}
		}
	}
}
