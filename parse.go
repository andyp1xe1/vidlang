package main

import (
	"fmt"
	"strconv"
)

type valueType int

const (
	valueLiteralBool valueType = iota
	valueLiteralNumber
	valueLiteralString

	valueGlobalStream
	valueIdentifier
	valueSelfStar

	valueExpr
	valueList
	valueSubExpr
)

type nodeValue interface {
	valueType() valueType
	String() string
}

type nodeLiteralString string

func (s nodeLiteralString) valueType() valueType { return valueLiteralString }
func (s nodeLiteralString) String() string       { return string(s) }

type nodeLiteralNumber float64

func (n nodeLiteralNumber) valueType() valueType { return valueLiteralNumber }
func (n nodeLiteralNumber) String() string       { return fmt.Sprintf("%f", n) }

type nodeLiteralBool bool

func (b nodeLiteralBool) valueType() valueType { return valueLiteralBool }
func (b nodeLiteralBool) String() string       { return fmt.Sprintf("%t", b) }

type nodeIdent string

func (n nodeIdent) valueType() valueType { return valueIdentifier }
func (n nodeIdent) String() string       { return string(n) }

type nodeSelfStar struct{ self string }

func (s nodeSelfStar) valueType() valueType { return valueSelfStar }
func (s nodeSelfStar) String() string       { return s.self }

type nodeSubExpr struct {
	body   nodeValue
	params nodeList[nodeIdent]
}

func (s nodeSubExpr) valueType() valueType { return valueSubExpr }
func (s nodeSubExpr) String() string {
	return fmt.Sprintf("%v -> %v", s.params, s.body)
}

type nodeExprMath struct {
	left  nodeValue
	op    itemType
	right nodeValue
}

func (n nodeExprMath) valueType() valueType { return valueExpr }
func (n nodeExprMath) String() string {
	return fmt.Sprintf("(%v %s %v)", n.left, n.op, n.right)
}

type node interface{}

type nodeList[T node] []T

func (n nodeList[T]) valueType() valueType { return valueList }
func (n nodeList[T]) String() string {
	return fmt.Sprintf("%v[%v]", []T(n), len(n))
}

type nodeCommand struct {
	name string
	args []nodeValue
}

type nodePipeline []nodeCommand

type nodeExpr struct {
	input    nodeList[nodeValue]
	pipeline nodePipeline
}

func (n nodeExpr) valueType() valueType { return valueExpr }
func (n nodeExpr) String() string {
	return fmt.Sprintf("%v", n.input) + " |> " + fmt.Sprintf("%v", n.pipeline)
}

func (n nodeExpr) inferType() valueType {
	return valueLiteralString // TODO
}

type nodeAssign struct {
	dest   nodeList[nodeIdent]
	value  nodeValue // some simple value, expr or subexpr
	define bool
}

func printNode(n node) {
	fmt.Printf("%#v\n", n)
}

type identifier struct {
	name string
	typ  string
}

type parser struct {
	lex         *lexer
	expressions chan node
	currItem    item
	peekItem    item
	peek2Item   item
	identSet    map[string]*identifier
}

func parse(input string) *parser {
	p := &parser{
		lex:         lex(input),
		expressions: make(chan node),
		identSet:    make(map[string]*identifier),
	}
	p.currItem = <-p.lex.items
	p.peekItem = <-p.lex.items
	p.peek2Item = <-p.lex.items
	go p.run()
	return p
}

// nextItem advances the parser to the next token, and sets the peekItem
func (p *parser) nextItem() {
	if p.currItem.typ == itemError {
		p.errorf("lexical error: %s", p.currItem.val)
	}
	p.currItem = p.peekItem
	p.peekItem = p.peek2Item
	p.peek2Item = <-p.lex.items

}

func (p *parser) errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	panic(err)
}

func (p *parser) run() {

	for i := 0; ; i++ {
		p.nextItem()
		switch p.currItem.typ {
		case itemEOF:
			close(p.expressions)
			return
		case itemIdentifier:
			// Check for assignment-like conditions
			switch p.peekItem.typ {
			case itemAssign, itemDeclare, itemComma:
				p.expressions <- p.parseAssignment()
			case itemPipe:
				p.expressions <- p.parseAssignable()
			}
		case itemLeftBrace, itemNumber, itemString, itemBool:
			p.expressions <- p.parseAssignable()
		default:
			if p.currItem.typ > itemCommand {
				p.expressions <- p.parseAssignable()
			} else {
			}
		}
	}
}

var validArgs = map[itemType]bool{
	itemLeftBrace:  true,
	itemIdentifier: true,
	itemNumber:     true,
	itemString:     true,
	itemBool:       true,
}

