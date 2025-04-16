package interpreter

import (
	"fmt"
	"strings"

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
	UseCopy  bool
}
type SplitNode struct {
	*ffmpeg.Node
	StreamType
}

type streamStore struct {
	splitNodes      map[parser.NodeIdent]*SplitNode
	originalStreams map[parser.NodeIdent]*Stream
	splitCounts     map[parser.NodeIdent]int
}

func newStreamStore() streamStore {
	return streamStore{
		splitNodes:      make(map[parser.NodeIdent]*SplitNode),
		originalStreams: make(map[parser.NodeIdent]*Stream),
		splitCounts:     make(map[parser.NodeIdent]int),
	}
}

func (s streamStore) get(name parser.NodeIdent) (*Stream, error) {
	stream, ok := s.originalStreams[name]
	if ok {
		if !stream.UseCopy {
			delete(s.originalStreams, name)
		}
		return stream, nil
	}
	fnode, ok := s.splitNodes[name]
	if !ok {
		return nil, fmt.Errorf("stream variable %s not defined", name)
	}
	stream = &Stream{
		FFStream: fnode.Get(fmt.Sprintf("%v", s.splitCounts)),
		UseCopy:  false,
		Type:     fnode.StreamType,
	}
	s.splitCounts[name] = s.splitCounts[name] + 1
	return stream, nil
}

func (s streamStore) set(name parser.NodeIdent, stream *Stream) {
	if stream.UseCopy {
		s.originalStreams[name] = stream
		return
	}
	s.splitNodes[name] = &SplitNode{stream.FFStream.Split(), stream.Type}
	s.splitCounts[name] = 0
}

// Context holds the running state of the interpreter
type Context struct {
	variables    map[parser.NodeIdent]ValueBox
	streams      streamStore
	scopeGStream *Stream
	debug        bool
}

// NewContext creates a new interpreter context
func NewContext(debug bool) *Context {
	return &Context{
		variables:    make(map[parser.NodeIdent]ValueBox),
		streams:      newStreamStore(),
		scopeGStream: nil,
		debug:        debug,
	}
}

func (c *Context) getStream(name parser.NodeIdent) (*Stream, error) {
	return c.streams.get(name)
}

func (c *Context) setStream(name parser.NodeIdent, stream *Stream) {
	c.streams.set(name, stream)
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

func literalToBox(node parser.Node) ValueBox {
	var box ValueBox
	switch v := node.(type) {
	case parser.NodeLiteralBool:
		box = ValueBox{v, ValueBool}
	case parser.NodeLiteralNumber:
		box = ValueBox{v, ValueNumber}
	case parser.NodeLiteralString:
		box = ValueBox{v, ValueString}
	}
	return box
}

func boxToPrimitive(v ValueBox) any {
	switch v.typ {
	case ValueBool:
		return bool(v.any.(parser.NodeLiteralBool))
	case ValueNumber:
		return float64(v.any.(parser.NodeLiteralNumber))
	case ValueString:
		return strings.Trim(v.any.(parser.NodeLiteralString).String(), "\"")
	}
	return nil
}

func (c *Context) setLiteral(name parser.NodeIdent, node parser.Node) {
	c.variables[name] = literalToBox(node)
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

	var stream *Stream
	var err error
	if value, ok := node.Value.(parser.NodeExpr); ok {
		stream, err = evaluateExpression(ctx, value)
		if err != nil {
			return err
		}
		ctx.setStream(node.Dest[0], stream)
	}

	if len(node.Dest) > 1 {
		return fmt.Errorf("cannot assign stream to multiple variables for now")
	}

	if stream == nil {
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
		input := expr.Input[0]
		v, ok := input.(parser.NodeIdent)
		if !ok {
			return nil, fmt.Errorf("invalid input type: %T", input)
		}
		if stream, err = ctx.getStream(v); err != nil {
			return nil, err
		}
	}

	for _, cmd := range expr.Pipeline {
		var err error
		stream, err = evaluateCommand(ctx, cmd, stream)
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
