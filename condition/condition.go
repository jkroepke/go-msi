// Package condition checks Windows Installer conditional statements against
// the grammar of docs/spec/MSISPEC.md section 8.8. The engine ignores a
// LaunchCondition or action condition that does not parse, so a builder has
// to reject it (RUN-04, section 9.1). The package parses only; it does not
// evaluate.
package condition

import (
	"fmt"
	"strings"
	"unicode"
)

// Error is a syntax error with its position.
type Error struct {
	Pos int
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("condition: %s at offset %d", e.Msg, e.Pos)
}

type tokenKind int

const (
	tokEOF tokenKind = iota
	tokIdent
	tokNumber
	tokString
	tokLParen
	tokRParen
	tokOp      // comparison operator, possibly with a ~ prefix
	tokKeyword // AND OR NOT XOR EQV IMP
	tokPrefix  // one of % $ ? & ! followed by an identifier
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

var keywords = map[string]bool{"AND": true, "OR": true, "NOT": true, "XOR": true, "EQV": true, "IMP": true}

func isIdentStart(r rune) bool { return r == '_' || unicode.IsLetter(r) }

func isIdentPart(r rune) bool {
	return r == '_' || r == '.' || r == '#' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

type lexer struct {
	s   string
	pos int
}

func (l *lexer) next() (token, error) {
	for l.pos < len(l.s) && (l.s[l.pos] == ' ' || l.s[l.pos] == '\t' || l.s[l.pos] == '\r' || l.s[l.pos] == '\n') {
		l.pos++
	}

	if l.pos >= len(l.s) {
		return token{kind: tokEOF, pos: l.pos}, nil
	}

	start := l.pos
	c := l.s[l.pos]

	switch {
	case c == '(':
		l.pos++

		return token{tokLParen, "(", start}, nil
	case c == ')':
		l.pos++

		return token{tokRParen, ")", start}, nil
	case c == '"':
		end := strings.IndexByte(l.s[l.pos+1:], '"')
		if end < 0 {
			return token{}, &Error{start, "unterminated string"}
		}

		l.pos += end + 2

		return token{tokString, l.s[start:l.pos], start}, nil
	case c == '%' || c == '$' || c == '?' || c == '&' || c == '!':
		l.pos++

		if l.pos >= len(l.s) || !isIdentStart(rune(l.s[l.pos])) {
			return token{}, &Error{start, fmt.Sprintf("%q must be followed by a name", c)}
		}

		for l.pos < len(l.s) && isIdentPart(rune(l.s[l.pos])) {
			l.pos++
		}

		return token{tokPrefix, l.s[start:l.pos], start}, nil
	case c == '~' || c == '=' || c == '<' || c == '>':
		p := l.pos
		if c == '~' {
			p++
		}

		if p < len(l.s) {
			switch l.s[p] {
			case '=':
				l.pos = p + 1

				return token{tokOp, l.s[start:l.pos], start}, nil
			case '<':
				if p+1 < len(l.s) && (l.s[p+1] == '>' || l.s[p+1] == '=' || l.s[p+1] == '<') {
					l.pos = p + 2
				} else {
					l.pos = p + 1
				}

				return token{tokOp, l.s[start:l.pos], start}, nil
			case '>':
				if p+1 < len(l.s) && (l.s[p+1] == '=' || l.s[p+1] == '<' || l.s[p+1] == '>') {
					l.pos = p + 2
				} else {
					l.pos = p + 1
				}

				return token{tokOp, l.s[start:l.pos], start}, nil
			}
		}

		return token{}, &Error{start, "'~' must be followed by a comparison operator"}
	case c == '-' || (c >= '0' && c <= '9'):
		l.pos++
		for l.pos < len(l.s) && (l.s[l.pos] >= '0' && l.s[l.pos] <= '9' || l.s[l.pos] == '.') {
			l.pos++
		}

		if l.s[start:l.pos] == "-" {
			return token{}, &Error{start, "'-' is not a number"}
		}

		return token{tokNumber, l.s[start:l.pos], start}, nil
	}

	r := []rune(l.s[l.pos:])[0]
	if isIdentStart(r) {
		for l.pos < len(l.s) {
			r := []rune(l.s[l.pos:])[0]
			if !isIdentPart(r) {
				break
			}

			l.pos += len(string(r))
		}

		text := l.s[start:l.pos]
		if keywords[strings.ToUpper(text)] {
			return token{tokKeyword, strings.ToUpper(text), start}, nil
		}

		return token{tokIdent, text, start}, nil
	}

	return token{}, &Error{start, fmt.Sprintf("unexpected character %q", r)}
}

type parser struct {
	lex *lexer
	tok token
}

func (p *parser) advance() error {
	t, err := p.lex.next()
	if err != nil {
		return err
	}

	p.tok = t

	return nil
}

// Check reports whether s is a syntactically valid condition. An empty or
// blank condition is valid (it means "no condition").
func Check(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	p := &parser{lex: &lexer{s: s}}
	if err := p.advance(); err != nil {
		return err
	}

	if err := p.expr(); err != nil {
		return err
	}

	if p.tok.kind != tokEOF {
		return &Error{p.tok.pos, fmt.Sprintf("unexpected %q", p.tok.text)}
	}

	return nil
}

// binary parses a left-associative chain of the keyword over the next level.
func (p *parser) binary(keyword string, next func() error) error {
	if err := next(); err != nil {
		return err
	}

	for p.tok.kind == tokKeyword && p.tok.text == keyword {
		if err := p.advance(); err != nil {
			return err
		}

		if err := next(); err != nil {
			return err
		}
	}

	return nil
}

func (p *parser) expr() error { return p.binary("IMP", p.eqv) }
func (p *parser) eqv() error  { return p.binary("EQV", p.xor) }
func (p *parser) xor() error  { return p.binary("XOR", p.or) }
func (p *parser) or() error   { return p.binary("OR", p.and) }
func (p *parser) and() error  { return p.binary("AND", p.not) }

func (p *parser) not() error {
	if p.tok.kind == tokKeyword && p.tok.text == "NOT" {
		if err := p.advance(); err != nil {
			return err
		}

		return p.not()
	}

	return p.term()
}

func (p *parser) term() error {
	if p.tok.kind == tokLParen {
		if err := p.advance(); err != nil {
			return err
		}

		if err := p.expr(); err != nil {
			return err
		}

		if p.tok.kind != tokRParen {
			return &Error{p.tok.pos, "missing ')'"}
		}

		return p.advance()
	}

	if err := p.value(); err != nil {
		return err
	}

	if p.tok.kind == tokOp {
		if err := p.advance(); err != nil {
			return err
		}

		return p.value()
	}

	return nil
}

func (p *parser) value() error {
	switch p.tok.kind {
	case tokIdent, tokNumber, tokString, tokPrefix:
		return p.advance()
	case tokEOF:
		return &Error{p.tok.pos, "unexpected end of condition"}
	case tokKeyword:
		return &Error{p.tok.pos, fmt.Sprintf("unexpected keyword %s", p.tok.text)}
	default:
		return &Error{p.tok.pos, fmt.Sprintf("unexpected %q", p.tok.text)}
	}
}

// Properties returns the property names a condition refers to, without the
// feature, component and environment references.
func Properties(s string) []string {
	l := &lexer{s: s}

	var out []string

	for {
		t, err := l.next()
		if err != nil || t.kind == tokEOF {
			return out
		}

		if t.kind == tokIdent {
			out = append(out, t.text)
		}
	}
}
