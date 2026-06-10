package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.k3nny.fr/glint/internal/fetcher"
	"git.k3nny.fr/glint/internal/model"
)

// IncludeWarning describes an include entry that could not be resolved.
// These are surfaced to the user as [WARNING] lines before the lint findings.
type IncludeWarning struct {
	// Label is a short human-readable identifier shown in the warning message.
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

// ExtendWarning describes an extends: reference whose base job could not be
// found. This typically means the base is in a remote include that was not
// fetched (e.g. no token). The job's extends chain is skipped; the job itself
// is still linted with whatever fields it directly defines.
type ExtendWarning struct {
	Job  string // job that declares the extends
	Base string // the unknown base job name
}

// ResolveIncludes processes the pipeline's include: block.
//
// rootDir is the repository root directory used to resolve local: includes.
// In practice this is the directory of the top-level pipeline file being linted.
//
// Local includes are read from disk and merged recursively.
// Project includes require authentication and are skipped with a warning when
// no token is configured. Component includes are attempted unauthenticated.
// Remote and template includes are silently skipped (resolved by GitLab at runtime).
//
// The second return value carries extends warnings discovered while recursively
// processing included files (forwarded from resolver.Resolve calls).
func ResolveIncludes(p *model.Pipeline, cfg fetcher.GitLabConfig, rootDir string) ([]IncludeWarning, []ExtendWarning) {
	visited := map[string]bool{}
	return resolveIncludes(p, p.Include, cfg, rootDir, visited)
}

// resolveIncludes is the recursive core of ResolveIncludes.
func resolveIncludes(p *model.Pipeline, includes []any, cfg fetcher.GitLabConfig, rootDir string, visited map[string]bool) ([]IncludeWarning, []ExtendWarning) {
	var warnings []IncludeWarning
	var extWarnings []ExtendWarning

	for _, inc := range includes {
		entry, ok := normaliseInclude(inc)
		if !ok {
			continue
		}

		if project, _ := entry["project"].(string); project != "" {
			w, ew := resolveProjectInclude(p, entry, project, cfg, rootDir, visited)
			warnings = append(warnings, w...)
			extWarnings = append(extWarnings, ew...)
			continue
		}

		if compRef, _ := entry["component"].(string); compRef != "" {
			w, ew, hadErr := resolveComponentInclude(p, compRef, cfg, rootDir, visited)
			if hadErr {
				warnings = append(warnings, w)
			}
			extWarnings = append(extWarnings, ew...)
			continue
		}

		if local, _ := entry["local"].(string); local != "" {
			w, ew := resolveLocalInclude(p, local, cfg, rootDir, visited)
			warnings = append(warnings, w...)
			extWarnings = append(extWarnings, ew...)
			continue
		}

		// remote, template — resolved by GitLab at runtime, skip silently.
	}
	return warnings, extWarnings
}

// resolveLocalInclude reads a local file from disk (paths are always relative
// to the repository root, with or without a leading slash), recursively
// resolves its own includes, and merges it into p.
func resolveLocalInclude(p *model.Pipeline, rawPath string, cfg fetcher.GitLabConfig, rootDir string, visited map[string]bool) ([]IncludeWarning, []ExtendWarning) {
	relPath := strings.TrimPrefix(rawPath, "/")
	absPath := filepath.Join(rootDir, relPath)
	label := "local " + rawPath

	if visited[absPath] {
		return nil, nil
	}
	visited[absPath] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return []IncludeWarning{{Label: label, Err: err}}, nil
	}

	included, err := model.ParseBytes(data)
	if err != nil {
		return []IncludeWarning{{Label: label, Err: fmt.Errorf("parsing YAML: %w", err)}}, nil
	}

	// Recursively resolve the included file's own includes first, merging
	// everything into `included` before we merge it into the parent `p`.
	var warnings []IncludeWarning
	var extWarnings []ExtendWarning
	if len(included.Include) > 0 {
		w, ew := resolveIncludes(included, included.Include, cfg, rootDir, visited)
		warnings = append(warnings, w...)
		extWarnings = append(extWarnings, ew...)
	}

	mergeIncluded(p, included)
	return warnings, extWarnings
}

// resolveProjectInclude fetches all files listed under a single project: entry
// and merges them into p.
func resolveProjectInclude(p *model.Pipeline, entry map[string]any, project string, cfg fetcher.GitLabConfig, rootDir string, visited map[string]bool) ([]IncludeWarning, []ExtendWarning) {
	ref, _ := entry["ref"].(string)
	var warnings []IncludeWarning
	var extWarnings []ExtendWarning

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

		if len(included.Include) > 0 {
			w, ew := resolveIncludes(included, included.Include, cfg, rootDir, visited)
			warnings = append(warnings, w...)
			extWarnings = append(extWarnings, ew...)
		}

		mergeIncluded(p, included)
	}
	return warnings, extWarnings
}

// resolveComponentInclude fetches a CI/CD catalog component and merges it into p.
// Returns (warning, extWarnings, true) if something went wrong; (zero, nil, false) on success.
func resolveComponentInclude(p *model.Pipeline, ref string, cfg fetcher.GitLabConfig, rootDir string, visited map[string]bool) (IncludeWarning, []ExtendWarning, bool) {
	label := "component " + ref

	if strings.ContainsRune(ref, '$') {
		return IncludeWarning{
			Label: label,
			Err:   fmt.Errorf("component reference contains a CI variable that cannot be resolved at lint time"),
		}, nil, true
	}

	host, project, component, version, err := parseComponentRef(ref)
	if err != nil {
		return IncludeWarning{Label: label, Err: err}, nil, true
	}

	hostCfg := cfg.ForHost(host)
	data, err := fetchComponentFile(hostCfg, project, component, version)
	if err != nil {
		return IncludeWarning{Label: label, Err: err}, nil, true
	}

	included, err := model.ParseBytes(data)
	if err != nil {
		return IncludeWarning{Label: label, Err: fmt.Errorf("parsing component YAML: %w", err)}, nil, true
	}

	var extWarnings []ExtendWarning
	if len(included.Include) > 0 {
		_, ew := resolveIncludes(included, included.Include, cfg, rootDir, visited)
		extWarnings = append(extWarnings, ew...)
	}

	mergeIncluded(p, included)
	return IncludeWarning{}, extWarnings, false
}

// parseComponentRef parses a CI/CD component reference of the form:
//
//	<host>/<project-path>/<component-name>@<version>
func parseComponentRef(ref string) (host, project, component, version string, err error) {
	atIdx := strings.LastIndex(ref, "@")
	if atIdx < 0 || atIdx == len(ref)-1 {
		err = fmt.Errorf("component reference %q must include a version (e.g. @v1.0 or @main)", ref)
		return
	}
	version = ref[atIdx+1:]
	path := ref[:atIdx]

	parts := strings.Split(path, "/")
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
func fetchComponentFile(cfg fetcher.GitLabConfig, project, component, version string) ([]byte, error) {
	primary := "templates/" + component + ".yml"
	data, err := cfg.FetchFile(project, primary, version)
	if err == nil {
		return data, nil
	}

	fallback := "templates/" + component + "/template.yml"
	data2, err2 := cfg.FetchFile(project, fallback, version)
	if err2 != nil {
		return nil, fmt.Errorf("%v (also tried %s: %v)", err, fallback, err2)
	}
	return data2, nil
}

// normaliseInclude converts a raw include: list element to a string-keyed map.
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
