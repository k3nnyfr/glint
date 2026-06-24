package linter

// RuleEntry holds the human-readable documentation for a single lint rule,
// shown by 'glint explain <RULE>'.
type RuleEntry struct {
	Title       string   // short title (one line)
	Severity    Severity // Error or Warning
	Description string   // what triggers the rule and why it matters
	Example     string   // YAML snippet that triggers the rule
	Fix         string   // corrected YAML
}

// RuleCatalog maps stable rule IDs to their documentation entries.
var RuleCatalog = map[string]RuleEntry{
	RuleNoStages: {
		Title:    "no stages defined",
		Severity: Warning,
		Description: "The pipeline has no 'stages:' block. GitLab falls back to the " +
			"built-in default stages (build, test, deploy), which may not match your " +
			"intended job ordering.",
		Example: `# No stages: block
build-job:
  script: make build

test-job:
  script: make test`,
		Fix: `stages:
  - build
  - test

build-job:
  stage: build
  script: make build

test-job:
  stage: test
  script: make test`,
	},

	RuleWorkflowWhen: {
		Title:    "workflow.rules.when invalid value",
		Severity: Error,
		Description: "'workflow.rules[n].when' only accepts 'always' or 'never'. Any " +
			"other value (such as 'on_success' or 'manual') is invalid and will cause " +
			"the pipeline to fail to create.",
		Example: `workflow:
  rules:
    - if: $CI_COMMIT_BRANCH
      when: on_success   # invalid here`,
		Fix: `workflow:
  rules:
    - if: $CI_COMMIT_BRANCH
      when: always`,
	},

	RuleMissingScript: {
		Title:    "missing required 'script' field",
		Severity: Error,
		Description: "Every non-trigger, non-pages job must define 'script:' (or 'run:' " +
			"for CI Steps). A job with no script will fail immediately when GitLab tries " +
			"to run it.",
		Example: `my-job:
  stage: test
  image: python:3.12
  # Missing script:`,
		Fix: `my-job:
  stage: test
  image: python:3.12
  script: pytest`,
	},

	RuleUnknownStage: {
		Title:    "job references undeclared stage",
		Severity: Error,
		Description: "The job's 'stage:' value does not match any name in the pipeline's " +
			"'stages:' list. GitLab will reject the pipeline configuration.",
		Example: `stages:
  - build
  - test

deploy-job:
  stage: production   # not in stages:
  script: ./deploy.sh`,
		Fix: `stages:
  - build
  - test
  - production

deploy-job:
  stage: production
  script: ./deploy.sh`,
	},

	RuleOnlyRulesConflict: {
		Title:    "'only:' and 'rules:' used together",
		Severity: Error,
		Description: "'only:' and 'rules:' are mutually exclusive. Using both on the " +
			"same job is a configuration error that prevents the pipeline from being " +
			"created.",
		Example: `my-job:
  only:
    - main
  rules:
    - if: $CI_COMMIT_BRANCH
  script: echo conflict`,
		Fix: `# Remove only: and use rules: exclusively
my-job:
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
  script: echo ok`,
	},

	RuleExceptRulesConflict: {
		Title:    "'except:' and 'rules:' used together",
		Severity: Error,
		Description: "'except:' and 'rules:' are mutually exclusive. Using both on the " +
			"same job is a configuration error that prevents the pipeline from being " +
			"created.",
		Example: `my-job:
  except:
    - tags
  rules:
    - if: $CI_COMMIT_BRANCH
  script: echo conflict`,
		Fix: `my-job:
  rules:
    - if: $CI_COMMIT_TAG
      when: never
    - if: $CI_COMMIT_BRANCH
  script: echo ok`,
	},

	RuleDeprecatedOnly: {
		Title:    "'only:' / 'except:' deprecated",
		Severity: Warning,
		Description: "'only:' and 'except:' are deprecated in favour of 'rules:'. " +
			"GitLab may remove support for these keywords in a future major version. " +
			"'rules:' is more expressive and supports variable references, merge-request " +
			"conditions, and fine-grained 'when:' control.",
		Example: `my-job:
  only:
    - main
  script: ./deploy.sh`,
		Fix: `my-job:
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
  script: ./deploy.sh`,
	},

	RuleInvalidWhen: {
		Title:    "'when:' has invalid value",
		Severity: Error,
		Description: "Job-level 'when:' accepts: on_success, on_failure, always, " +
			"manual, delayed, never. Any other value is rejected by GitLab.",
		Example: `my-job:
  when: sometimes   # invalid
  script: echo hi`,
		Fix: `my-job:
  when: on_success
  script: echo hi`,
	},

	RuleDelayedNoStartIn: {
		Title:    "'when: delayed' without 'start_in:'",
		Severity: Error,
		Description: "'when: delayed' requires a 'start_in:' duration (e.g. " +
			"'30 minutes', '1 hour'). Without it GitLab rejects the job configuration.",
		Example: `my-job:
  when: delayed
  script: echo hi`,
		Fix: `my-job:
  when: delayed
  start_in: 30 minutes
  script: echo hi`,
	},

	RuleStartInNoDelayed: {
		Title:    "'start_in:' without 'when: delayed'",
		Severity: Error,
		Description: "'start_in:' is only meaningful when 'when: delayed'. Setting it " +
			"alongside any other 'when:' value is a configuration error.",
		Example: `my-job:
  when: on_success
  start_in: 30 minutes   # only valid with delayed
  script: echo hi`,
		Fix: `my-job:
  when: delayed
  start_in: 30 minutes
  script: echo hi`,
	},

	RuleInvalidParallel: {
		Title:    "'parallel:' value invalid",
		Severity: Error,
		Description: "'parallel:' must be an integer between 2 and 200 (inclusive), " +
			"or a map with a 'matrix:' key for matrix jobs. Values outside this range " +
			"or the wrong type are rejected by GitLab.",
		Example: `my-job:
  parallel: 1   # must be 2–200
  script: echo hi`,
		Fix: `my-job:
  parallel: 5
  script: echo hi`,
	},

	RuleInvalidRetry: {
		Title:    "'retry:' value invalid",
		Severity: Error,
		Description: "'retry:' accepts an integer 0–2 or a map with optional 'max:' " +
			"(0–2) and 'when:' keys. Values outside this range are rejected.",
		Example: `my-job:
  retry: 5   # max is 2
  script: echo hi`,
		Fix: `my-job:
  retry: 2
  script: echo hi`,
	},

	RuleInvalidRetryWhen: {
		Title:    "'retry.when:' has invalid failure type",
		Severity: Error,
		Description: "'retry.when:' must be a recognised GitLab CI failure type " +
			"(e.g. script_failure, runner_system_failure) or 'always'. Unknown " +
			"values are rejected.",
		Example: `my-job:
  retry:
    max: 2
    when: disk_full   # not a valid failure type
  script: echo hi`,
		Fix: `my-job:
  retry:
    max: 2
    when: runner_system_failure
  script: echo hi`,
	},

	RuleInvalidAllowFailure: {
		Title:    "'allow_failure:' invalid value",
		Severity: Error,
		Description: "'allow_failure:' must be a boolean (true/false) or a map with " +
			"an 'exit_codes:' key. Other forms are rejected by GitLab.",
		Example: `my-job:
  allow_failure: maybe   # must be true/false or map
  script: echo hi`,
		Fix: `my-job:
  allow_failure: true
  script: echo hi`,
	},

	RuleInvalidInterruptible: {
		Title:    "'interruptible:' is not a boolean",
		Severity: Error,
		Description: "'interruptible:' must be a boolean (true or false). Other types " +
			"are rejected.",
		Example: `my-job:
  interruptible: yes   # must be a YAML boolean true/false
  script: echo hi`,
		Fix: `my-job:
  interruptible: true
  script: echo hi`,
	},

	RuleTriggerWithScript: {
		Title:    "trigger job also defines 'script:'",
		Severity: Error,
		Description: "Jobs with a 'trigger:' key (downstream pipeline triggers) cannot " +
			"use 'script:'. These are mutually exclusive — a trigger job has no runner " +
			"environment to execute scripts in.",
		Example: `trigger-job:
  trigger:
    project: mygroup/myproject
  script: echo this will fail`,
		Fix: `trigger-job:
  trigger:
    project: mygroup/myproject`,
	},

	RuleInvalidTrigger: {
		Title:    "trigger: map missing 'project:' or 'include:'",
		Severity: Error,
		Description: "When 'trigger:' is a map it must specify either 'project:' " +
			"(downstream project trigger) or 'include:' (dynamic child pipeline). " +
			"Without one of these keys GitLab cannot determine what to trigger.",
		Example: `trigger-job:
  trigger:
    strategy: depend   # missing project: or include:`,
		Fix: `trigger-job:
  trigger:
    project: mygroup/downstream
    strategy: depend`,
	},

	RuleInvalidCoverage: {
		Title:    "'coverage:' is not a regex pattern",
		Severity: Error,
		Description: "'coverage:' must be a regex pattern wrapped in forward slashes " +
			"(e.g. '/Coverage: \\d+\\.?\\d*%/'). GitLab uses this pattern to extract " +
			"the coverage percentage from job output.",
		Example: `my-job:
  coverage: "\\d+%"   # missing surrounding /…/
  script: pytest --cov`,
		Fix: `my-job:
  coverage: '/Coverage: \d+\.?\d*%/'
  script: pytest --cov`,
	},

	RuleInvalidRelease: {
		Title:    "'release:' missing required 'tag_name:'",
		Severity: Error,
		Description: "'release:' must be a map and must include a 'tag_name:' key. " +
			"Without it GitLab cannot create the release.",
		Example: `release-job:
  script: echo releasing
  release:
    name: My Release   # missing tag_name:`,
		Fix: `release-job:
  script: echo releasing
  release:
    tag_name: $CI_COMMIT_TAG
    name: My Release`,
	},

	RuleInvalidEnvironment: {
		Title:    "'environment:' invalid url or action",
		Severity: Error,
		Description: "'environment.url' requires 'environment.name' to be set. " +
			"'environment.action' must be one of: start, stop, prepare, verify, access.",
		Example: `my-job:
  script: ./deploy.sh
  environment:
    url: https://prod.example.com   # name: is required`,
		Fix: `my-job:
  script: ./deploy.sh
  environment:
    name: production
    url: https://prod.example.com`,
	},

	RuleInvalidArtifacts: {
		Title:    "'artifacts:' invalid configuration",
		Severity: Error,
		Description: "'artifacts.when' must be on_success, on_failure, or always. " +
			"'artifacts.expose_as' requires 'artifacts.paths' to be set.",
		Example: `my-job:
  script: make
  artifacts:
    when: sometimes   # invalid value`,
		Fix: `my-job:
  script: make
  artifacts:
    when: on_failure
    paths:
      - build/logs/`,
	},

	RulePagesPublic: {
		Title:    "pages job missing 'public' in artifacts.paths",
		Severity: Warning,
		Description: "The 'pages' job must include 'public' (or a path starting with " +
			"'public/') in 'artifacts.paths' for GitLab Pages to deploy the site.",
		Example: `pages:
  script: hugo
  artifacts:
    paths:
      - dist/   # should include public/`,
		Fix: `pages:
  script: hugo --destination public
  artifacts:
    paths:
      - public/`,
	},

	RuleInvalidCache: {
		Title:    "'cache:' invalid when or policy",
		Severity: Error,
		Description: "'cache.when' must be on_success, on_failure, or always. " +
			"'cache.policy' must be pull, push, or pull-push.",
		Example: `my-job:
  script: npm ci
  cache:
    paths: [node_modules/]
    policy: read-only   # invalid; use pull`,
		Fix: `my-job:
  script: npm ci
  cache:
    paths: [node_modules/]
    policy: pull`,
	},

	RuleInvalidRulesWhen: {
		Title:    "'rules[n].when:' has invalid value",
		Severity: Error,
		Description: "Inside a 'rules:' block, 'when:' must be one of: on_success, " +
			"on_failure, always, manual, delayed, never. Other values are rejected.",
		Example: `my-job:
  rules:
    - if: $CI_COMMIT_BRANCH
      when: conditional   # not valid here
  script: echo hi`,
		Fix: `my-job:
  rules:
    - if: $CI_COMMIT_BRANCH
      when: on_success
  script: echo hi`,
	},

	RuleInvalidImage: {
		Title:    "'image:' map form missing 'name:'",
		Severity: Error,
		Description: "When 'image:' is a map, it must include a 'name:' key specifying " +
			"the Docker image. Without it GitLab cannot resolve which image to pull.",
		Example: `my-job:
  image:
    entrypoint: ["/bin/sh"]   # missing name:
  script: echo hi`,
		Fix: `my-job:
  image:
    name: alpine:3.19
    entrypoint: ["/bin/sh"]
  script: echo hi`,
	},

	RuleInvalidInherit: {
		Title:    "'inherit.default' or 'inherit.variables' invalid type",
		Severity: Error,
		Description: "'inherit.default' and 'inherit.variables' must each be a boolean " +
			"(true/false) or a list of field/variable names. Other types are rejected.",
		Example: `my-job:
  inherit:
    default: "no"   # must be true/false or a list
  script: echo hi`,
		Fix: `my-job:
  inherit:
    default: false
  script: echo hi`,
	},

	RuleNeedsUnknown: {
		Title:    "'needs:' references unknown job",
		Severity: Error,
		Description: "A job in the 'needs:' list does not exist in the pipeline. " +
			"GitLab will refuse to create the pipeline.",
		Example: `build:
  stage: build
  script: make

test:
  stage: test
  needs: [build, lint]   # lint does not exist
  script: make test`,
		Fix: `lint:
  stage: build
  script: make lint

build:
  stage: build
  script: make

test:
  stage: test
  needs: [build, lint]
  script: make test`,
	},

	RuleNeedsStageOrder: {
		Title:    "'needs:' job is in a later stage",
		Severity: Error,
		Description: "A job listed in 'needs:' is in a later stage than the current " +
			"job. GitLab does not allow needs: to reference future stages.",
		Example: `stages: [build, test, deploy]

test:
  stage: test
  needs: [deploy-job]   # deploy-job is in a later stage
  script: make test

deploy-job:
  stage: deploy
  script: ./deploy.sh`,
		Fix: `stages: [build, test, deploy]

test:
  stage: test
  needs: [build-job]
  script: make test

build-job:
  stage: build
  script: make`,
	},

	RuleNeedsCycle: {
		Title:    "circular dependency in 'needs:' graph",
		Severity: Error,
		Description: "Two or more jobs in the 'needs:' graph form a cycle — A needs B " +
			"and B needs A (directly or transitively). GitLab will reject the pipeline.",
		Example: `job-a:
  stage: test
  needs: [job-b]
  script: echo a

job-b:
  stage: test
  needs: [job-a]
  script: echo b`,
		Fix: `job-a:
  stage: test
  script: echo a

job-b:
  stage: test
  needs: [job-a]
  script: echo b`,
	},

	RuleUnknownDependency: {
		Title:    "'dependencies:' references unknown job",
		Severity: Error,
		Description: "A job listed in 'dependencies:' does not exist in the pipeline. " +
			"GitLab will refuse to create the pipeline.",
		Example: `test:
  stage: test
  dependencies: [build, missing-job]
  script: make test`,
		Fix: `build:
  stage: build
  script: make
  artifacts:
    paths: [dist/]

test:
  stage: test
  dependencies: [build]
  script: make test`,
	},

	RuleDependencyStage: {
		Title:    "'dependencies:' job in same or later stage",
		Severity: Error,
		Description: "'dependencies:' can only reference jobs in earlier stages. " +
			"Referencing a job in the same or a later stage is not allowed because " +
			"artifacts would not yet be available.",
		Example: `stages: [test, deploy]

test:
  stage: test
  dependencies: [deploy-job]   # deploy is a later stage
  script: make test

deploy-job:
  stage: deploy
  script: ./deploy.sh`,
		Fix: `stages: [build, test, deploy]

build:
  stage: build
  script: make
  artifacts:
    paths: [dist/]

test:
  stage: test
  dependencies: [build]
  script: make test`,
	},

	RuleUndeclaredVariable: {
		Title:    "'rules:if:' references undeclared variable",
		Severity: Warning,
		Description: "A 'rules:if:' expression references a variable that is not " +
			"declared in any 'variables:' block in the pipeline YAML. This may be a " +
			"typo, or the variable may be set in GitLab project settings (invisible to " +
			"glint). Predefined CI_* and GITLAB_* variables are always exempt.",
		Example: `variables:
  APP_ENV: staging

my-job:
  rules:
    - if: $DEPLOY_ENV == "prod"   # DEPLOY_ENV not declared
  script: ./deploy.sh`,
		Fix: `variables:
  APP_ENV: staging
  DEPLOY_ENV: staging   # declare it, or set it in GitLab project settings

my-job:
  rules:
    - if: $DEPLOY_ENV == "prod"
  script: ./deploy.sh`,
	},

	RuleDeadRules: {
		Title:    "rules: block has all 'when: never' — job permanently excluded",
		Severity: Warning,
		Description: "Every rule in the job's 'rules:' block has an explicit " +
			"'when: never'. No matter which conditions match, the job will never " +
			"be included in any pipeline run.",
		Example: `my-job:
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
      when: never
    - when: never
  script: echo unreachable`,
		Fix: `# Option 1: remove the job entirely
# Option 2: give at least one rule a non-never when:
my-job:
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
      when: on_success
    - when: never
  script: echo deploy`,
	},

	RuleInvalidService: {
		Title:    "'services:' map form missing 'name:' or invalid 'alias:'",
		Severity: Error,
		Description: "When a service entry is a map it must include a 'name:' key. " +
			"'alias:' (if set) must be a valid DNS label: alphanumeric characters, " +
			"hyphens, and dots only; must start and end with alphanumeric.",
		Example: `my-job:
  script: pytest
  services:
    - alias: my_db   # underscore is not valid in a DNS label`,
		Fix: `my-job:
  script: pytest
  services:
    - name: postgres:15
      alias: my-db`,
	},

	RuleAbsoluteGlobPath: {
		Title:    "'rules:changes' or 'rules:exists' uses absolute path",
		Severity: Warning,
		Description: "Paths in 'rules:changes' and 'rules:exists' are always relative " +
			"to the repository root. An absolute path starting with '/' will never " +
			"match any file and will silently disable the rule.",
		Example: `my-job:
  rules:
    - changes:
        - /src/**/*.go   # absolute — will never match
  script: make`,
		Fix: `my-job:
  rules:
    - changes:
        - src/**/*.go
  script: make`,
	},

	RuleInvalidTimeout: {
		Title:    "'timeout:' is not a valid duration string",
		Severity: Error,
		Description: "'timeout:' must be a GitLab CI duration string composed of " +
			"recognised time units: weeks (w), days (d), hours (h), minutes (m/min), " +
			"seconds (s). Examples: '1h 30m', '90 minutes', '2 hours'.",
		Example: `my-job:
  timeout: "1:30:00"   # colon-separated format not supported
  script: long-running-task.sh`,
		Fix: `my-job:
  timeout: 1h 30m
  script: long-running-task.sh`,
	},

	RuleInvalidIDToken: {
		Title:    "'id_tokens:' entry missing required 'aud:' key",
		Severity: Error,
		Description: "Each entry under 'id_tokens:' must be a map with an 'aud:' " +
			"(audience) key. Without 'aud:' GitLab cannot generate the ID token " +
			"and the job will fail.",
		Example: `my-job:
  id_tokens:
    MY_TOKEN:
      # missing aud:
  script: vault-login.sh`,
		Fix: `my-job:
  id_tokens:
    MY_TOKEN:
      aud: https://vault.example.com
  script: vault-login.sh`,
	},

	RuleInvalidSecret: {
		Title:    "'secrets:' entry missing a provider key",
		Severity: Error,
		Description: "Each entry under 'secrets:' must specify a provider: one of " +
			"'vault:', 'gcp_secret_manager:', or 'azure_key_vault:'. Without a " +
			"provider GitLab cannot retrieve the secret.",
		Example: `my-job:
  secrets:
    DB_PASSWORD:
      path: secret/db/password   # no provider key
  script: run.sh`,
		Fix: `my-job:
  secrets:
    DB_PASSWORD:
      vault:
        engine:
          name: kv-v2
          path: secret
        path: db/password
        field: password
  script: run.sh`,
	},

	RulePagesPublish: {
		Title:    "'pages:' keyword publish directory missing from 'artifacts.paths'",
		Severity: Warning,
		Description: "A job using the 'pages:' keyword must include its publish " +
			"directory in 'artifacts.paths'. Without this, GitLab Pages will not " +
			"deploy the site.",
		Example: `my-pages-job:
  script: hugo
  pages:
    publish: public
  artifacts:
    paths:
      - dist/   # missing public/`,
		Fix: `my-pages-job:
  script: hugo
  pages:
    publish: public
  artifacts:
    paths:
      - public/`,
	},

	RuleDuplicateStage: {
		Title:    "duplicate stage name in 'stages:'",
		Severity: Warning,
		Description: "A stage name appears more than once in the 'stages:' list. " +
			"GitLab silently merges duplicate stage entries, which can make the " +
			"pipeline order confusing.",
		Example: `stages:
  - build
  - test
  - test   # duplicate`,
		Fix: `stages:
  - build
  - test`,
	},

	RuleInvalidCacheKeyFiles: {
		Title:    "'cache.key.files' contains a glob pattern",
		Severity: Warning,
		Description: "'cache.key.files' must be a list of exact file paths. Glob " +
			"patterns (*, ?, [...]) are not expanded — the literal glob string is " +
			"used as the cache key, which probably doesn't match your intent.",
		Example: `my-job:
  script: npm ci
  cache:
    key:
      files:
        - package*.json   # glob — use exact path`,
		Fix: `my-job:
  script: npm ci
  cache:
    key:
      files:
        - package.json
        - package-lock.json`,
	},

	RuleStaticDeadRules: {
		Title:    "rules:if: block statically never activates",
		Severity: Warning,
		Description: "Every 'rules:if:' condition in this job evaluates to false " +
			"using the variable values declared in the pipeline YAML. The job will " +
			"never be included in any pipeline run. This check only fires when ALL " +
			"referenced variables are declared in the YAML (predefined CI_* variables " +
			"are not evaluated to avoid false positives).",
		Example: `variables:
  ENABLE_DEPLOY: "false"

deploy:
  rules:
    - if: '$ENABLE_DEPLOY == "true"'   # always false: ENABLE_DEPLOY is "false"
  script: ./deploy.sh`,
		Fix: `variables:
  ENABLE_DEPLOY: "true"   # fix the variable value

deploy:
  rules:
    - if: '$ENABLE_DEPLOY == "true"'
  script: ./deploy.sh

# Or add an unconditional fallback:
deploy:
  rules:
    - if: '$ENABLE_DEPLOY == "true"'
    - when: manual   # allow manual trigger as a fallback
  script: ./deploy.sh`,
	},

	RuleInheritNoDefault: {
		Title:    "'inherit: default:' declared but no 'default:' block",
		Severity: Warning,
		Description: "A job declares 'inherit: default:' but the pipeline has no " +
			"'default:' block, so the declaration has no effect. This can also fire " +
			"when 'inherit: default: [list]' names fields that are not set in the " +
			"'default:' block.",
		Example: `# No default: block exists

my-job:
  inherit:
    default: false   # no-op: nothing to opt out of
  script: echo hi`,
		Fix: `# Either add a default: block...
default:
  before_script:
    - source venv/bin/activate

my-job:
  inherit:
    default: false
  script: echo hi

# ...or remove the pointless inherit: declaration
my-job:
  script: echo hi`,
	},

	RuleRulesNeedsUnknown: {
		Title:    "'rules:needs:' references unknown job",
		Severity: Error,
		Description: "A job listed in a rule's 'needs:' override does not exist in " +
			"the pipeline. 'rules:needs:' (GitLab CI 16.4+) overrides the top-level " +
			"'needs:' list when that specific rule matches. GitLab will refuse to " +
			"create the pipeline if any referenced job is missing.",
		Example: `stages: [build, test]

build:
  stage: build
  script: make

test:
  stage: test
  script: make test
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
      needs: [build, lint]   # lint does not exist`,
		Fix: `stages: [build, test]

lint:
  stage: build
  script: make lint

build:
  stage: build
  script: make

test:
  stage: test
  script: make test
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
      needs: [build, lint]`,
	},
}
