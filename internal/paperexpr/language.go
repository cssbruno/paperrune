// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperexpr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// LanguageLimits bounds source parsing and the VM program produced by Compile.
// A zero value uses DefaultLanguageLimits. Nonzero limits must be complete.
type LanguageLimits struct {
	MaxSourceBytes uint32
	MaxTokens      uint32
	MaxDepth       uint32
	MaxNodes       uint32
	Program        Limits
}

func DefaultLanguageLimits() LanguageLimits {
	return LanguageLimits{
		MaxSourceBytes: 1 << 20,
		MaxTokens:      4096,
		MaxDepth:       128,
		MaxNodes:       4096,
		Program:        DefaultLimits(),
	}
}

// PathKind is an explicit static binding contract. Compile rejects duplicate,
// invalid, undeclared, or unsupported path kinds before producing bytecode.
type PathKind struct {
	Path     string
	Kind     Kind
	Optional bool
}

// Expression is an immutable parsed expression. Its syntax tree is intentionally
// private so callers cannot construct an unbounded or invalid tree.
type Expression struct {
	root       *expressionNode
	source     string
	tokenCount uint32
	nodeCount  uint32
	maxDepth   uint32
}

func (e Expression) Source() string { return e.source }

// ExpressionError identifies the byte-offset range responsible for a parse or
// compile failure. Cause is one of ErrInvalid, ErrType, ErrBinding, or ErrLimit.
type ExpressionError struct {
	Offset  uint32
	End     uint32
	Problem string
	Cause   error
}

func (e *ExpressionError) Error() string {
	return fmt.Sprintf("%v at bytes %d:%d: %s", e.Cause, e.Offset, e.End, e.Problem)
}

func (e *ExpressionError) Unwrap() error { return e.Cause }

// Parse accepts this precedence, from tightest to loosest:
// primary/parentheses, !, typed +, ==/matches, &&, ||, and right-associative ?:.
func Parse(source string, limits LanguageLimits) (Expression, error) {
	normalized, err := normalizeLanguageLimits(limits)
	if err != nil {
		return Expression{}, expressionError(0, 0, "language limits are incomplete or exceed hard caps", ErrLimit)
	}
	if uint64(len(source)) > uint64(normalized.MaxSourceBytes) {
		return Expression{}, expressionError(normalized.MaxSourceBytes, boundedSourceOffset(len(source)), "source exceeds MaxSourceBytes", ErrLimit)
	}
	if !utf8.ValidString(source) {
		return Expression{}, expressionError(0, uint32(len(source)), "source is not valid UTF-8", ErrInvalid) // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
	}
	tokens, err := lexExpression(source, normalized)
	if err != nil {
		return Expression{}, err
	}
	parser := expressionParser{tokens: tokens, limits: normalized}
	root, err := parser.parseSelect(1)
	if err != nil {
		return Expression{}, err
	}
	if token := parser.peek(); token.kind != tokenEOF {
		return Expression{}, expressionError(token.start, token.end, fmt.Sprintf("unexpected token %q after expression", token.text), ErrInvalid)
	}
	return Expression{
		root:       root,
		source:     source,
		tokenCount: uint32(len(tokens) - 1), // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
		nodeCount:  parser.nodes,
		maxDepth:   parser.peakDepth,
	}, nil
}

// ComponentReferences returns the closed @component literals mentioned by an
// expression, including literals in lazy branches. Quoted strings that happen
// to begin with @ are deliberately not references.
func ComponentReferences(source string, limits LanguageLimits) ([]string, error) {
	normalized, err := normalizeLanguageLimits(limits)
	if err != nil {
		return nil, err
	}
	if _, err := Parse(source, normalized); err != nil {
		return nil, err
	}
	tokens, err := lexExpression(source, normalized)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool)
	for _, token := range tokens {
		if token.kind == tokenComponentReference {
			set[token.value.String] = true
		}
	}
	result := make([]string, 0, len(set))
	for reference := range set {
		result = append(result, reference)
	}
	sort.Strings(result)
	return result, nil
}

// ValidateComponentSelection proves that every possible result is a closed
// unquoted @component literal or null. Conditions may use the ordinary typed
// expression language, but schema strings cannot become component names.
func ValidateComponentSelection(source string, limits LanguageLimits) ([]string, error) {
	expression, err := Parse(source, limits)
	if err != nil {
		return nil, err
	}
	references := make(map[string]bool)
	var validate func(*expressionNode) error
	validate = func(node *expressionNode) error {
		if node == nil {
			return expressionError(0, 0, "component selection has no result", ErrInvalid)
		}
		if node.kind == nodeSelect {
			if err := validate(node.right); err != nil {
				return err
			}
			return validate(node.alt)
		}
		if node.kind == nodeLiteral && node.value.Kind == Null {
			return nil
		}
		if node.kind == nodeLiteral && node.value.Kind == String && node.value.Reference {
			references[node.value.String] = true
			return nil
		}
		return expressionError(node.start, node.end, "component result must be an unquoted @component reference or null", ErrType)
	}
	if err := validate(expression.root); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(references))
	for reference := range references {
		result = append(result, reference)
	}
	sort.Strings(result)
	return result, nil
}

// Compile parses, statically checks, and emits deterministic VM bytecode. The
// returned Kind is the expression result kind.
func Compile(source string, environment []PathKind, limits LanguageLimits) (Program, Kind, error) {
	expression, err := Parse(source, limits)
	if err != nil {
		return Program{}, Null, err
	}
	return CompileExpression(expression, environment, limits)
}

