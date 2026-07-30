package model

import "gopkg.in/yaml.v3"

// Pipeline represents the top-level structure of a .gitlab-ci.yml file.
// Unknown top-level keys are collected into Jobs.
type Pipeline struct {
	SourceFile string         // path of the root pipeline file; set by Parse
	Stages     []string       `yaml:"stages"`
	Variables  map[string]any `yaml:"variables"` // string or {value,description,options} map
	Default    *DefaultConfig `yaml:"default"`
	Include    []any          `yaml:"include"`
	Workflow   *Workflow      `yaml:"workflow"`
	// Jobs holds every non-reserved top-level key (i.e. job definitions).
	Jobs    map[string]Job            `yaml:"-"`
	RawJobs map[string]map[string]any `yaml:"-"` // pre-resolution raw maps, used by the resolver
	// Suppressions maps job names to lists of suppressed rule IDs parsed from
	// "# glint: ignore RULE" comments in the pipeline YAML. Only populated for
	// the root pipeline file (not for included templates).
	Suppressions map[string][]string `yaml:"-"`
}

// SetJobOrigin sets the File field on all jobs that don't already have one.
// Called after ParseBytes to record which file each job came from.
func (p *Pipeline) SetJobOrigin(file string) {
	for name, j := range p.Jobs {
		if j.File == "" {
			j.File = file
		}
		p.Jobs[name] = j
	}
}

type DefaultConfig struct {
	Image        any      `yaml:"image"`        // string or {name,pull_policy,...} map
	BeforeScript any      `yaml:"before_script"` // []string or string (block scalar)
	AfterScript  any      `yaml:"after_script"`  // []string or string
	Cache        any      `yaml:"cache"`
	Artifacts    any      `yaml:"artifacts"`
	Retry        any      `yaml:"retry"`
	Timeout      string   `yaml:"timeout"`
	Tags         []string `yaml:"tags"`
}

type Workflow struct {
	Name       string `yaml:"name"`
	AutoCancel any    `yaml:"auto_cancel"`
	Rules      Rules  `yaml:"rules"`
}

type Job struct {
	Name   string // set by parser, not from YAML
	File   string // source file; set by Parse / resolver
	Line   int    // line of the job key in its source file; set by parser
	Column int    // column of the job key (1-indexed); set by parser
	Stage        string   `yaml:"stage"`
	Script       any `yaml:"script"`       // []string or string (block scalar)
	Run          any `yaml:"run"`          // alternative to script (CI steps)
	BeforeScript any `yaml:"before_script"` // []string or string
	AfterScript  any `yaml:"after_script"`  // []string or string
	Image        any      `yaml:"image"`
	Services     []any    `yaml:"services"`
	Variables    map[string]any `yaml:"variables"` // string or {value,description,options} map
	Rules        Rules          `yaml:"rules"`
	Only         any      `yaml:"only"`
	Except       any      `yaml:"except"`
	Needs        []any    `yaml:"needs"`
	Dependencies []string `yaml:"dependencies"`
	Artifacts    any      `yaml:"artifacts"`
	Cache        any      `yaml:"cache"`
	Tags         []string `yaml:"tags"`
	Allow        any      `yaml:"allow_failure"`
	When         string   `yaml:"when"`
	StartIn      string   `yaml:"start_in"`
	Timeout      string   `yaml:"timeout"`
	Retry        any      `yaml:"retry"`
	Parallel     any      `yaml:"parallel"`
	Extends      any      `yaml:"extends"`
	Trigger      any      `yaml:"trigger"`
	Inherit      any      `yaml:"inherit"`
	Environment  any      `yaml:"environment"`
	Release      any      `yaml:"release"`
	Coverage     string   `yaml:"coverage"`
	Secrets      any      `yaml:"secrets"`
	IDTokens     any      `yaml:"id_tokens"`
	Pages        any      `yaml:"pages"`
	Interruptible any     `yaml:"interruptible"`
	ResourceGroup string  `yaml:"resource_group"`
}

type Rule struct {
	If           string         `yaml:"if"`
	When         string         `yaml:"when"`
	Changes      any            `yaml:"changes"`       // []string or {paths,compare_to} map
	Exists       any            `yaml:"exists"`        // []string or map form
	Variables    map[string]any `yaml:"variables"`     // set/override variables when rule matches (GitLab CI 15.0+)
	Needs        []any          `yaml:"needs"`         // override needs: when this rule matches (GitLab CI 16.4+)
	AllowFailure any            `yaml:"allow_failure"` // bool or {exit_codes:} map (GitLab CI 15.0+)
	GroupID      int            `yaml:"-"`             // >0 means AND-group; set by Rules.UnmarshalYAML
}

// Rules is a list of Rule entries that also supports GitLab CI's nested-array
// form where each outer entry can itself be a sequence of rules (AND-group).
// Rules with the same non-zero GroupID form an AND-group: all conditions must
// match for the group to fire. GroupID == 0 means standalone rule.
type Rules []Rule

func (r *Rules) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.SequenceNode {
		return nil
	}
	nextGroup := 1
	for _, item := range value.Content {
		switch item.Kind {
		case yaml.MappingNode:
			var rule Rule // GroupID stays 0 (standalone)
			if err := item.Decode(&rule); err != nil {
				return err
			}
			*r = append(*r, rule)
		case yaml.SequenceNode:
			// AND-group: assign a shared GroupID so the evaluator can apply AND logic.
			var group []Rule
			if err := item.Decode(&group); err != nil {
				return err
			}
			for i := range group {
				group[i].GroupID = nextGroup
			}
			*r = append(*r, group...)
			nextGroup++
		}
	}
	return nil
}

// Groups returns the rules partitioned into AND-groups. Standalone rules
// (GroupID == 0) form single-element groups; rules with a shared GroupID form
// one group where all conditions must match.
func (r Rules) Groups() [][]Rule {
	var groups [][]Rule
	seen := map[int]int{} // groupID → index in groups
	for _, rule := range r {
		if rule.GroupID == 0 {
			groups = append(groups, []Rule{rule})
		} else {
			idx, ok := seen[rule.GroupID]
			if !ok {
				idx = len(groups)
				seen[rule.GroupID] = idx
				groups = append(groups, nil)
			}
			groups[idx] = append(groups[idx], rule)
		}
	}
	return groups
}

// ReservedKeys are top-level GitLab CI keys that are NOT job definitions.
var ReservedKeys = map[string]bool{
	"stages":    true,
	"variables": true,
	"default":   true,
	"include":   true,
	"workflow":  true,
	"spec":      true, // CI component spec header
}
