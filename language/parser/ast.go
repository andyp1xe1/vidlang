package parser

import "fmt"

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