// CompileExpression statically checks a previously parsed expression and emits
// sorted/deduplicated constants and paths plus deterministic postfix bytecode.
func CompileExpression(expression Expression, environment []PathKind, limits LanguageLimits) (Program, Kind, error) {
	normalized, err := normalizeLanguageLimits(limits)
	if err != nil {
		return Program{}, Null, expressionError(0, 0, "language limits are incomplete or exceed hard caps", ErrLimit)
	}
	if expression.root == nil {
		return Program{}, Null, expressionError(0, 0, "expression has no parsed syntax tree", ErrInvalid)
	}
	if uint64(len(expression.source)) > uint64(normalized.MaxSourceBytes) || expression.tokenCount > normalized.MaxTokens ||
		expression.nodeCount > normalized.MaxNodes || expression.maxDepth > normalized.MaxDepth {
		return Program{}, Null, expressionError(expression.root.start, expression.root.end, "parsed expression exceeds compile limits", ErrLimit)
	}
	kinds, err := normalizePathKinds(environment, normalized.Program)
	if err != nil {
		return Program{}, Null, err
	}
	compiler := expressionCompiler{pathKinds: kinds, limits: normalized.Program}
	resultKind, err := compiler.check(expression.root)
	if err != nil {
		return Program{}, Null, err
	}
	compiler.indexInputs(expression.root)
	if uint32(len(compiler.constants)) > normalized.Program.MaxConstants { // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
		return Program{}, Null, expressionError(expression.root.start, expression.root.end, "constant count exceeds MaxConstants", ErrLimit)
	}
	if uint32(len(compiler.paths)) > normalized.Program.MaxPaths { // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
		return Program{}, Null, expressionError(expression.root.start, expression.root.end, "path count exceeds MaxPaths", ErrLimit)
	}
	program := Program{Constants: compiler.constants, Paths: compiler.paths, root: expression.root, ResultNullable: expression.root.nullable}
	for _, path := range compiler.paths {
		if contract := kinds[path]; contract.Optional {
			program.OptionalPaths = append(program.OptionalPaths, path)
		}
	}
	compiler.emit(expression.root, &program.Code)
	if uint32(len(program.Code)) > normalized.Program.MaxInstructions { // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
		return Program{}, Null, expressionError(expression.root.start, expression.root.end, "instruction count exceeds MaxInstructions", ErrLimit)
	}
	if _, err := validateProgram(program, normalized.Program); err != nil {
		return Program{}, Null, expressionError(expression.root.start, expression.root.end, "generated program exceeds VM limits", err)
	}
	return program, resultKind, nil
}

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenNull
	tokenBool
	tokenInteger
	tokenString
	tokenComponentReference
	tokenPath
	tokenLeftParen
	tokenRightParen
	tokenNot
	tokenEqual
	tokenMatches
	tokenAnd
	tokenOr
	tokenPlus
	tokenMinus
	tokenMultiply
	tokenDivide
	tokenNotEqual
	tokenLess
	tokenLessEqual
	tokenGreater
	tokenGreaterEqual
	tokenQuestion
	tokenColon
)

type expressionToken struct {
	kind       tokenKind
	start, end uint32
	text       string
	value      Value
}

