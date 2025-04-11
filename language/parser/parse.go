package parser

import (
	"fmt"
	"strconv"
)

type identifier struct {
	name string
	typ  string
}

type Parser struct {
	Expressions chan node

	lex       *lexer
	currItem  item
	peekItem  item
	peek2Item item
	identSet  map[string]*identifier // TODO: to keep track of identifiers
}

func Parse(input string) *Parser {
	p := &Parser{
		Expressions: make(chan node),

		lex:      lex(input),
		identSet: make(map[string]*identifier),
	}
	p.currItem = <-p.lex.items
	p.peekItem = <-p.lex.items
	p.peek2Item = <-p.lex.items
	go p.run()
	return p
}

// nextItem advances the Parser to the next token, and sets the peekItem
func (p *Parser) nextItem() {
	if p.currItem.typ == itemError {
		p.errorf("lexical error: %s", p.currItem.val)
	}
	p.currItem = p.peekItem
	p.peekItem = p.peek2Item
	p.peek2Item = <-p.lex.items

}

func (p *Parser) errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	panic(err)
}

func (p *Parser) run() {

	for i := 0; ; i++ {
		p.nextItem()
		switch p.currItem.typ {
		case itemEOF:
			close(p.Expressions)
			return
		case itemIdentifier:
			// Check for assignment-like conditions
			switch p.peekItem.typ {
			case itemAssign, itemDeclare, itemComma:
				p.Expressions <- p.parseAssignment()
			case itemPipe:
				p.Expressions <- p.parseAssignable()
			}
		case itemLeftBrace, itemNumber, itemString, itemBool:
			p.Expressions <- p.parseAssignable()
		default:
			if p.currItem.typ > itemCommand {
				p.Expressions <- p.parseAssignable()
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

func (p *Parser) parseCommand() nodeCommand {
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
func (p *Parser) parseValue() nodeValue {
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

func (p *Parser) parseAssignable() nodeValue {
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
func (p *Parser) parseSubExprBody() nodeValue {
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

func (p *Parser) parseSubExpr(v nodeValue) nodeSubExpr {

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

func (p *Parser) parsePipeline() nodePipeline {
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

// TODO: Rewrite this. it does not handle trailing commas, empty lists, and lists spanning multiple lines
func (p *Parser) parseSimpleValueList() nodeList[nodeValue] {
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

func (p *Parser) parseSimpleValue() nodeValue {
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

func (p *Parser) parseAssignment() nodeAssign {
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

func (p *Parser) parseIdentList() nodeList[nodeIdent] {
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

// TODO: Respect operator precedence
func (p *Parser) parseMathExpression() nodeValue {
	return p.parseTerm()
}

func (p *Parser) parseTerm() nodeValue {
	node := p.parseFactor()
	for p.currItem.typ == itemPlus || p.currItem.typ == itemMinus {
		op := p.currItem.typ
		p.nextItem()
		node = nodeExprMath{left: node, op: op, right: p.parseFactor()}
	}
	return node
}

func (p *Parser) parseFactor() nodeValue {
	node := p.parsePrimary()
	for p.currItem.typ == itemMult || p.currItem.typ == itemDiv {
		op := p.currItem.typ
		p.nextItem()
		node = nodeExprMath{left: node, op: op, right: p.parsePrimary()}
	}
	return node
}

func (p *Parser) parsePrimary() nodeValue {
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