func (p *parser) parseCommand() nodeCommand {
	var node nodeCommand
	node.name = p.currItem.val
	node.args = make([]nodeValue, 0)
	for validArgs[p.peekItem.typ] && p.peekItem.typ != itemNewline {
		p.nextItem()
		assert(
			validArgs[p.currItem.typ],
			"parseCommand's loop should be entered with a valid arg, but got %s",
			p.currItem)
		assert(
			p.currItem.typ != itemNewline,
			"parseCommand's loop should not process newlines",
		)
		node.args = append(node.args, p.parseValue())
	}
	return node
}

var validValues = map[itemType]bool{
	itemLeftBrace:  true,
	itemIdentifier: true,
	itemNumber:     true,
	itemString:     true,
	itemBool:       true,
	itemSelfStar:   true,
	itemStream:     true,
}

// if it's a list, parse it
// else if it's a value,
// if it is a number and peek is an operation, parse math expression
// else parse the simple value
// if a subexpression continues, parse it and give it the parameter list
// if a pipeline continues, parse it, adding the list or value as input
// return the value
// else if it's a command, parse the pipeline, form an expression, return it
func (p *parser) parseValue() nodeValue {
	assert(validValues[p.currItem.typ],
		"parseValue should be invoked with currItem at a simple value, list or subexpression got %s",
		p.currItem)

	var n nodeValue

	if p.currItem.typ == itemLeftBrace {
		n = p.parseSimpleValueList()
		assert(
			p.currItem.typ == itemRightBrace, "assumed that the list was terminated successfully by a right brace, but got %s -> %s", p.currItem, p.peekItem)
		if p.peekItem.typ == itemLeftParen {
			n = p.parseSubExpr(n)
		}
	} else if (p.currItem.typ == itemNumber || p.currItem.typ == itemLeftParen) && p.peekItem.typ == mathSymbols[p.peekItem.val] {
		n = p.parseMathExpression()
	} else {
		n = p.parseSimpleValue()
	}

	return n
}

func (p *parser) parseAssignable() nodeValue {
	assert(validValues[p.currItem.typ] || p.currItem.typ > itemCommand,
		"parseValue should be invoked with currItem at a simple value, list, subexpression or command, got %s",
		p.currItem)

	var n nodeValue

	if validValues[p.currItem.typ] {
		n = p.parseValue()

		if p.peekItem.typ == itemNewline && p.peek2Item.typ == itemPipe {
			p.nextItem()
		}

		if p.peekItem.typ == itemPipe { // TODO:
			p.nextItem()
		}

		if p.currItem.typ == itemPipe {
			p.nextItem()
			if n.valueType() != valueList {
				n = nodeList[nodeValue]{n}
			}
			n = nodeExpr{input: n.(nodeList[nodeValue]), pipeline: p.parsePipeline()}
		}
	} else if p.currItem.typ > itemCommand {
		n = nodeExpr{pipeline: p.parsePipeline(), input: nil}
	}

	return n
}

// TODO hangle empty body case `()`
func (p *parser) parseSubExprBody() nodeValue {
	assert(p.currItem.typ == itemLeftParen,
		"parseSubExprBody should be invoked with currItem at left paren (the beginning of its body), got %s",
		p.currItem)
	p.nextItem()

	n := p.parseAssignable()
	// TODO: This is a temporary fix! Very bad should be done something else
	if p.currItem.typ != itemRightParen {
		p.nextItem()
	}

	if p.currItem.typ != itemRightParen {
		p.errorf("expected right paren at the end of subexpression body, got %s -> %s", p.currItem, p.peekItem)
	}
	//p.nextItem()

	return n
}

func (p *parser) parseSubExpr(v nodeValue) nodeSubExpr {

	assert(v.valueType() == valueList,
		"parseSubExpr's argument is assumed to be a list, but got %s", p.currItem)

	argList := make(nodeList[nodeIdent], 0)
	for _, arg := range v.(nodeList[nodeValue]) {
		if arg.valueType() != valueIdentifier {
			p.errorf("a subexpression's argument list must be a list of identifiers, but got %s", arg)
		}
		argList = append(argList, arg.(nodeIdent))
	}

	var n nodeSubExpr
	n.params = argList

	p.nextItem()
	n.body = p.parseSubExprBody()

	return n
}