func lexExpression(source string, limits LanguageLimits) ([]expressionToken, error) {
	tokens := make([]expressionToken, 0)
	for offset := 0; offset < len(source); {
		character := source[offset]
		if character == ' ' || character == '\t' || character == '\r' || character == '\n' {
			offset++
			continue
		}
		start := offset
		var token expressionToken
		switch character {
		case '(':
			offset++
			token = simpleToken(tokenLeftParen, source, start, offset)
		case ')':
			offset++
			token = simpleToken(tokenRightParen, source, start, offset)
		case '+':
			offset++
			token = simpleToken(tokenPlus, source, start, offset)
		case '-':
			if offset+1 < len(source) && source[offset+1] >= '0' && source[offset+1] <= '9' && expressionCanStartSignedLiteral(tokens) {
				var err error
				token, offset, err = lexExpressionNumber(source, start)
				if err != nil {
					return nil, err
				}
			} else {
				offset++
				token = simpleToken(tokenMinus, source, start, offset)
			}
		case '*':
			offset++
			token = simpleToken(tokenMultiply, source, start, offset)
		case '/':
			offset++
			token = simpleToken(tokenDivide, source, start, offset)
		case '?':
			offset++
			token = simpleToken(tokenQuestion, source, start, offset)
		case ':':
			offset++
			token = simpleToken(tokenColon, source, start, offset)
		case '!':
			if offset+1 < len(source) && source[offset+1] == '=' {
				offset += 2
				token = simpleToken(tokenNotEqual, source, start, offset)
			} else {
				offset++
				token = simpleToken(tokenNot, source, start, offset)
			}
		case '<', '>':
			offset++
			inclusive := offset < len(source) && source[offset] == '='
			if inclusive {
				offset++
			}
			kind := tokenLess
			if character == '>' {
				kind = tokenGreater
			}
			if inclusive && character == '<' {
				kind = tokenLessEqual
			} else if inclusive {
				kind = tokenGreaterEqual
			}
			token = simpleToken(kind, source, start, offset)
		case '@':
			offset++
			if offset >= len(source) || !isPathStart(source[offset]) || source[offset] == '_' {
				return nil, expressionError(uint32(start), uint32(offset), "component reference requires a readable @name", ErrInvalid)
			}
			for offset < len(source) && (isPathStart(source[offset]) || source[offset] >= '0' && source[offset] <= '9' || source[offset] == '-') {
				offset++
			}
			text := source[start:offset]
			token = expressionToken{kind: tokenComponentReference, start: uint32(start), end: uint32(offset), text: text, value: Value{Kind: String, String: text, Reference: true}}
		case '=':
			if offset+1 >= len(source) || source[offset+1] != '=' {
				return nil, expressionError(uint32(start), uint32(start+1), "expected ==", ErrInvalid)
			}
			offset += 2
			token = simpleToken(tokenEqual, source, start, offset)
		case '&':
			if offset+1 >= len(source) || source[offset+1] != '&' {
				return nil, expressionError(uint32(start), uint32(start+1), "expected &&", ErrInvalid)
			}
			offset += 2
			token = simpleToken(tokenAnd, source, start, offset)
		case '|':
			if offset+1 >= len(source) || source[offset+1] != '|' {
				return nil, expressionError(uint32(start), uint32(start+1), "expected ||", ErrInvalid)
			}
			offset += 2
			token = simpleToken(tokenOr, source, start, offset)
		case '"':
			var err error
			token, offset, err = lexExpressionString(source, start)
			if err != nil {
				return nil, err
			}
		default:
			if character >= '0' && character <= '9' {
				var err error
				token, offset, err = lexExpressionNumber(source, start)
				if err != nil {
					return nil, err
				}
			} else if isPathStart(character) {
				for offset < len(source) && isPathByte(source[offset]) {
					offset++
				}
				text := source[start:offset]
				switch text {
				case "null":
					token = expressionToken{kind: tokenNull, start: uint32(start), end: uint32(offset), text: text, value: Value{Kind: Null}}
				case "true":
					token = expressionToken{kind: tokenBool, start: uint32(start), end: uint32(offset), text: text, value: Value{Kind: Bool, Bool: true}}
				case "false":
					token = expressionToken{kind: tokenBool, start: uint32(start), end: uint32(offset), text: text, value: Value{Kind: Bool}}
				case "matches":
					token = expressionToken{kind: tokenMatches, start: uint32(start), end: uint32(offset), text: text}
				default:
					if !validPath(text) {
						return nil, expressionError(uint32(start), uint32(offset), "invalid dotted binding path", ErrInvalid)
					}
					token = expressionToken{kind: tokenPath, start: uint32(start), end: uint32(offset), text: text}
				}
			} else {
				_, size := utf8.DecodeRuneInString(source[offset:])
				return nil, expressionError(uint32(start), uint32(start+size), fmt.Sprintf("unexpected character %q", source[start:start+size]), ErrInvalid) // #nosec G115 -- fixed-width conversion is bounded by the surrounding parser, planner, or resource invariant
			}
		}
		tokens = append(tokens, token)
		if uint32(len(tokens)) > limits.MaxTokens { // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
			return nil, expressionError(token.start, token.end, "token count exceeds MaxTokens", ErrLimit)
		}
	}
	tokens = append(tokens, expressionToken{kind: tokenEOF, start: uint32(len(source)), end: uint32(len(source))}) // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
	return tokens, nil
}

func simpleToken(kind tokenKind, source string, start, end int) expressionToken {
	return expressionToken{kind: kind, start: uint32(start), end: uint32(end), text: source[start:end]} // #nosec G115 -- fixed-width conversion is bounded by the surrounding parser, planner, or resource invariant
}

func expressionCanStartSignedLiteral(tokens []expressionToken) bool {
	if len(tokens) == 0 {
		return true
	}
	switch tokens[len(tokens)-1].kind {
	case tokenLeftParen, tokenNot, tokenEqual, tokenMatches, tokenAnd, tokenOr,
		tokenPlus, tokenMinus, tokenMultiply, tokenDivide, tokenNotEqual, tokenLess,
		tokenLessEqual, tokenGreater, tokenGreaterEqual, tokenQuestion, tokenColon:
		return true
	default:
		return false
	}
}

func lexExpressionNumber(source string, start int) (expressionToken, int, error) {
	offset := start
	if source[offset] == '-' {
		offset++
	}
	for offset < len(source) && source[offset] >= '0' && source[offset] <= '9' {
		offset++
	}
	if offset < len(source) && source[offset] == '.' {
		offset++
		fractionStart := offset
		for offset < len(source) && source[offset] >= '0' && source[offset] <= '9' {
			offset++
		}
		if offset == fractionStart {
			return expressionToken{}, offset, expressionError(uint32(start), uint32(offset), "decimal point requires fractional digits", ErrInvalid) // #nosec G115 -- source offsets are bounded by MaxSourceBytes before tokenization
		}
	}
	numberEnd := offset
	unit := ""
	if offset < len(source) && source[offset] == '%' {
		unit, offset = "%", offset+1
	} else {
		unitStart := offset
		for offset < len(source) && source[offset] >= 'a' && source[offset] <= 'z' {
			offset++
		}
		if offset > unitStart {
			unit = source[unitStart:offset]
		}
	}
	raw := source[start:numberEnd]
	value, err := ParseNumber(raw)
	if err != nil {
		return expressionToken{}, offset, expressionError(uint32(start), uint32(numberEnd), strings.TrimPrefix(err.Error(), ErrInvalid.Error()+": "), ErrInvalid) // #nosec G115 -- source offset is bounded by validated input or parser state
	}
	if unit != "" {
		if !validUnit(unit) {
			return expressionToken{}, offset, expressionError(uint32(numberEnd), uint32(offset), fmt.Sprintf("unsupported unit %q", unit), ErrType) // #nosec G115 -- source offsets are bounded by MaxSourceBytes before tokenization
		}
		value.Kind, value.Unit = Unit, unit
	}
	return expressionToken{kind: tokenInteger, start: uint32(start), end: uint32(offset), text: source[start:offset], value: value}, offset, nil // #nosec G115 -- source offset is bounded by validated input or parser state
}

