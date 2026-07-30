package linter

// Rule ID constants. Each ID is stable across versions and uniquely identifies
// one lint check. Use these when filtering output, writing suppression rules,
// or referencing a check in documentation.
//
// Prefix GL = Glint / GitLab CI lint.
// Numbering is sequential by category; gaps may appear as rules are added.
const (
	// ── Pipeline-level ──────────────────────────────────────────────────────

	// GL001: no stages: block defined; GitLab falls back to default stages.
	RuleNoStages = "GL001"

	// GL002: workflow.rules[n].when has an invalid value (only always/never allowed).
	RuleWorkflowWhen = "GL002"

	// ── Job structure ────────────────────────────────────────────────────────

	// GL003: job is missing a required script: (or run:) field.
	RuleMissingScript = "GL003"

	// GL004: job references a stage not declared in stages:.
	RuleUnknownStage = "GL004"

	// GL005: only: and rules: used together on the same job.
	RuleOnlyRulesConflict = "GL005"

	// GL006: except: and rules: used together on the same job.
	RuleExceptRulesConflict = "GL006"

	// GL007: only:/except: used (deprecated; prefer rules:).
	RuleDeprecatedOnly = "GL007"

	// ── Keyword constraints ──────────────────────────────────────────────────

	// GL008: when: has an invalid value.
	RuleInvalidWhen = "GL008"

	// GL009: when: delayed without start_in:.
	RuleDelayedNoStartIn = "GL009"

	// GL010: start_in: set when when: is not delayed.
	RuleStartInNoDelayed = "GL010"

	// GL011: parallel: value is invalid (integer out of range or map missing matrix:).
	RuleInvalidParallel = "GL011"

	// GL012: retry: integer is out of range 0–2, or retry: is neither int nor map.
	RuleInvalidRetry = "GL012"

	// GL013: retry.when: contains an unrecognised failure type.
	RuleInvalidRetryWhen = "GL013"

	// GL014: allow_failure: is not a boolean or a map with exit_codes:.
	RuleInvalidAllowFailure = "GL014"

	// GL015: interruptible: is not a boolean.
	RuleInvalidInterruptible = "GL015"

	// GL016: trigger: job also defines script: (mutually exclusive).
	RuleTriggerWithScript = "GL016"

	// GL017: trigger: map does not specify project: or include:.
	RuleInvalidTrigger = "GL017"

	// GL018: coverage: is not a regex pattern wrapped in /.
	RuleInvalidCoverage = "GL018"

	// GL019: release: is missing required tag_name:, or is not a map.
	RuleInvalidRelease = "GL019"

	// GL020: environment: has an invalid url/action configuration.
	RuleInvalidEnvironment = "GL020"

	// GL021: artifacts: has an invalid when/expose_as configuration.
	RuleInvalidArtifacts = "GL021"

	// GL022: pages job artifacts.paths does not include public/.
	RulePagesPublic = "GL022"

	// GL023: cache: has an invalid when/policy value.
	RuleInvalidCache = "GL023"

	// GL024: rules[n].when has an invalid value.
	RuleInvalidRulesWhen = "GL024"

	// GL025: image: map form is missing a name: key.
	RuleInvalidImage = "GL025"

	// GL026: inherit.default or inherit.variables is not a boolean or list.
	RuleInvalidInherit = "GL026"

	// ── Cross-job graph ──────────────────────────────────────────────────────

	// GL027: needs: references a job that does not exist in the pipeline.
	RuleNeedsUnknown = "GL027"

	// GL028: needs: references a job in a later stage than the current job.
	RuleNeedsStageOrder = "GL028"

	// GL029: circular dependency detected in the needs: graph.
	RuleNeedsCycle = "GL029"

	// GL030: dependencies: references a job that does not exist.
	RuleUnknownDependency = "GL030"

	// GL031: dependencies: references a job in the same or a later stage.
	RuleDependencyStage = "GL031"

	// ── Expression validation ────────────────────────────────────────────────

	// GL032: rules:if: references a variable not declared in pipeline variables:,
	// the job's own variables:, or any workflow:rules:variables: block.
	// May be a false positive for variables set in GitLab CI/CD project settings.
	RuleUndeclaredVariable = "GL032"

	// GL033: every rule in a job's rules: block has when: never, so the job
	// can never be included in any pipeline run.
	RuleDeadRules = "GL033"

	// GL034: services: map form is missing 'name', or 'alias' is not a valid DNS label.
	RuleInvalidService = "GL034"

	// GL035: rules:changes or rules:exists contains an absolute path (starts with /);
	// GitLab CI paths are relative to the repository root and absolute paths never match.
	RuleAbsoluteGlobPath = "GL035"

	// GL036: timeout: is not a valid GitLab CI duration string (e.g. '1h 30m', '90 minutes').
	RuleInvalidTimeout = "GL036"

	// GL037: id_tokens: entry is missing the required 'aud' key.
	RuleInvalidIDToken = "GL037"

	// GL038: secrets: entry is missing a provider key (vault, gcp_secret_manager, or azure_key_vault).
	RuleInvalidSecret = "GL038"

	// GL039: a job has the pages: keyword but artifacts.paths does not include the publish directory.
	RulePagesPublish = "GL039"

	// GL040: a stage name appears more than once in stages:; GitLab silently merges duplicates.
	RuleDuplicateStage = "GL040"

	// GL041: cache.key.files contains a glob pattern; it must be a list of exact file paths.
	RuleInvalidCacheKeyFiles = "GL041"

	// GL042: every rules:if: condition in a job's rules: block evaluates to false
	// given the values of variables declared in the pipeline YAML, so the job can
	// never be active. Only fires when all referenced variables are declared
	// (predefined CI_* / GITLAB_* vars are not evaluated to avoid false positives).
	RuleStaticDeadRules = "GL042"

	// GL043: a job declares 'inherit: default:' (true, false, or list) but the
	// pipeline has no 'default:' block, making the declaration a no-op. Also fires
	// when 'inherit: default: [list]' names fields not set in the default: block.
	RuleInheritNoDefault = "GL043"

	// GL044: a rules:needs: entry references a job that does not exist in the pipeline.
	// rules:needs: overrides the top-level needs: when a specific rule matches (GitLab CI 16.4+).
	RuleRulesNeedsUnknown = "GL044"

	// GL045: an include: remote: entry uses plain HTTP instead of HTTPS.
	// CI templates fetched over HTTP are transmitted in cleartext and can be
	// intercepted or modified in transit.
	RuleInsecureRemoteInclude = "GL045"

	// GL046: image: or services[n]: pull_policy: contains an unrecognised value.
	// Valid values: always, if-not-present, never (or a list of those values).
	RuleInvalidPullPolicy = "GL046"

	// GL047: a variable declared with options: has a default value: that is not
	// listed in the options list. GitLab rejects the pipeline at creation time.
	RuleVariableValueNotInOptions = "GL047"

	// GL048: trigger.forward: contains an unrecognised key. Only
	// pipeline_variables and yaml_variables are valid.
	RuleInvalidTriggerForward = "GL048"

	// GL049: rules[n].allow_failure: is not a boolean or a map with exit_codes:.
	RuleInvalidRulesAllowFailure = "GL049"
)
