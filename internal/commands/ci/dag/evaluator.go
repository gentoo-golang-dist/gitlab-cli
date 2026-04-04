package dag

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// EvalExpression evaluates a GitLab CI rules:if expression against a variable map.
// Returns true if the expression is satisfied.
func EvalExpression(expr string, vars map[string]string) (bool, error) {
	t := &tokenizer{input: strings.TrimSpace(expr)}
	p := &parser{t: t}

	node, err := p.parseExpression()
	if err != nil {
		return false, fmt.Errorf("parsing expression %q: %w", expr, err)
	}

	tok := t.next()
	if tok.kind != tokEOF {
		return false, fmt.Errorf("unexpected token %q after expression", tok.value)
	}

	return node.eval(vars)
}

// --- Token types ---

type tokenKind int

const (
	tokVariable tokenKind = iota
	tokString
	tokNull
	tokEq       // ==
	tokNeq      // !=
	tokMatch    // =~
	tokNotMatch // !~
	tokAnd      // &&
	tokOr       // ||
	tokNot      // ! (prefix)
	tokLParen
	tokRParen
	tokRegex // /pattern/ or /pattern/i
	tokEOF
)

type token struct {
	kind  tokenKind
	value string
}

// --- Tokenizer ---

type tokenizer struct {
	input   string
	pos     int
	wantRex bool // true after =~ or !~ to expect regex
	peeked  *token
	hasPeek bool
}

func (t *tokenizer) peek() token {
	if t.hasPeek {
		return *t.peeked
	}
	tok := t.next()
	t.peeked = &tok
	t.hasPeek = true
	return tok
}

func (t *tokenizer) next() token {
	if t.hasPeek {
		t.hasPeek = false
		return *t.peeked
	}
	return t.scan()
}

func (t *tokenizer) scan() token {
	t.skipWhitespace()
	if t.pos >= len(t.input) {
		return token{kind: tokEOF}
	}

	// If we expect a regex (after =~ or !~), parse it.
	if t.wantRex && t.input[t.pos] == '/' {
		t.wantRex = false
		return t.scanRegex()
	}
	t.wantRex = false

	ch := t.input[t.pos]

	switch {
	case ch == '$':
		return t.scanVariable()
	case ch == '"' || ch == '\'':
		return t.scanString(ch)
	case ch == '(':
		t.pos++
		return token{kind: tokLParen, value: "("}
	case ch == ')':
		t.pos++
		return token{kind: tokRParen, value: ")"}
	case ch == '=' && t.pos+1 < len(t.input) && t.input[t.pos+1] == '=':
		t.pos += 2
		return token{kind: tokEq, value: "=="}
	case ch == '=' && t.pos+1 < len(t.input) && t.input[t.pos+1] == '~':
		t.pos += 2
		t.wantRex = true
		return token{kind: tokMatch, value: "=~"}
	case ch == '!' && t.pos+1 < len(t.input) && t.input[t.pos+1] == '=':
		t.pos += 2
		return token{kind: tokNeq, value: "!="}
	case ch == '!' && t.pos+1 < len(t.input) && t.input[t.pos+1] == '~':
		t.pos += 2
		t.wantRex = true
		return token{kind: tokNotMatch, value: "!~"}
	case ch == '&' && t.pos+1 < len(t.input) && t.input[t.pos+1] == '&':
		t.pos += 2
		return token{kind: tokAnd, value: "&&"}
	case ch == '|' && t.pos+1 < len(t.input) && t.input[t.pos+1] == '|':
		t.pos += 2
		return token{kind: tokOr, value: "||"}
	case ch == '!':
		t.pos++
		return token{kind: tokNot, value: "!"}
	default:
		// Try to read a bare word (null).
		if t.pos+4 <= len(t.input) && t.input[t.pos:t.pos+4] == "null" {
			// Ensure "null" is not part of a longer identifier.
			if t.pos+4 >= len(t.input) || !isIdentChar(rune(t.input[t.pos+4])) {
				t.pos += 4
				return token{kind: tokNull, value: "null"}
			}
		}
		return token{kind: tokEOF, value: string(ch)}
	}
}

func (t *tokenizer) scanVariable() token {
	t.pos++ // skip $
	start := t.pos
	for t.pos < len(t.input) && isIdentChar(rune(t.input[t.pos])) {
		t.pos++
	}
	return token{kind: tokVariable, value: t.input[start:t.pos]}
}

func (t *tokenizer) scanString(quote byte) token {
	t.pos++ // skip opening quote
	start := t.pos
	for t.pos < len(t.input) && t.input[t.pos] != quote {
		t.pos++
	}
	val := t.input[start:t.pos]
	if t.pos < len(t.input) {
		t.pos++ // skip closing quote
	}
	return token{kind: tokString, value: val}
}

func (t *tokenizer) scanRegex() token {
	t.pos++ // skip opening /
	start := t.pos
	for t.pos < len(t.input) && t.input[t.pos] != '/' {
		if t.input[t.pos] == '\\' && t.pos+1 < len(t.input) {
			t.pos++ // skip escaped char
		}
		t.pos++
	}
	pattern := t.input[start:t.pos]
	if t.pos < len(t.input) {
		t.pos++ // skip closing /
	}
	// Check for flags.
	flags := ""
	if t.pos < len(t.input) && t.input[t.pos] == 'i' {
		flags = "i"
		t.pos++
	}
	return token{kind: tokRegex, value: "/" + pattern + "/" + flags}
}

func (t *tokenizer) skipWhitespace() {
	for t.pos < len(t.input) && unicode.IsSpace(rune(t.input[t.pos])) {
		t.pos++
	}
}