func lexExpressionString(source string, start int) (expressionToken, int, error) {
	offset := start + 1
	for offset < len(source) {
		character := source[offset]
		if character == '"' {
			offset++
			raw := source[start:offset]
			if err := validateJSONStringEscapes(raw); err != nil {
				return expressionToken{}, offset, expressionError(uint32(start), uint32(offset), err.Error(), ErrInvalid) // #nosec G115 -- source offset is bounded by validated input or parser state
			}
			var decoded string
			if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
				return expressionToken{}, offset, expressionError(uint32(start), uint32(offset), "invalid quoted UTF-8 string", ErrInvalid) // #nosec G115 -- source offset is bounded by validated input or parser state
			}
			return expressionToken{kind: tokenString, start: uint32(start), end: uint32(offset), text: raw, value: Value{Kind: String, String: decoded}}, offset, nil // #nosec G115 -- source offset is bounded by validated input or parser state
		}
		if character < 0x20 {
			return expressionToken{}, offset, expressionError(uint32(offset), uint32(offset+1), "unescaped control character in string", ErrInvalid) // #nosec G115 -- source offset is bounded by validated input or parser state
		}
		if character == '\\' {
			offset += 2
			continue
		}
		_, size := utf8.DecodeRuneInString(source[offset:])
		offset += size
	}
	return expressionToken{}, offset, expressionError(uint32(start), uint32(len(source)), "unterminated string literal", ErrInvalid) // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
}

func validateJSONStringEscapes(raw string) error {
	for index := 1; index < len(raw)-1; index++ {
		if raw[index] != '\\' {
			continue
		}
		index++
		if index >= len(raw)-1 {
			return errors.New("incomplete string escape")
		}
		switch raw[index] {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		case 'u':
			if index+4 >= len(raw) {
				return errors.New("incomplete Unicode escape")
			}
			first, ok := parseHex16(raw[index+1 : index+5])
			if !ok {
				return errors.New("invalid Unicode escape")
			}
			index += 4
			if first >= 0xd800 && first <= 0xdbff {
				if index+6 >= len(raw) || raw[index+1:index+3] != `\u` {
					return errors.New("high surrogate requires a low surrogate")
				}
				second, ok := parseHex16(raw[index+3 : index+7])
				if !ok || second < 0xdc00 || second > 0xdfff {
					return errors.New("high surrogate requires a low surrogate")
				}
				index += 6
			} else if first >= 0xdc00 && first <= 0xdfff {
				return errors.New("low surrogate has no high surrogate")
			}
		default:
			return fmt.Errorf("unsupported string escape \\%c", raw[index])
		}
	}
	return nil
}

func parseHex16(value string) (uint16, bool) {
	parsed, err := strconv.ParseUint(value, 16, 16)
	return uint16(parsed), err == nil
}

func isPathStart(character byte) bool {
	return character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func isPathByte(character byte) bool {
	return isPathStart(character) || character >= '0' && character <= '9' || character == '-' || character == '.'
}

type expressionNodeKind uint8

const (
	nodeLiteral expressionNodeKind = iota
	nodePath
	nodeNot
	nodeEqual
	nodeMatches
	nodeAnd
	nodeOr
	nodePlus
	nodeMinus
	nodeMultiply
	nodeDivide
	nodeNegate
	nodeNotEqual
	nodeLess
	nodeLessEqual
	nodeGreater
	nodeGreaterEqual
	nodeSelect
)

type expressionNode struct {
	kind             expressionNodeKind
	start, end       uint32
	opOffset         uint32
	value            Value
	path             string
	left, right, alt *expressionNode
	inferred         Kind
	nullable         bool
	height           uint32
}

type expressionParser struct {
	tokens    []expressionToken
	index     int
	nodes     uint32
	peakDepth uint32
	limits    LanguageLimits
}

func (p *expressionParser) parseSelect(depth uint32) (*expressionNode, error) {
	if err := p.checkDepth(depth); err != nil {
		return nil, err
	}
	condition, err := p.parseOr(depth)
	if err != nil || p.peek().kind != tokenQuestion {
		return condition, err
	}
	operator := p.take()
	whenTrue, err := p.parseSelect(depth + 1)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenColon, "conditional expression requires :"); err != nil {
		return nil, err
	}
	whenFalse, err := p.parseSelect(depth + 1)
	if err != nil {
		return nil, err
	}
	return p.node(nodeSelect, condition.start, whenFalse.end, operator.start, condition, whenTrue, whenFalse)
}

func (p *expressionParser) parseOr(depth uint32) (*expressionNode, error) {
	return p.parseBinary(depth, p.parseAnd, tokenOr, nodeOr)
}

func (p *expressionParser) parseAnd(depth uint32) (*expressionNode, error) {
	return p.parseBinary(depth, p.parseEqual, tokenAnd, nodeAnd)
}

