package cicontext

import (
	"regexp"
	"strings"
)

// EvalIf evaluates a GitLab CI rules:if: expression string against the provided
// variable resolver.
//
// Supported:
//   - Variable references: $VAR_NAME or ${VAR_NAME}
//   - String literals:     "value" or 'value'
//   - Null keyword:        null
//   - Comparison:          == != =~ !~
//   - Boolean:             && || !
//   - Grouping:            ( )
//   - Regex flags:         /pattern/i (case-insensitive), /pattern/m, /pattern/s
//   - Multi-line:          newlines between tokens are treated as whitespace
//   - Variable regex RHS:  $VAR =~ $PATTERN when $PATTERN holds a /regex/ string
//
// Regex patterns use Go's regexp syntax, which covers the common RE2 subset
// used by GitLab CI. Unsupported or unparseable expressions fall back to true
// (permissive) so the linter never silently drops jobs it cannot evaluate.
func EvalIf(expr string, vars func(string) string) bool {
	p := &exprParser{s: strings.TrimSpace(expr), vars: vars}
	result, ok := p.parseOr()
	if !ok || p.pos < len(p.s) {
		return true // unparseable → permissive
	}
	return result
}

// ── Parser ────────────────────────────────────────────────────────────────────

type exprParser struct {
	s    string
	pos  int
	vars func(string) string
}

func (p *exprParser) peek() byte {
	if p.pos >= len(p.s) {
		return 0
	}
	return p.s[p.pos]
}

func (p *exprParser) startsWith(tok string) bool {
	return strings.HasPrefix(p.s[p.pos:], tok)
}

func (p *exprParser) consume(tok string) bool {
	if p.startsWith(tok) {
		p.pos += len(tok)
		return true
	}
	return false
}

func (p *exprParser) skipWS() {
	for p.pos < len(p.s) {
		b := p.s[p.pos]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			p.pos++
			continue
		}
		break
	}
}

// ── Grammar (recursive descent) ───────────────────────────────────────────────
//
//   or_expr    → and_expr ( '||' and_expr )*
//   and_expr   → not_expr ( '&&' not_expr )*
//   not_expr   → '!' not_expr | primary
//   primary    → '(' or_expr ')' | comparison
//   comparison → value ( op value | regex_op regex_rhs )?
//   value      → '$' '{' ident '}' | '$' ident | '"' … '"' | "'" … "'" | 'null'
//   op         → '==' | '!='
//   regex_op   → '=~' | '!~'
//   regex_rhs  → '/' … '/' flags? | '$' ident (where ident value is '/…/flags')

func (p *exprParser) parseOr() (bool, bool) {
	left, ok := p.parseAnd()
	if !ok {
		return false, false
	}
	for {
		p.skipWS()
		if !p.consume("||") {
			return left, true
		}
		right, ok := p.parseAnd()
		if !ok {
			return false, false
		}
		left = left || right
	}
}

func (p *exprParser) parseAnd() (bool, bool) {
	left, ok := p.parseNot()
	if !ok {
		return false, false
	}
	for {
		p.skipWS()
		if !p.consume("&&") {
			return left, true
		}
		right, ok := p.parseNot()
		if !ok {
			return false, false
		}
		left = left && right
	}
}

func (p *exprParser) parseNot() (bool, bool) {
	p.skipWS()
	// '!' is logical NOT only when not followed by '=' (which would be '!=').
	if p.peek() == '!' && !p.startsWith("!=") {
		p.pos++
		val, ok := p.parseNot() // right-associative: !!x == x
		return !val, ok
	}
	return p.parsePrimary()
}

func (p *exprParser) parsePrimary() (bool, bool) {
	p.skipWS()
	if p.peek() == '(' {
		p.pos++ // consume '('
		val, ok := p.parseOr()
		if !ok {
			return false, false
		}
		p.skipWS()
		if p.peek() != ')' {
			return false, false // unmatched parenthesis
		}
		p.pos++ // consume ')'
		return val, true
	}
	return p.parseComparison()
}

func (p *exprParser) parseComparison() (bool, bool) {
	p.skipWS()
	leftStr, ok := p.parseValue()
	if !ok {
		return false, false
	}
	p.skipWS()

	switch {
	case p.consume("=="):
		p.skipWS()
		rightStr, ok := p.parseValue()
		if !ok {
			return false, false
		}
		return leftStr == rightStr, true

	case p.consume("!="):
		p.skipWS()
		rightStr, ok := p.parseValue()
		if !ok {
			return false, false
		}
		return leftStr != rightStr, true

	case p.consume("=~"):
		p.skipWS()
		pat, patOk, permissive := p.parseRegexRHS()
		if permissive {
			return true, true
		}
		if !patOk {
			return false, false
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return true, true // bad pattern → permissive
		}
		return re.MatchString(leftStr), true

	case p.consume("!~"):
		p.skipWS()
		pat, patOk, permissive := p.parseRegexRHS()
		if permissive {
			return true, true
		}
		if !patOk {
			return false, false
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return true, true // bad pattern → permissive
		}
		return !re.MatchString(leftStr), true
	}

	// No operator: variable is truthy when non-empty (defined and non-null).
	return leftStr != "", true
}

