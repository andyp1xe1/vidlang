package main

import "fmt"

// an attempt to make the nodes types expressed through interfaces,
// could delete all later in favour of simple structs

type node interface {
	String() string
}

type nodeCommand struct {
	name string
	args []node
}

func (c *nodeCommand) String() string {
	return fmt.Sprintf("%s, args: %v", c.name, c.args)
}

type nodePipeline struct {
	cmds []*nodeCommand
}

type exprType interface {
	nodeCommand | nodePipeline
}

type exprInputType interface {
	nodeIdent | nodeList
}

type nodeExpr[T exprInputType, V exprType] struct {
	input V
	expr  T
}

type assignIdentType interface {
	nodeIdent | nodeIdents
}

type nodeAssign[S assignIdentType, T exprInputType, V exprType] struct {
	left   S
	right  nodeExpr[T, V]
	define bool
}

type nodeIdent struct {
	item
}

type nodeIdents struct {
	items []nodeIdent
}

type nodeList struct {
	items []nodeIdent
}

type Parser struct {
	lex      *lexer
	root     node
	currItem item
	peekItem item
}

func parse(input string) Parser {
	parser := Parser{
		lex:  lex(input),
		root: nil,
	}
	return parser
}

// nextItem advances the parser to the next token, and sets the peekItem
func (p *Parser) nextItem() {
	p.currItem = p.peekItem
	item := <-p.lex.items
	if item.typ != itemError {
		p.peekItem = item
		return
	}

	fmt.Errorf(item.val)
	panic(item.val)
}

func (p *Parser) errorf(format string, args ...any) {
	err := fmt.Errorf(format, args...)
	fmt.Println(err)
	panic(err)
}

// the main entry point
func (p *Parser) parse() {
	var expressions []node
	for {
		var currNode node
		p.nextItem()
		switch p.currItem.typ {
		case itemEOF:
			return
		case itemNewline:
			continue
		case itemIdentifier:
			// currNode = p.parseIdents()
			// Will parse the itentifiers separated by commas
			// and the assignable expression

		}
		expressions = append(expressions, currNode)
	}
}

// func (p *Parser) parseIdents() node {
// 	idents := nodeIdents{make([]nodeIdent, 0)}
// 	idents.items = append(idents.items, nodeIdent{p.currItem})
// 	p.nextItem()
// 	for p.currItem.typ != itemComma {
// 		if p.peekItem.typ != itemCommand && p.peekItem.typ != itemIdentifier {
// 			p.errorf("expected command or identifier")
// 		}
// 		idents.items = append(idents.items, nodeIdent{p.currItem})
// 	}
// }