func (p *expressionParser) parseEqual(depth uint32) (*expressionNode, error) {
	left, err := p.parseCompare(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokenEqual || p.peek().kind == tokenNotEqual || p.peek().kind == tokenMatches {
		operator := p.take()
		right, err := p.parsePlus(depth)
		if err != nil {
			return nil, err
		}
		kind := nodeEqual
		switch operator.kind {
		case tokenNotEqual:
			kind = nodeNotEqual
		case tokenMatches:
			kind = nodeMatches
		}
		left, err = p.node(kind, left.start, right.end, operator.start, left, right, nil)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *expressionParser) parseCompare(depth uint32) (*expressionNode, error) {
	left, err := p.parsePlus(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokenLess || p.peek().kind == tokenLessEqual || p.peek().kind == tokenGreater || p.peek().kind == tokenGreaterEqual {
		operator := p.take()
		right, err := p.parsePlus(depth)
		if err != nil {
			return nil, err
		}
		kind := nodeLess
		switch operator.kind {
		case tokenLessEqual:
			kind = nodeLessEqual
		case tokenGreater:
			kind = nodeGreater
		case tokenGreaterEqual:
			kind = nodeGreaterEqual
		}
		left, err = p.node(kind, left.start, right.end, operator.start, left, right, nil)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *expressionParser) parsePlus(depth uint32) (*expressionNode, error) {
	left, err := p.parseMultiply(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokenPlus || p.peek().kind == tokenMinus {
		operator := p.take()
		right, err := p.parseMultiply(depth)
		if err != nil {
			return nil, err
		}
		kind := nodePlus
		if operator.kind == tokenMinus {
			kind = nodeMinus
		}
		left, err = p.node(kind, left.start, right.end, operator.start, left, right, nil)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *expressionParser) parseMultiply(depth uint32) (*expressionNode, error) {
	left, err := p.parseUnary(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokenMultiply || p.peek().kind == tokenDivide {
		operator := p.take()
		right, err := p.parseUnary(depth)
		if err != nil {
			return nil, err
		}
		kind := nodeMultiply
		if operator.kind == tokenDivide {
			kind = nodeDivide
		}
		left, err = p.node(kind, left.start, right.end, operator.start, left, right, nil)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

type parseLevel func(uint32) (*expressionNode, error)

func (p *expressionParser) parseBinary(depth uint32, next parseLevel, tokenKind tokenKind, nodeKind expressionNodeKind) (*expressionNode, error) {
	left, err := next(depth)
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokenKind {
		operator := p.take()
		right, err := next(depth)
		if err != nil {
			return nil, err
		}
		left, err = p.node(nodeKind, left.start, right.end, operator.start, left, right, nil)
		if err != nil {
			return nil, err
		}
	}
	return left, nil
}

func (p *expressionParser) parseUnary(depth uint32) (*expressionNode, error) {
	if token := p.peek(); token.kind == tokenNot {
		p.take()
		child, err := p.parseUnary(depth + 1)
		if err != nil {
			return nil, err
		}
		return p.node(nodeNot, token.start, child.end, token.start, child, nil, nil)
	}
	if token := p.peek(); token.kind == tokenMinus {
		p.take()
		child, err := p.parseUnary(depth + 1)
		if err != nil {
			return nil, err
		}
		if child.kind == nodeLiteral && (child.value.Kind == Integer || child.value.Kind == Unit) && child.value.Integer == 0 {
			return nil, expressionError(token.start, child.end, "negative zero is not canonical", ErrInvalid)
		}
		return p.node(nodeNegate, token.start, child.end, token.start, child, nil, nil)
	}
	return p.parsePrimary(depth)
}

func (p *expressionParser) parsePrimary(depth uint32) (*expressionNode, error) {
	if err := p.checkDepth(depth); err != nil {
		return nil, err
	}
	token := p.take()
	switch token.kind {
	case tokenNull, tokenBool, tokenInteger, tokenString, tokenComponentReference:
		return p.leaf(nodeLiteral, token)
	case tokenPath:
		node, err := p.leaf(nodePath, token)
		if err == nil {
			node.path = token.text
		}
		return node, err
	case tokenLeftParen:
		node, err := p.parseSelect(depth + 1)
		if err != nil {
			return nil, err
		}
		closing, err := p.expect(tokenRightParen, "opening parenthesis requires )")
		if err != nil {
			return nil, err
		}
		node.start, node.end = token.start, closing.end
		return node, nil
	case tokenEOF:
		return nil, expressionError(token.start, token.end, "expected expression", ErrInvalid)
	default:
		return nil, expressionError(token.start, token.end, fmt.Sprintf("unexpected token %q", token.text), ErrInvalid)
	}
}

func (p *expressionParser) leaf(kind expressionNodeKind, token expressionToken) (*expressionNode, error) {
	p.nodes++
	if p.nodes > p.limits.MaxNodes {
		return nil, expressionError(token.start, token.end, "node count exceeds MaxNodes", ErrLimit)
	}
	return &expressionNode{kind: kind, start: token.start, end: token.end, value: token.value, height: 1}, nil
}

func (p *expressionParser) node(kind expressionNodeKind, start, end, op uint32, left, right, alt *expressionNode) (*expressionNode, error) {
	p.nodes++
	if p.nodes > p.limits.MaxNodes {
		return nil, expressionError(start, end, "node count exceeds MaxNodes", ErrLimit)
	}
	height := uint32(1)
	for _, child := range []*expressionNode{left, right, alt} {
		if child != nil && child.height+1 > height {
			height = child.height + 1
		}
	}
	if height > p.limits.MaxDepth {
		return nil, expressionError(start, end, "expression depth exceeds MaxDepth", ErrLimit)
	}
	if height > p.peakDepth {
		p.peakDepth = height
	}
	return &expressionNode{kind: kind, start: start, end: end, opOffset: op, left: left, right: right, alt: alt, height: height}, nil
}

func (p *expressionParser) checkDepth(depth uint32) error {
	if depth > p.limits.MaxDepth {
		token := p.peek()
		return expressionError(token.start, token.end, "parse nesting exceeds MaxDepth", ErrLimit)
	}
	if depth > p.peakDepth {
		p.peakDepth = depth
	}
	return nil
}

func (p *expressionParser) expect(kind tokenKind, problem string) (expressionToken, error) {
	token := p.peek()
	if token.kind != kind {
		return expressionToken{}, expressionError(token.start, token.end, problem, ErrInvalid)
	}
	p.index++
	return token, nil
}

func (p *expressionParser) peek() expressionToken { return p.tokens[p.index] }

func (p *expressionParser) take() expressionToken {
	token := p.tokens[p.index]
	if token.kind != tokenEOF {
		p.index++
	}
	return token
}

type expressionCompiler struct {
	pathKinds     map[string]PathKind
	limits        Limits
	constants     []Value
	paths         []string
	constantIndex map[Value]uint32
	pathIndex     map[string]uint32
}

func (c *expressionCompiler) check(node *expressionNode) (Kind, error) {
	return c.checkWithNarrowing(node, nil)
}

func (c *expressionCompiler) checkWithNarrowing(node *expressionNode, narrowed map[string]bool) (Kind, error) {
	switch node.kind {
	case nodeLiteral:
		if node.value.Kind == String && uint32(len(node.value.String)) > c.limits.MaxStringBytes { // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
			return Null, expressionError(node.start, node.end, "string literal exceeds MaxStringBytes", ErrLimit)
		}
		node.inferred = node.value.Kind
		node.nullable = node.value.Kind == Null
	case nodePath:
		contract, exists := c.pathKinds[node.path]
		if !exists {
			return Null, expressionError(node.start, node.end, fmt.Sprintf("binding path %q is not declared", node.path), ErrBinding)
		}
		node.inferred = contract.Kind
		node.nullable = contract.Optional && !narrowed[node.path]
	case nodeNot:
		kind, err := c.checkWithNarrowing(node.left, narrowed)
		if err != nil {
			return Null, err
		}
		if kind != Bool || node.left.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+1, "! requires bool", ErrType)
		}
		node.inferred = Bool
	case nodeNegate:
		kind, err := c.checkWithNarrowing(node.left, narrowed)
		if err != nil {
			return Null, err
		}
		if kind != Integer && kind != Unit || node.left.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+1, "unary - requires a non-null number or unit", ErrType)
		}
		node.inferred = kind
	case nodeEqual, nodeNotEqual:
		left, right, err := c.checkPair(node, narrowed)
		if err != nil {
			return Null, err
		}
		if left != right && left != Null && right != Null {
			return Null, expressionError(node.opOffset, node.opOffset+2, "== operands must have the same static kind", ErrType)
		}
		if left == Unit && right == Unit && literalUnitsIncompatible(node.left, node.right) {
			return Null, expressionError(node.opOffset, node.opOffset+2, "unit equality requires compatible units", ErrType)
		}
		node.inferred = Bool
	case nodeLess, nodeLessEqual, nodeGreater, nodeGreaterEqual:
		left, right, err := c.checkPair(node, narrowed)
		if err != nil {
			return Null, err
		}
		if left != right || left != Integer && left != String && left != Unit || node.left.nullable || node.right.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+1, "ordering requires two non-null numbers, strings, or compatible units", ErrType)
		}
		if left == Unit && literalUnitsIncompatible(node.left, node.right) {
			return Null, expressionError(node.opOffset, node.opOffset+1, "unit ordering requires matching units", ErrType)
		}
		node.inferred = Bool
	case nodeMatches:
		left, right, err := c.checkPair(node, narrowed)
		if err != nil {
			return Null, err
		}
		if left != String || right != String || node.left.nullable || node.right.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+7, "matches requires two strings", ErrType)
		}
		if node.right.kind == nodeLiteral {
			if uint32(len(node.right.value.String)) > c.limits.MaxPatternBytes { // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
				return Null, expressionError(node.right.start, node.right.end, "match pattern exceeds MaxPatternBytes", ErrLimit)
			}
			work := uint64(0)
			if _, err := wildcardTokens(context.Background(), node.right.value.String, c.limits, &work); err != nil {
				return Null, expressionError(node.right.start, node.right.end, err.Error(), err)
			}
		}
		node.inferred = Bool
	case nodeAnd, nodeOr:
		left, err := c.checkWithNarrowing(node.left, narrowed)
		if err != nil {
			return Null, err
		}
		rightNarrowed := cloneNarrowing(narrowed)
		truth := node.kind == nodeAnd
		applyNarrowing(rightNarrowed, node.left, truth)
		right, err := c.checkWithNarrowing(node.right, rightNarrowed)
		if err != nil {
			return Null, err
		}
		if left != Bool || right != Bool || node.left.nullable || node.right.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+2, "boolean operator requires bool operands", ErrType)
		}
		node.inferred = Bool
	case nodePlus:
		left, right, err := c.checkPair(node, narrowed)
		if err != nil {
			return Null, err
		}
		if left != right || left != Integer && left != String && left != Unit || node.left.nullable || node.right.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+1, "+ requires two integers or two strings, or two matching units; operands must be non-null", ErrType)
		}
		if left == Unit && literalUnitsIncompatible(node.left, node.right) {
			return Null, expressionError(node.opOffset, node.opOffset+1, "unit addition requires matching units", ErrType)
		}
		node.inferred = left
	case nodeMinus, nodeMultiply, nodeDivide:
		left, right, err := c.checkPair(node, narrowed)
		if err != nil {
			return Null, err
		}
		if node.left.nullable || node.right.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+1, "arithmetic requires non-null operands", ErrType)
		}
		switch node.kind {
		case nodeMinus:
			if left != right || left != Integer && left != Unit || left == Unit && literalUnitsIncompatible(node.left, node.right) {
				return Null, expressionError(node.opOffset, node.opOffset+1, "subtraction requires two numbers or matching units", ErrType)
			}
			node.inferred = left
		case nodeMultiply:
			if left != Integer && left != Unit || right != Integer && right != Unit || left == Unit && right == Unit {
				return Null, expressionError(node.opOffset, node.opOffset+1, "multiplication supports number*number or number*unit", ErrType)
			}
			if left == Unit || right == Unit {
				node.inferred = Unit
			} else {
				node.inferred = Integer
			}
		case nodeDivide:
			if right != Integer || left != Integer && left != Unit {
				return Null, expressionError(node.opOffset, node.opOffset+1, "division supports number/number or unit/number", ErrType)
			}
			node.inferred = left
		}
	case nodeSelect:
		condition, err := c.checkWithNarrowing(node.left, narrowed)
		if err != nil {
			return Null, err
		}
		trueNarrowed := cloneNarrowing(narrowed)
		applyNarrowing(trueNarrowed, node.left, true)
		whenTrue, err := c.checkWithNarrowing(node.right, trueNarrowed)
		if err != nil {
			return Null, err
		}
		falseNarrowed := cloneNarrowing(narrowed)
		applyNarrowing(falseNarrowed, node.left, false)
		whenFalse, err := c.checkWithNarrowing(node.alt, falseNarrowed)
		if err != nil {
			return Null, err
		}
		if condition != Bool || node.left.nullable {
			return Null, expressionError(node.opOffset, node.opOffset+1, "conditional condition must be bool", ErrType)
		}
		if whenTrue != whenFalse && whenTrue != Null && whenFalse != Null {
			return Null, expressionError(node.opOffset, node.opOffset+1, "conditional branches must have the same static kind", ErrType)
		}
		node.inferred = whenTrue
		if node.inferred == Null {
			node.inferred = whenFalse
		}
		node.nullable = node.right.nullable || node.alt.nullable || whenTrue == Null || whenFalse == Null
	default:
		return Null, expressionError(node.start, node.end, "unknown expression node", ErrInvalid)
	}
	return node.inferred, nil
}