// parseRegexRHS parses the right-hand side of =~ / !~ operators.
// Returns (pattern, ok, permissive):
//   - /regex/flags literal  → (pattern, true, false)
//   - $VAR whose value is /regex/flags → (pattern, true, false)
//   - $VAR whose value is empty or not a /regex/ → ("", false, true) — caller uses permissive true
//   - parse error → ("", false, false)
func (p *exprParser) parseRegexRHS() (pat string, ok bool, permissive bool) {
	if p.peek() == '/' {
		pat, ok = p.parseRegexLiteral()
		return pat, ok, false
	}
	if p.peek() == '$' {
		varVal, varOk := p.parseValue()
		if !varOk {
			return "", false, false
		}
		pat, ok = extractRegexFromString(varVal)
		if !ok {
			return "", false, true // variable is not a /regex/ value → permissive
		}
		return pat, true, false
	}
	return "", false, false
}

// parseValue reads $VAR, ${VAR}, "string", 'string', null, true, false, or an
// integer literal. null and undefined variables both produce an empty string.
// true/false and integers produce their string representations (GitLab CI
// compares all values as strings).
func (p *exprParser) parseValue() (string, bool) {
	p.skipWS()

	if p.peek() == '$' {
		p.pos++ // consume '$'
		if p.peek() == '{' {
			p.pos++ // consume '{'
			name := p.parseIdent()
			if name == "" || p.peek() != '}' {
				return "", false
			}
			p.pos++ // consume '}'
			return p.vars(name), true
		}
		name := p.parseIdent()
		if name == "" {
			return "", false
		}
		return p.vars(name), true
	}

	// Keywords and string literals must not be prefixes of longer identifiers.
	for _, kw := range []struct{ tok, val string }{
		{"null", ""},
		{"true", "true"},
		{"false", "false"},
	} {
		if p.startsWith(kw.tok) {
			end := p.pos + len(kw.tok)
			if end >= len(p.s) || !isIdentByte(p.s[end]) {
				p.pos += len(kw.tok)
				return kw.val, true
			}
		}
	}

	if p.peek() == '"' || p.peek() == '\'' {
		return p.parseStringLiteral()
	}

	// Integer literal — returned as its decimal string for string comparison.
	if p.peek() >= '0' && p.peek() <= '9' {
		start := p.pos
		for p.pos < len(p.s) && p.s[p.pos] >= '0' && p.s[p.pos] <= '9' {
			p.pos++
		}
		return p.s[start:p.pos], true
	}

	return "", false
}

func (p *exprParser) parseIdent() string {
	start := p.pos
	for p.pos < len(p.s) && isIdentByte(p.s[p.pos]) {
		p.pos++
	}
	return p.s[start:p.pos]
}

func (p *exprParser) parseStringLiteral() (string, bool) {
	quote := p.s[p.pos]
	p.pos++ // consume opening quote
	var sb strings.Builder
	for p.pos < len(p.s) {
		b := p.s[p.pos]
		if b == quote {
			p.pos++ // consume closing quote
			return sb.String(), true
		}
		if b == '\\' && p.pos+1 < len(p.s) {
			p.pos++
			sb.WriteByte(p.s[p.pos])
		} else {
			sb.WriteByte(b)
		}
		p.pos++
	}
	return "", false // unterminated string
}

func (p *exprParser) parseRegexLiteral() (string, bool) {
	if p.peek() != '/' {
		return "", false
	}
	p.pos++ // consume opening '/'
	var sb strings.Builder
	for p.pos < len(p.s) {
		b := p.s[p.pos]
		if b == '/' {
			p.pos++ // consume closing '/'
			flags := p.parseRegexFlags()
			return applyRegexFlags(flags, sb.String()), true
		}
		if b == '\\' && p.pos+1 < len(p.s) {
			p.pos++
			sb.WriteByte('\\')
			sb.WriteByte(p.s[p.pos])
		} else {
			sb.WriteByte(b)
		}
		p.pos++
	}
	return "", false // unterminated regex
}

// parseRegexFlags reads zero or more regex flag letters (i, m, s) after the
// closing '/'. Unknown letters are consumed but ignored.
func (p *exprParser) parseRegexFlags() string {
	start := p.pos
	for p.pos < len(p.s) && isIdentByte(p.s[p.pos]) {
		p.pos++
	}
	return p.s[start:p.pos]
}

// applyRegexFlags prepends Go regexp flag groups to pattern (e.g. (?i) for 'i').
// Unknown flags are silently ignored.
func applyRegexFlags(flags, pattern string) string {
	if flags == "" {
		return pattern
	}
	var prefix strings.Builder
	for _, f := range flags {
		switch f {
		case 'i':
			prefix.WriteString("(?i)")
		case 'm':
			prefix.WriteString("(?m)")
		case 's':
			prefix.WriteString("(?s)")
		}
	}
	return prefix.String() + pattern
}

// extractRegexFromString parses a /pattern/flags string (typically from a CI
// variable) and returns a Go regexp pattern with flags applied.
func extractRegexFromString(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || s[0] != '/' {
		return "", false
	}
	var sb strings.Builder
	i := 1
	for i < len(s) {
		b := s[i]
		if b == '/' {
			i++ // past closing '/'
			var flags strings.Builder
			for i < len(s) && isIdentByte(s[i]) {
				flags.WriteByte(s[i])
				i++
			}
			return applyRegexFlags(flags.String(), sb.String()), true
		}
		if b == '\\' && i+1 < len(s) {
			i++
			sb.WriteByte('\\')
			sb.WriteByte(s[i])
		} else {
			sb.WriteByte(b)
		}
		i++
	}
	return "", false // unterminated
}

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
