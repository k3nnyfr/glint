package model

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse reads a .gitlab-ci.yml file and returns a Pipeline.
func Parse(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// First pass: decode into a raw map to extract job keys.
	var raw map[string]yaml.Node
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	// Second pass: decode known top-level fields into Pipeline.
	p := &Pipeline{}
	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parsing pipeline structure: %w", err)
	}

	p.Jobs = make(map[string]Job)
	p.RawJobs = make(map[string]map[string]any)
	for key, node := range raw {
		if ReservedKeys[key] {
			continue
		}
		var rawMap map[string]any
		if err := node.Decode(&rawMap); err != nil {
			return nil, fmt.Errorf("parsing raw job %q: %w", key, err)
		}
		p.RawJobs[key] = rawMap

		var j Job
		if err := node.Decode(&j); err != nil {
			return nil, fmt.Errorf("parsing job %q: %w", key, err)
		}
		j.Name = key
		p.Jobs[key] = j
	}

	return p, nil
}