func (c *expressionCompiler) checkPair(node *expressionNode, narrowed map[string]bool) (Kind, Kind, error) {
	left, err := c.checkWithNarrowing(node.left, narrowed)
	if err != nil {
		return Null, Null, err
	}
	right, err := c.checkWithNarrowing(node.right, narrowed)
	return left, right, err
}

func cloneNarrowing(input map[string]bool) map[string]bool {
	result := make(map[string]bool, len(input)+1)
	for path, value := range input {
		result[path] = value
	}
	return result
}

// applyNarrowing records facts proven by the truth or falsity of a guard. It
// deliberately recognizes only sound null checks and their boolean
// composition; arbitrary predicates never narrow a schema path.
func applyNarrowing(result map[string]bool, node *expressionNode, truth bool) {
	if node == nil {
		return
	}
	if node.kind == nodeNot {
		applyNarrowing(result, node.left, !truth)
		return
	}
	if node.kind == nodeAnd && truth {
		applyNarrowing(result, node.left, true)
		applyNarrowing(result, node.right, true)
		return
	}
	if node.kind == nodeOr && !truth {
		applyNarrowing(result, node.left, false)
		applyNarrowing(result, node.right, false)
		return
	}
	if node.kind != nodeEqual && node.kind != nodeNotEqual {
		return
	}
	path := ""
	if node.left.kind == nodePath && node.right.kind == nodeLiteral && node.right.value.Kind == Null {
		path = node.left.path
	}
	if node.right.kind == nodePath && node.left.kind == nodeLiteral && node.left.value.Kind == Null {
		path = node.right.path
	}
	if path == "" {
		return
	}
	nonNull := truth == (node.kind == nodeNotEqual)
	if nonNull {
		result[path] = true
	}
}

