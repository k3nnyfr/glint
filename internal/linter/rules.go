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
)
