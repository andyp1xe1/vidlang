package interpreter

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/andyp1xe1/vidlang/language/parser"
)

// StreamType represents the type of media stream
type StreamType int

type valueType int

const (
	ValueBool = iota
	ValueNumber
	ValueString
	ValueList
	ValueSubExpr
)

type ValueBox struct {
	any
	typ valueType
}

// Context holds the running state of the interpreter
type Context struct {
	variables  map[parser.NodeIdent]ValueBox
	streams    streamStore
	debug      bool
	preview    bool
	previewCmd *exec.Cmd
}

// NewContext creates a new interpreter context
func NewContext(debug, preview bool) *Context {
	return &Context{
		variables: make(map[parser.NodeIdent]ValueBox),
		streams:   newStreamStore(),
		debug:     debug,
		preview:   preview,
	}
}

// StartPreviewPlayer launches ffplay to display the UDP stream
// func (c *Context) StartPreviewPlayer() error {
// 	// Kill any existing preview process
// 	if c.previewCmd != nil && c.previewCmd.Process != nil {
// 		c.previewCmd.Process.Kill()
// 	}
//
// 	// Launch ffplay to display the UDP stream
// 	c.previewCmd = exec.Command("setsid", "ffplay", "-fflags", "nobuffer", "-flags", "low_delay", "-framedrop", "-i", "udp://127.0.0.1:1234")
//
// 	// Run in background
// 	return c.previewCmd.Start()
// }

func (c *Context) StartPreviewPlayer() error {
	// check if ffplay is already running our specific stream
	cmd := exec.Command("pgrep", "-f", "ffplay.*udp://127.0.0.1:1234")
	if err := cmd.Run(); err == nil {
		return nil // already running
	}

	// not running, start it
	c.previewCmd = exec.Command("setsid", "ffplay", "-fflags", "nobuffer", "-flags", "low_delay", "-framedrop", "-i", "udp://127.0.0.1:1234")
	return c.previewCmd.Start()
}

func (c *Context) getVar(name parser.NodeIdent) (ValueBox, error) {
	if name == "stream" {
		return ValueBox{}, fmt.Errorf("global stream is not a box value")
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

func Interpret(code string, debug, preview bool) error {
	parser := parser.Parse(code, false)

	i := &Interpreter{
		parser: parser,
		ctx:    NewContext(debug, preview),
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
		entry, canCp, err := evaluateExpression(ctx, n) // TODO factor canCopy in Stream??
		if err != nil {
			return err
		}
		ctx.streams.set("stream", entry, canCp)
		return nil
	case parser.AstError:
		return fmt.Errorf("interpreter error: %v", n.Error())

	default:
		return fmt.Errorf("unsupported node type: %T", node)
	}
}

// evaluateAssignment evaluates an assignment node
func evaluateAssignment(ctx *Context, node parser.NodeAssign) error {
	if len(node.Dest) == 0 {
		return fmt.Errorf("invalid assignment: no destination")
	}

	var entry interface{}
	var canCopy bool
	var err error
	if value, ok := node.Value.(parser.NodeExpr); ok {
		entry, canCopy, err = evaluateExpression(ctx, value)
		if err != nil {
			return err
		}
		ctx.streams.set(node.Dest[0], entry, canCopy)
	}

	if len(node.Dest) > 1 {
		return fmt.Errorf("cannot assign stream to multiple variables for now")
	}

	if entry == nil {
		ctx.setLiteral(node.Dest[0], node.Value)
	}

	return nil
}

// evaluateExpression evaluates an expression node
func evaluateExpression(ctx *Context, expr parser.NodeExpr) (StreamList, bool, error) {
	var entry interface{}
	var canCopy, canPipelieCp = true, true
	var err error

	if len(expr.Input) > 1 {
		return nil, false, fmt.Errorf("multiple literal inputs not supported yet")
	}

	if len(expr.Input) == 1 {
		input := expr.Input[0]
		v, ok := input.(parser.NodeIdent)
		if !ok {
			return nil, false, fmt.Errorf("invalid input type: %T", input)
		}
		if entry, canCopy, err = ctx.streams.getAuto(v); err != nil {
			return nil, false, err
		}
	}

	streams, canPipelieCp, err := evaluatePipeline(ctx, expr.Pipeline, entry)
	canCopy = canCopy && canPipelieCp
	return streams, canCopy, err
}

func evaluatePipelineThread(ctx *Context, pipeline parser.NodePipeline, stream *Stream) (*Stream, bool, error) {

	canCopy := true

	for _, cmd := range pipeline {
		log.Println("thread cmd", cmd)
		var err error
		var canCmdCp bool
		stream, canCmdCp, err = evaluateCommand(ctx, cmd, stream)
		if err != nil {
			return nil, false, err
		}
		canCopy = canCopy && canCmdCp
	}

	return stream, canCopy, nil
}

func evaluatePipeline(ctx *Context, pipeline parser.NodePipeline, entry interface{}) (StreamList, bool, error) {
	streams := entryToList(entry)
	// if len(streams) == 0 {
	// 	return []*Stream{}, false, nil
	// }

	var err error
	first := pipeline[0]
	if first.Name == "open" {
		if streams, err = cmdOpen(ctx, first.Args); err != nil {
			return StreamList{}, false, err
		}
		pipeline = pipeline[1:]
	}

	results := make([]*Stream, 0, len(streams))
	var canCopy bool

	if len(streams) == 0 {
		result, cp, err := evaluatePipelineThread(ctx, pipeline, nil)
		if err != nil {
			return nil, false, err
		}
		results = append(results, result)
		canCopy = cp

		return results, canCopy, nil
	}
	for _, stream := range streams {
		result, cp, err := evaluatePipelineThread(ctx, pipeline, stream)
		if err != nil {
			return nil, false, err
		}
		results = append(results, result)
		canCopy = cp
	}

	return results, canCopy, nil
}

// evaluateCommand evaluates a command node
func evaluateCommand(ctx *Context, cmd parser.NodeCommand, input *Stream) (*Stream, bool, error) {
	if handler, ok := handlerMap[cmd.Name]; ok {
		if ctx.debug {
			log.Println("command: ", cmd)
		}
		return handler(ctx, input, cmd.Args)
	}
	return nil, false, fmt.Errorf("unknown command: %s", cmd.Name)
}

func applyCommandOnList(cmd cmdHandler, ctx *Context, input []*Stream, args []parser.NodeValue) ([]*Stream, bool, error) {

	outpt := make([]*Stream, 0)

	var canCpy = true
	var cp bool
	var out *Stream
	var err error

	for _, in := range input {
		if out, cp, err = cmd(ctx, in, args); err != nil {
			return []*Stream{}, false, err
		}

		outpt = append(outpt, out)
		canCpy = canCpy && cp
	}

	return outpt, canCpy, nil
}

func entryToList(entry interface{}) StreamList {
	if entry == nil {
		return StreamList{&Stream{}}
	}
	streams := make(StreamList, 0)
	var ok bool
	if streams, ok = entry.(StreamList); !ok {
		s := entry.(*Stream)
		streams = append(streams, s)
	}
	return streams
}