func (p *parser) parsePipeline() nodePipeline {
	node := make(nodePipeline, 0)

	assert(p.currItem.typ > itemCommand,
		"parsePipeline should be invoked with currItem at a command, got %s", p.currItem)

	for p.currItem.typ > itemCommand {

		node = append(node, p.parseCommand())
		if p.peekItem.typ == itemNewline && p.peek2Item.typ == itemPipe {
			p.nextItem()
		}
		if p.peekItem.typ != itemPipe {
			break
		}
		p.nextItem()
		if p.peekItem.typ < itemCommand {
			p.errorf("expected command after pipe, got %s", p.peekItem)
		}
		p.nextItem()
	}

	return node
}

func (p *parser) parseSimpleValueList() nodeList[nodeValue] {
	list := make(nodeList[nodeValue], 0)
	assert(p.currItem.typ == itemLeftBrace, "parseSimpleValueList should be invoked with currItem at left brace, got %s", p.currItem)
	for p.currItem.typ != itemRightBrace {
		p.nextItem()
		list = append(list, p.parseSimpleValue())
		p.nextItem()
		if p.currItem.typ != itemComma {
			if p.currItem.typ != itemRightBrace {
				p.errorf("list not terminated properly, expected comma or right brace, got %s", p.currItem)
			}
			break
		}
	}
	assert(
		p.currItem.typ == itemRightBrace,
		"it was assumed that the list was terminated by a right brace but got %s -> %s", p.currItem, p.peekItem)
	return list
}

func (p *parser) parseSimpleValue() nodeValue {
	assert(
		validValues[p.currItem.typ],
		"parseSimpleValue should be invoked with a valid value, but got %s", p.currItem,
	)
	var n nodeValue
	switch p.currItem.typ {
	case itemIdentifier, itemSelfStar, itemStream:
		n = nodeIdent(p.currItem.val)
	case itemNumber:
		n = nodeLiteralNumber(strToLiteralNumber(p.currItem.val))
	case itemBool:
		n = nodeLiteralBool(strToLiteralBool(p.currItem.val))
	case itemString:
		n = nodeLiteralString(p.currItem.val)
	default:
		p.errorf("parseSimpleValue should be invoked with a valid value, but got %s", p.currItem)
	}
	//p.nextItem()
	return n
}

func strToLiteralBool(s string) nodeLiteralBool { return s == "true" }
func strToLiteralNumber(s string) nodeLiteralNumber {
	n, err := strconv.ParseFloat(s, 64)
	assert(err == nil, "lexer mus provided a valid number, failed to parse number %s", s)
	return nodeLiteralNumber(n)
}

func (p *parser) parseAssignment() nodeAssign {
	var node nodeAssign

	node.dest = p.parseIdentList()

	if p.currItem.typ == itemDeclare {
		node.define = true
	} else if p.currItem.typ != itemAssign {
		p.errorf("expected assignment or declaration, got %s", p.peekItem)
	}

	p.nextItem()

	for p.currItem.typ == itemNewline {
		p.nextItem()
	}

	node.value = p.parseAssignable()

	return node
}

func (p *parser) parseIdentList() nodeList[nodeIdent] {
	var idents nodeList[nodeIdent]
	for p.currItem.typ == itemIdentifier {

		idents = append(idents, nodeIdent(p.currItem.val))
		p.nextItem()

		if p.currItem.typ == itemComma {
			p.nextItem()
		}

	}
	return idents
}

func (p *parser) parseMathExpression() nodeValue {
	return p.parseTerm()
}

func (p *parser) parseTerm() nodeValue {
	node := p.parseFactor()
	for p.currItem.typ == itemPlus || p.currItem.typ == itemMinus {
		op := p.currItem.typ
		p.nextItem()
		node = nodeExprMath{left: node, op: op, right: p.parseFactor()}
	}
	return node
}

func (p *parser) parseFactor() nodeValue {
	node := p.parsePrimary()
	for p.currItem.typ == itemMult || p.currItem.typ == itemDiv {
		op := p.currItem.typ
		p.nextItem()
		node = nodeExprMath{left: node, op: op, right: p.parsePrimary()}
	}
	return node
}

func (p *parser) parsePrimary() nodeValue {
	switch p.currItem.typ {
	case itemIdentifier:
		node := nodeIdent(p.currItem.val)
		p.nextItem()
		return node
	case itemNumber:
		node := strToLiteralNumber(p.currItem.val)
		p.nextItem()
		return node
	case itemLeftParen:
		p.nextItem()
		node := p.parseMathExpression()
		if p.currItem.typ != itemRightParen {
			p.errorf("missing closing parenthesis")
		}
		p.nextItem()
		return node
	default:
		p.errorf("unexpected token in expression %s", p.currItem)
		return nil
	}
}

func assert(condition bool, msg string, args ...any) {
	if !condition {
		panic(fmt.Errorf("assertion failed: "+msg, args...))
	}
}
