package interpreter

import (
	"fmt"
	"os"
	"strings"

	"github.com/andyp1xe1/vidlang/language/parser"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type cmdHandler func(*Context, *Stream, []parser.NodeValue) (*Stream, error)

var handlerMap = map[string]cmdHandler{
	"open":   cmdOpen,
	"export": cmdExport,
}

// cmdOpen implements the 'open' command
func cmdOpen(ctx *Context, _ *Stream, args []parser.NodeValue) (*Stream, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("open command requires exactly one argument")
	}

	var filename string

	if val, err := getArg(ctx, args[0], ValueString); err != nil {
		return nil, fmt.Errorf("open command requires a string argument but: %v", err)
	} else {
		filename = boxToPrimitive(val).(string)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filename)
	}

	ffStream := ffmpeg.Input(filename)

	stream := &Stream{
		FFStream: ffStream,
		Type:     MultiStream, // TODO split
		UseCopy:  true,
	}

	if ctx.debug {
		fmt.Printf("Opened file: %s\n", filename)
	}

	return stream, nil
}

// cmdExport implements the 'export' command
func cmdExport(env *Context, _ *Stream, args []parser.NodeValue) (*Stream, error) {

	if len(args) != 2 {
		return nil, fmt.Errorf("export command requires exactly two arguments")
	}

	var outputFile string
	var input *Stream
	var err error

	// Get the stream argument
	if input, err = getStreamArg(env, args[0]); err != nil {
		return nil, fmt.Errorf(
			"export command requires as first argument a stream but: %v", err)
	}

	// Get the output filename
	if val, err := getArg(env, args[1], ValueString); err != nil {
		return nil, fmt.Errorf(
			"export command requires as second argument a string but: %v", err)
	} else {
		outputFile = boxToPrimitive(val).(string)
	}

	ffargs := make(ffmpeg.KwArgs)
	if input.UseCopy {
		ffargs["c"] = "copy"
	}
	outputStream := input.FFStream.Output(outputFile, ffargs)

	var errBuf strings.Builder
	err = outputStream.ErrorToStdOut().WithErrorOutput(&errBuf).Run()
	if err != nil {
		ffmpegOutput := errBuf.String()
		return nil, fmt.Errorf("export failed: %w\nFFmpeg output:\n%s", err, ffmpegOutput)
	}

	if env.debug {
		fmt.Println("Export completed successfully")
	}
	return input, nil

}

func getStreamArg(env *Context, arg parser.NodeValue) (*Stream, error) {
	if arg.ValueType() == parser.ValueIdentifier {
		return env.getStream(arg.(parser.NodeIdent))
	}
	return nil, fmt.Errorf("expected an identifier but got %s", arg)
}

func getArg(env *Context, arg parser.NodeValue, expectType valueType) (ValueBox, error) {
	switch v := arg.(type) {
	case parser.NodeIdent:
		val, err := env.getVar(v)
		if err != nil {
			return ValueBox{}, err
		}
		if val.typ != expectType {
			return ValueBox{}, fmt.Errorf("expected type %v but got %v", expectType, val.typ)
		}
		return val, nil
	default:
		box := literalToBox(v)
		if box.typ != expectType {
			return ValueBox{}, fmt.Errorf("expected type %v but got %v", expectType, box.typ)
		}
		return box, nil
	}
}
