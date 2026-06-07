package cicontext

import (
	"regexp"
	"strings"
)

// EvalIf evaluates a GitLab CI rules:if: expression string against the provided
// variable resolver.
//
// Supported:
//   - Variable references: $VAR_NAME
//   - String literals:     "value" or 'value'
//   - Null keyword:        null
//   - Comparison:          == != =~ !~
//   - Boolean:             && || !
//   - Grouping:            ( )
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
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

// ── Grammar (recursive descent) ───────────────────────────────────────────────
//
//   or_expr    → and_expr ( '||' and_expr )*
//   and_expr   → not_expr ( '&&' not_expr )*
//   not_expr   → '!' not_expr | primary
//   primary    → '(' or_expr ')' | comparison
//   comparison → value ( op value | regex_op regex )? | value
//   value      → '$' ident | '"' … '"' | "'" … "'" | 'null'
//   op         → '==' | '!='
//   regex_op   → '=~' | '!~'
//   regex      → '/' … '/'

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
		pat, ok := p.parseRegexLiteral()
		if !ok {
			return false, false
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			return true, true // bad pattern → permissive
		}
		return re.MatchString(leftStr), true

	case p.consume("!~"):
		p.skipWS()
		pat, ok := p.parseRegexLiteral()
		if !ok {
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

// parseValue reads $VAR, "string", 'string', or null.
// null and undefined variables both produce an empty string.
func (p *exprParser) parseValue() (string, bool) {
	p.skipWS()

	if p.peek() == '$' {
		p.pos++ // consume '$'
		name := p.parseIdent()
		if name == "" {
			return "", false
		}
		return p.vars(name), true
	}

	// null keyword — must not be a prefix of a longer identifier.
	if p.startsWith("null") {
		end := p.pos + 4
		if end >= len(p.s) || !isIdentByte(p.s[end]) {
			p.pos += 4
			return "", true // null → empty string
		}
	}

	if p.peek() == '"' || p.peek() == '\'' {
		return p.parseStringLiteral()
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
			return sb.String(), true
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

func isIdentByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