func isIdentChar(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// --- AST Nodes ---

type exprNode interface {
	eval(vars map[string]string) (bool, error)
}

// varNode represents a $VARIABLE reference.
type varNode struct {
	name string
}

func (n *varNode) eval(vars map[string]string) (bool, error) {
	v, ok := vars[n.name]
	return ok && v != "", nil
}

func (n *varNode) resolve(vars map[string]string) string {
	return vars[n.name]
}

// literalNode represents a string literal or null.
type literalNode struct {
	value  string
	isNull bool
}

func (n *literalNode) eval(_ map[string]string) (bool, error) {
	if n.isNull {
		return false, nil
	}
	return n.value != "", nil
}

// notNode represents the ! prefix operator.
type notNode struct {
	operand exprNode
}

func (n *notNode) eval(vars map[string]string) (bool, error) {
	result, err := n.operand.eval(vars)
	if err != nil {
		return false, err
	}
	return !result, nil
}

// comparisonNode represents ==, !=, =~, !~.
type comparisonNode struct {
	op    tokenKind
	left  exprNode
	right exprNode
}

func (n *comparisonNode) eval(vars map[string]string) (bool, error) {
	leftVal := resolveValue(n.left, vars)
	leftIsNull := isNullValue(n.left, vars)

	switch n.op {
	case tokEq:
		if isNullLiteral(n.right) {
			return leftIsNull, nil
		}
		rightVal := resolveValue(n.right, vars)
		if isNullLiteral(n.left) {
			return isNullValue(n.right, vars), nil
		}
		return leftVal == rightVal, nil

	case tokNeq:
		if isNullLiteral(n.right) {
			return !leftIsNull, nil
		}
		rightVal := resolveValue(n.right, vars)
		if isNullLiteral(n.left) {
			return !isNullValue(n.right, vars), nil
		}
		return leftVal != rightVal, nil

	case tokMatch:
		rightVal := resolveValue(n.right, vars)
		return evalRegex(rightVal, leftVal)

	case tokNotMatch:
		rightVal := resolveValue(n.right, vars)
		matched, err := evalRegex(rightVal, leftVal)
		if err != nil {
			return false, err
		}
		return !matched, nil
	}

	return false, fmt.Errorf("unknown comparison operator")
}

// logicalNode represents && or ||.
type logicalNode struct {
	op    tokenKind
	left  exprNode
	right exprNode
}

func (n *logicalNode) eval(vars map[string]string) (bool, error) {
	leftResult, err := n.left.eval(vars)
	if err != nil {
		return false, err
	}

	// Short-circuit evaluation.
	if n.op == tokAnd && !leftResult {
		return false, nil
	}
	if n.op == tokOr && leftResult {
		return true, nil
	}

	return n.right.eval(vars)
}

// --- Helper functions ---

func resolveValue(node exprNode, vars map[string]string) string {
	switch n := node.(type) {
	case *varNode:
		return n.resolve(vars)
	case *literalNode:
		return n.value
	}
	return ""
}

func isNullValue(node exprNode, vars map[string]string) bool {
	switch n := node.(type) {
	case *varNode:
		v, ok := vars[n.name]
		return !ok || v == ""
	case *literalNode:
		return n.isNull
	}
	return false
}

func isNullLiteral(node exprNode) bool {
	if lit, ok := node.(*literalNode); ok {
		return lit.isNull
	}
	return false
}

func evalRegex(pattern, input string) (bool, error) {
	// Pattern is like "/foo/i" -- strip delimiters, extract flags.
	if !strings.HasPrefix(pattern, "/") {
		return false, fmt.Errorf("invalid regex %q: must start with /", pattern)
	}
	inner := pattern[1:]
	caseInsensitive := false
	if strings.HasSuffix(inner, "/i") {
		inner = inner[:len(inner)-2]
		caseInsensitive = true
	} else if strings.HasSuffix(inner, "/") {
		inner = inner[:len(inner)-1]
	} else {
		return false, fmt.Errorf("invalid regex %q: must end with /", pattern)
	}
	if caseInsensitive {
		inner = "(?i)" + inner
	}
	re, err := regexp.Compile(inner)
	if err != nil {
		return false, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	return re.MatchString(input), nil
}

// --- Parser ---

type parser struct {
	t *tokenizer
}

func (p *parser) parseExpression() (exprNode, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (exprNode, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.t.peek().kind == tokOr {
		p.t.next() // consume ||
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &logicalNode{op: tokOr, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (exprNode, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.t.peek().kind == tokAnd {
		p.t.next() // consume &&
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &logicalNode{op: tokAnd, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseComparison() (exprNode, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	tok := p.t.peek()
	switch tok.kind {
	case tokEq, tokNeq, tokMatch, tokNotMatch:
		p.t.next() // consume operator
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &comparisonNode{op: tok.kind, left: left, right: right}, nil
	}

	return left, nil
}

func (p *parser) parsePrimary() (exprNode, error) {
	tok := p.t.next()

	switch tok.kind {
	case tokVariable:
		return &varNode{name: tok.value}, nil
	case tokString:
		return &literalNode{value: tok.value}, nil
	case tokNull:
		return &literalNode{isNull: true}, nil
	case tokRegex:
		return &literalNode{value: tok.value}, nil
	case tokNot:
		operand, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &notNode{operand: operand}, nil
	case tokLParen:
		node, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if p.t.next().kind != tokRParen {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		return node, nil
	case tokEOF:
		return nil, fmt.Errorf("unexpected end of expression")
	default:
		return nil, fmt.Errorf("unexpected token %q", tok.value)
	}
}