func literalUnitsIncompatible(left, right *expressionNode) bool {
	return left.kind == nodeLiteral && right.kind == nodeLiteral && left.value.Kind == Unit && right.value.Kind == Unit && !unitsCompatible(left.value.Unit, right.value.Unit)
}

func (c *expressionCompiler) indexInputs(root *expressionNode) {
	constantSet := make(map[Value]bool)
	pathSet := make(map[string]bool)
	var walk func(*expressionNode)
	walk = func(node *expressionNode) {
		if node == nil {
			return
		}
		switch node.kind {
		case nodeLiteral:
			constantSet[node.value] = true
		case nodePath:
			pathSet[node.path] = true
		}
		walk(node.left)
		walk(node.right)
		walk(node.alt)
	}
	walk(root)
	for value := range constantSet {
		c.constants = append(c.constants, value)
	}
	sort.Slice(c.constants, func(i, j int) bool { return lessExpressionValue(c.constants[i], c.constants[j]) })
	c.constantIndex = make(map[Value]uint32, len(c.constants))
	for index, value := range c.constants {
		c.constantIndex[value] = uint32(index)
	}
	for path := range pathSet {
		c.paths = append(c.paths, path)
	}
	sort.Strings(c.paths)
	c.pathIndex = make(map[string]uint32, len(c.paths))
	for index, path := range c.paths {
		c.pathIndex[path] = uint32(index)
	}
}

