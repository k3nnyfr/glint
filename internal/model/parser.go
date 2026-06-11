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
	p, err := ParseBytes(data)
	if err != nil {
		return nil, err
	}
	p.SourceFile = path
	p.SetJobOrigin(path)
	return p, nil
}

// ParseBytes parses YAML from an in-memory byte slice.
func ParseBytes(data []byte) (*Pipeline, error) {
	data = sanitizeYAMLEscapes(data)

	// First pass: parse into a yaml.Node document to extract job keys with
	// their exact source line numbers (key nodes carry the line, value nodes
	// carry the body we decode into Job / map[string]any).
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	// Second pass: decode known top-level fields into Pipeline.
	p := &Pipeline{}
	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parsing pipeline structure: %w", err)
	}

	p.Jobs = make(map[string]Job)
	p.RawJobs = make(map[string]map[string]any)

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return p, nil
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return p, nil
	}

	// Walk root mapping in key/value pairs.
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]
		key := keyNode.Value
		if ReservedKeys[key] {
			continue
		}

		var rawMap map[string]any
		if err := valNode.Decode(&rawMap); err != nil {
			return nil, fmt.Errorf("parsing raw job %q: %w", key, err)
		}
		p.RawJobs[key] = rawMap

		var j Job
		if err := valNode.Decode(&j); err != nil {
			return nil, fmt.Errorf("parsing job %q: %w", key, err)
		}
		j.Name = key
		j.Line = keyNode.Line // exact line of the job name key
		p.Jobs[key] = j
	}

	return p, nil
}

// sanitizeYAMLEscapes rewrites double-quoted YAML strings, replacing the \/
// escape sequence (unrecognised by gopkg.in/yaml.v3) with \\/ so that the
// parser produces a literal backslash+slash — preserving regex patterns like
// /^us\// that appear in GitLab CI if: expressions.
func sanitizeYAMLEscapes(data []byte) []byte {
	type state int
	const (
		stOutside state = iota
		stSingleQ
		stDoubleQ
		stEscape
	)

	out := make([]byte, 0, len(data))
	s := stOutside

	for i := 0; i < len(data); i++ {
		b := data[i]
		switch s {
		case stOutside:
			out = append(out, b)
			switch b {
			case '"':
				s = stDoubleQ
			case '\'':
				s = stSingleQ
			}
		case stSingleQ:
			out = append(out, b)
			if b == '\'' {
				if i+1 < len(data) && data[i+1] == '\'' {
					// '' inside a single-quoted string is an escaped single-quote
					out = append(out, data[i+1])
					i++
				} else {
					s = stOutside
				}
			}
		case stDoubleQ:
			out = append(out, b)
			switch b {
			case '\\':
				s = stEscape
			case '"':
				s = stOutside
			}
		case stEscape:
			if b == '/' {
				// \/ is not recognised by yaml.v3; rewrite as \\/ which
				// the parser resolves to a literal backslash + slash.
				out = append(out, '\\', '/')
			} else {
				out = append(out, b)
			}
			s = stDoubleQ
		}
	}
	return out
}
