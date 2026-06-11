package model

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
	Rules []Rule `yaml:"rules"`
}

type Job struct {
	Name string // set by parser, not from YAML
	File string // source file; set by Parse / resolver
	Line int    // line of the job key in its source file; set by parser
	Stage        string   `yaml:"stage"`
	Script       any `yaml:"script"`       // []string or string (block scalar)
	Run          any `yaml:"run"`          // alternative to script (CI steps)
	BeforeScript any `yaml:"before_script"` // []string or string
	AfterScript  any `yaml:"after_script"`  // []string or string
	Image        any      `yaml:"image"`
	Services     []any    `yaml:"services"`
	Variables    map[string]any `yaml:"variables"` // string or {value,description,options} map
	Rules        []Rule         `yaml:"rules"`
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
	If      string `yaml:"if"`
	When    string `yaml:"when"`
	Changes any    `yaml:"changes"` // []string or {paths,compare_to} map
	Exists  any    `yaml:"exists"`  // []string or map form
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