func (c *expressionCompiler) emit(node *expressionNode, code *[]Instruction) {
	switch node.kind {
	case nodeLiteral:
		*code = append(*code, Instruction{Op: OpConstant, Arg: c.constantIndex[node.value]})
	case nodePath:
		*code = append(*code, Instruction{Op: OpLoad, Arg: c.pathIndex[node.path]})
	case nodeNot, nodeNegate:
		c.emit(node.left, code)
		op := OpNot
		if node.kind == nodeNegate {
			op = OpNegateInteger
		}
		*code = append(*code, Instruction{Op: op})
	case nodeEqual, nodeNotEqual, nodeLess, nodeLessEqual, nodeGreater, nodeGreaterEqual, nodeMatches, nodeAnd, nodeOr, nodePlus, nodeMinus, nodeMultiply, nodeDivide:
		c.emit(node.left, code)
		c.emit(node.right, code)
		op := OpEqual
		switch node.kind {
		case nodeNotEqual:
			op = OpNotEqual
		case nodeLess:
			op = OpLess
		case nodeLessEqual:
			op = OpLessEqual
		case nodeGreater:
			op = OpGreater
		case nodeGreaterEqual:
			op = OpGreaterEqual
		case nodeAnd:
			op = OpAnd
		case nodeMatches:
			op = OpMatches
		case nodeOr:
			op = OpOr
		case nodePlus:
			if node.inferred == String {
				op = OpConcat
			} else {
				op = OpAddInteger
			}
		case nodeMinus:
			op = OpSubInteger
		case nodeMultiply:
			op = OpMultiplyInteger
		case nodeDivide:
			op = OpDivideInteger
		}
		*code = append(*code, Instruction{Op: op})
	case nodeSelect:
		c.emit(node.left, code)
		c.emit(node.right, code)
		c.emit(node.alt, code)
		*code = append(*code, Instruction{Op: OpSelect})
	}
}

func lessExpressionValue(left, right Value) bool {
	if left.Kind != right.Kind {
		return left.Kind < right.Kind
	}
	switch left.Kind {
	case Bool:
		return !left.Bool && right.Bool
	case Integer:
		if left.Integer != right.Integer {
			return left.Integer < right.Integer
		}
		return left.Scale < right.Scale
	case String:
		if left.Reference != right.Reference {
			return !left.Reference && right.Reference
		}
		return left.String < right.String
	case Unit:
		if left.Unit != right.Unit {
			return left.Unit < right.Unit
		}
		if left.Integer != right.Integer {
			return left.Integer < right.Integer
		}
		return left.Scale < right.Scale
	default:
		return false
	}
}

func normalizePathKinds(environment []PathKind, limits Limits) (map[string]PathKind, error) {
	result := make(map[string]PathKind, len(environment))
	for _, entry := range environment {
		if !validPath(entry.Path) || uint32(len(entry.Path)) > limits.MaxStringBytes { // #nosec G115 -- collection length is bounded by the surrounding limit or container invariant
			return nil, expressionError(0, 0, fmt.Sprintf("invalid environment path %q", entry.Path), ErrBinding)
		}
		if entry.Kind > Unit || entry.Kind == Null {
			return nil, expressionError(0, 0, fmt.Sprintf("invalid kind for %q", entry.Path), ErrType)
		}
		if _, duplicate := result[entry.Path]; duplicate {
			return nil, expressionError(0, 0, fmt.Sprintf("duplicate environment path %q", entry.Path), ErrBinding)
		}
		result[entry.Path] = entry
	}
	return result, nil
}

func normalizeLanguageLimits(limits LanguageLimits) (LanguageLimits, error) {
	if limits == (LanguageLimits{}) {
		return DefaultLanguageLimits(), nil
	}
	hard := DefaultLanguageLimits()
	if limits.MaxSourceBytes == 0 || limits.MaxSourceBytes > hard.MaxSourceBytes ||
		limits.MaxTokens == 0 || limits.MaxTokens > hard.MaxTokens ||
		limits.MaxDepth == 0 || limits.MaxDepth > hard.MaxDepth ||
		limits.MaxNodes == 0 || limits.MaxNodes > hard.MaxNodes {
		return LanguageLimits{}, ErrLimit
	}
	program, err := normalizeLimits(limits.Program)
	if err != nil {
		return LanguageLimits{}, err
	}
	limits.Program = program
	return limits, nil
}

func expressionError(start, end uint32, problem string, cause error) error {
	return &ExpressionError{Offset: start, End: end, Problem: problem, Cause: cause}
}

func boundedSourceOffset(offset int) uint32 {
	if uint64(offset) > uint64(^uint32(0)) { // #nosec G115 -- source offset is bounded by validated input or parser state
		return ^uint32(0)
	}
	return uint32(offset) // #nosec G115 -- source offset is bounded by validated input or parser state
}
