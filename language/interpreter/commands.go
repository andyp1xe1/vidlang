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
	switch arg := args[0].(type) {
	case parser.NodeLiteralString:
		filename = strings.Trim(string(arg), "\"")
	default:
		return nil, fmt.Errorf("open command requires a string argument")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", filename)
	}

	ffStream := ffmpeg.Input(filename)

	stream := &Stream{
		FFStream: ffStream,
		Type:     MultiStream, // TODO split
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
	var ident parser.NodeIdent

	// Get the stream argument
	ident, ok := args[0].(parser.NodeIdent)
	if !ok {
		return nil, fmt.Errorf("export command requires as first argument a stream but got: %T", args[0])
	}
	if v, err := env.getVar(ident); err != nil {
		return nil, err
	} else if v.typ != ValueStream {
		return nil, fmt.Errorf("export command requires as first argument a stream but got: %s", args[0])
	} else {
		input = v.any.(*Stream)
	}

	// Get the output filename
	switch arg := args[1].(type) {
	case parser.NodeLiteralString:
		outputFile = strings.Trim(string(arg), "\"")
	case parser.NodeIdent:
		var err error
		v, err := env.getVar(arg)
		if err != nil {
			return nil, err
		}
		if v.typ != ValueString {
			return nil, fmt.Errorf("export command requires as second argument a string but got: %s", args[1])
		}
	default:
		return nil, fmt.Errorf("export command requires as second argument a string but got: %s", args[1])
	}

	outputStream := input.FFStream.Output(outputFile).OverWriteOutput()

	if env.debug {
		fmt.Printf("Exporting to file: %s\n", outputFile)
		fmt.Printf("FFmpeg command: %s\n", outputStream)
	}

	var errBuf strings.Builder

	err := outputStream.ErrorToStdOut().WithErrorOutput(&errBuf).Run()
	if err != nil {
		ffmpegOutput := errBuf.String()
		return nil, fmt.Errorf("export failed: %w\nFFmpeg output:\n%s", err, ffmpegOutput)
	}

	if env.debug {
		fmt.Println("Export completed successfully")
	}

	return input, nil
}
