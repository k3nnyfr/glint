package resolver

import (
	"fmt"

	"git.k3nny.fr/gitlab-sim/internal/model"
	"gopkg.in/yaml.v3"
)

// Resolve resolves all extends: references in p.Jobs in place.
// Jobs are merged depth-first so that base definitions are resolved before
// derived ones. Mutates p.Jobs with the fully merged Job structs.
func Resolve(p *model.Pipeline) error {
	// Build extends graph: jobName -> ordered list of base job names.
	extendsGraph := make(map[string][]string, len(p.Jobs))
	for name, job := range p.Jobs {
		bases, err := parseExtends(job.Extends)
		if err != nil {
			return fmt.Errorf("job %q: invalid extends: %w", name, err)
		}
		if len(bases) == 0 {
			continue
		}
		for _, base := range bases {
			if _, ok := p.RawJobs[base]; !ok {
				return fmt.Errorf("job %q extends unknown job %q", name, base)
			}
		}
		extendsGraph[name] = bases
	}

	if len(extendsGraph) == 0 {
		return nil
	}

	// Topological sort — bases must be resolved before derived jobs.
	order, err := topoSort(extendsGraph)
	if err != nil {
		return err
	}

	// resolved holds the final merged raw map for each processed job.
	resolved := make(map[string]map[string]any, len(order))

	for _, name := range order {
		bases := extendsGraph[name]

		// Merge bases left-to-right (later bases override earlier ones),
		// then overlay the job's own definition on top.
		merged := make(map[string]any)
		for _, baseName := range bases {
			baseMap := resolved[baseName]
			if baseMap == nil {
				baseMap = p.RawJobs[baseName]
			}
			merged = deepMerge(merged, baseMap)
		}
		merged = deepMerge(merged, p.RawJobs[name])
		resolved[name] = merged

		// Re-decode the merged map into a Job struct.
		data, err := yaml.Marshal(merged)
		if err != nil {
			return fmt.Errorf("job %q: re-encoding merged definition: %w", name, err)
		}
		var j model.Job
		if err := yaml.Unmarshal(data, &j); err != nil {
			return fmt.Errorf("job %q: re-decoding merged definition: %w", name, err)
		}
		j.Name = name
		p.Jobs[name] = j
	}

	return nil
}

// parseExtends normalises the extends field (string or []any) into []string.
func parseExtends(extends any) ([]string, error) {
	if extends == nil {
		return nil, nil
	}
	switch v := extends.(type) {
	case string:
		return []string{v}, nil
	case []any:
		names := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("expected string item, got %T", item)
			}
			names = append(names, s)
		}
		return names, nil
	default:
		return nil, fmt.Errorf("unexpected type %T", extends)
	}
}

// topoSort returns the keys of graph in dependency order (bases first).
// Returns an error if a cycle is detected.
func topoSort(graph map[string][]string) ([]string, error) {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	state := make(map[string]int, len(graph))
	order := make([]string, 0, len(graph))

	var visit func(name string) error
	visit = func(name string) error {
		switch state[name] {
		case visited:
			return nil
		case visiting:
			return fmt.Errorf("cycle in extends involving job %q", name)
		}
		state[name] = visiting
		for _, dep := range graph[name] {
			if _, inGraph := graph[dep]; inGraph {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		state[name] = visited
		order = append(order, name)
		return nil
	}

	for name := range graph {
		if err := visit(name); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// deepMerge returns a new map that is override merged onto base.
// Maps are merged recursively; all other values are replaced by override.
func deepMerge(base, override map[string]any) map[string]any {
	result := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		if baseVal, exists := result[k]; exists {
			if baseMap, ok := baseVal.(map[string]any); ok {
				if overMap, ok := v.(map[string]any); ok {
					result[k] = deepMerge(baseMap, overMap)
					continue
				}
			}
		}
		result[k] = v
	}
	return result
}
