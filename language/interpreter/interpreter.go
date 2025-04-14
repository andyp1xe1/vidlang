package interpreter

import (
	"fmt"

	"github.com/andyp1xe1/vidlang/language/parser"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// StreamType represents the type of media stream
type StreamType int

const (
	VideoStream StreamType = iota
	AudioStream
	MultiStream
)

type valueType int

const (
	ValueStream valueType = iota
	ValueBool
	ValueNumber
	ValueString
	ValueList
	ValueSubExpr
)

type ValueBox struct {
	any
	typ valueType
}

// Stream represents a media stream in our DSL
type Stream struct {
	FFStream *ffmpeg.Stream
	Type     StreamType
}

// Context holds the running state of the interpreter
type Context struct {
	variables    map[parser.NodeIdent]ValueBox
	scopeGStream *Stream
	debug        bool
}

// NewContext creates a new interpreter context
func NewContext(debug bool) *Context {
	return &Context{
		variables:    make(map[parser.NodeIdent]ValueBox),
		scopeGStream: nil,
		debug:        debug,
	}
}

func (c *Context) getVar(name parser.NodeIdent) (ValueBox, error) {
	if name == "stream" {
		if c.scopeGStream == nil {
			return ValueBox{}, fmt.Errorf("global stream is not set")
		}
		return ValueBox{c.scopeGStream, ValueStream}, nil
	}

	val, ok := c.variables[name]
	if !ok {
		return ValueBox{}, fmt.Errorf("variable %s not found", name)
	}
	return val, nil
}

func (c *Context) setVar(name parser.NodeIdent, typ valueType, v any) {
	c.variables[name] = ValueBox{v, typ}
}

func (c *Context) setLiteral(name parser.NodeIdent, node parser.Node) {
	var box ValueBox
	switch v := node.(type) {
	case parser.NodeLiteralBool:
		box = ValueBox{v, ValueBool}
	case parser.NodeLiteralNumber:
		box = ValueBox{v, ValueNumber}
	case parser.NodeLiteralString:
		box = ValueBox{v, ValueString}
	case parser.NodeSubExpr:
		box = ValueBox{v, ValueSubExpr}
	}
	c.variables[name] = box
}

func (c *Context) setBox(name parser.NodeIdent, box ValueBox) {
	c.variables[name] = box
}

type Interpreter struct {
	ctx    *Context
	parser *parser.Parser
}

func Interpret(code string, debug bool) error {
	parser := parser.Parse(code)

	i := &Interpreter{
		parser: parser,
		ctx:    NewContext(debug),
	}

	return i.run()
}

// Execute runs the interpreter on the given AST nodes
func (i *Interpreter) run() error {
	for node := range i.parser.Expressions {
		if err := evaluate(i.ctx, node); err != nil {
			return err
		}
	}
	return nil
}

// Evaluate evaluates a parsed AST node
func evaluate(ctx *Context, node parser.Node) error {
	switch n := node.(type) {
	case parser.NodeAssign:
		return evaluateAssignment(ctx, n)
	case parser.NodeExpr:
		stream, err := evaluateExpression(ctx, n)
		if err != nil {
			return err
		}
		ctx.scopeGStream = stream
		return nil
	default:
		return fmt.Errorf("unsupported node type: %T", node)
	}
}

// evaluateAssignment evaluates an assignment node
func evaluateAssignment(ctx *Context, node parser.NodeAssign) error {
	if len(node.Dest) == 0 {
		return fmt.Errorf("invalid assignment: no destination")
	}

	var val ValueBox

	if value, ok := node.Value.(parser.NodeExpr); ok {
		res, err := evaluateExpression(ctx, value)
		if err != nil {
			return err
		}
		val = ValueBox{res, ValueStream}
	}

	if len(node.Dest) > 1 {
		return fmt.Errorf("cannot assign stream to multiple variables for now")
	}

	if val.any != nil {
		ctx.setBox(node.Dest[0], val)
	} else {
		ctx.setLiteral(node.Dest[0], node.Value)
	}

	return nil
}

// evaluateExpression evaluates an expression node
func evaluateExpression(ctx *Context, expr parser.NodeExpr) (*Stream, error) {
	var stream *Stream
	var err error

	if len(expr.Input) > 1 {
		return nil, fmt.Errorf("multiple inputs not supported yet")
	}

	if len(expr.Input) == 1 {
		var val ValueBox
		input := expr.Input[0]
		v, ok := input.(parser.NodeIdent)
		if !ok {
			return nil, fmt.Errorf("invalid input type: %T", input)
		}
		if val, err = ctx.getVar(v); err != nil {
			return nil, err
		} else if val.typ != ValueStream {
			return nil, fmt.Errorf("variable %s is not a stream", v)
		}

		stream = val.any.(*Stream)
	}

	for _, cmd := range expr.Pipeline {
		var err error
		stream, err = evaluateCommand(ctx, cmd, nil)
		if err != nil {
			return nil, err
		}
	}

	return stream, nil
}

// evaluateCommand evaluates a command node
func evaluateCommand(ctx *Context, cmd parser.NodeCommand, input *Stream) (*Stream, error) {
	if handler, ok := handlerMap[cmd.Name]; ok {
		return handler(ctx, input, cmd.Args)
	}
	return nil, fmt.Errorf("unknown command: %s", cmd.Name)
}
