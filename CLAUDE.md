# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

gitlab-sim is a tool aiming to simulate and lint gitlab-ci pipelines locally and produce a graph.

## Commit Message Guidelines

**IMPORTANT: This project uses [Conventional Commits](https://www.conventionalcommits.org/) format.**

All commit messages must follow this format:
```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to build process or auxiliary tools
- `perf`: Performance improvements
- `style`: Code style changes (formatting, missing semicolons, etc.)

**Scopes (commonly used):**
- `auth`: Authentication/authorization changes
- `security`: Security-related changes
- `gui`: Web GUI changes
- `api`: API changes
- `readme`: README.md changes
- `claude`: CLAUDE.md changes
- `core`: Core library changes

**Breaking Changes:**
Add `!` after type/scope for breaking changes:
- `feat(api)!: remove deprecated endpoint`

**Note:** Always include a scope in parentheses, even for documentation changes.

When Claude Code creates commits, it will automatically follow this format.
