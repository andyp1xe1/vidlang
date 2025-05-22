package interpreter

import (
	"fmt"
	"os"
	"strings"

	"github.com/andyp1xe1/vidlang/language/parser"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type cmdHandler func(*Context, *Stream, []parser.NodeValue) (*Stream, bool, error)

var handlerMap = map[string]cmdHandler{
	"open":       cmdOpen,
	"export":     cmdExport,
	"contrast":   cmdContrast,
	"brightness": cmdBrightness,
}

// Mock for testing
func cmdContrast(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("Contrast: %v\n", args)
	}
	if len(args) != 1 {
		return nil, canCopy, fmt.Errorf("command contrast requires exactly 1 argument")
	}
	contrast, err := getArg(ctx, args[0], ValueNumber)
	if err != nil {
		return nil, canCopy, fmt.Errorf(
			"command contrast requires a number argument but: %v", err)
	}
	//input.UseCopy = false
	return &Stream{
		FFStream: input.FFStream.Filter(
			"eq", ffmpeg.Args{fmt.Sprintf("contrast=%v", boxToPrimitive(contrast))}),
		Type: input.Type,
	}, canCopy, nil
}

func cmdBrightness(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("Brightness: %v\n", args)
	}
	if len(args) != 1 {
		return nil, canCopy, fmt.Errorf("command brightness requires exactly 1 argument")
	}
	brightness, err := getArg(ctx, args[0], ValueNumber)
	if err != nil {
		return nil, canCopy, fmt.Errorf(
			"command brightness requires a number argument but: %v", err)
	}
	//input.UseCopy = false
	return &Stream{
		FFStream: input.FFStream.Filter(
			"eq", ffmpeg.Args{fmt.Sprintf("brightness=%v", boxToPrimitive(brightness))}),
		Type: input.Type,
	}, canCopy, nil
}

// cmdOpen implements the 'open' command
func cmdOpen(ctx *Context, _ *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := true

	if len(args) != 1 {
		return nil, canCopy, fmt.Errorf("open command requires exactly one argument")
	}

	var filename string

	if val, err := getArg(ctx, args[0], ValueString); err != nil {
		return nil, canCopy, fmt.Errorf("open command requires a string argument but: %v", err)
	} else {
		filename = boxToPrimitive(val).(string)
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, canCopy, fmt.Errorf("file not found: %s", filename)
	}

	ffStream := ffmpeg.Input(filename)

	stream := &Stream{
		FFStream: ffStream,
		Type:     MultiStream, // TODO split
	}

	if ctx.debug {
		fmt.Printf("Opened file: %s\n", filename)
	}

	return stream, canCopy, nil
}

// cmdExport implements the 'export' command
func cmdExport(env *Context, _ *Stream, args []parser.NodeValue) (*Stream, bool, error) {

	if len(args) != 2 {
		return nil, false, fmt.Errorf("export command requires exactly two arguments")
	}

	var outputFile string
	var input *Stream
	var canCopy bool
	var err error

	// Get the stream argument
	if input, canCopy, err = getStreamArg(env, args[0]); err != nil {
		return nil, canCopy, fmt.Errorf(
			"export command requires as first argument a stream but: %v", err)
	}

	// Get the output filename
	if val, err := getArg(env, args[1], ValueString); err != nil {
		return nil, canCopy, fmt.Errorf(
			"export command requires as second argument a string but: %v", err)
	} else {
		outputFile = boxToPrimitive(val).(string)
	}

	ffargs := make(ffmpeg.KwArgs)
	ffStreamArgs := make(ffmpeg.KwArgs)
	if canCopy {
		ffargs["c"] = "copy"
		ffStreamArgs["c"] = "copy"
	}

	// -preset ultrafast -tune zerolatency -b:v 1M -f mpegts "udp://127.0.0.1:1234"
	ffStreamArgs["tune"] = "zerolatency"
	ffStreamArgs["preset"] = "ultrahigh"
	ffStreamArgs["f"] = "mpegts"
	//ffStreamArgs["b:v"] = "1M"

	split := input.FFStream.Split()
	outputStream := split.Get("0").Output(outputFile, ffargs)
	udpStream := split.Get("1").Output("udp://127.0.0.1:1234", ffStreamArgs)
	final := ffmpeg.MergeOutputs(outputStream, udpStream)

	var errBuf strings.Builder
	err = final.ErrorToStdOut().WithErrorOutput(&errBuf).Run()
	if err != nil {
		ffmpegOutput := errBuf.String()
		return nil, canCopy, fmt.Errorf("export failed: %w\nFFmpeg output:\n%s", err, ffmpegOutput)
	}

	if env.debug {
		fmt.Println("Export completed successfully")
	}
	return input, canCopy, nil

}

func getStreamArg(env *Context, arg parser.NodeValue) (*Stream, bool, error) {
	if arg.ValueType() == parser.ValueIdentifier {
		return env.streams.getAuto(arg.(parser.NodeIdent))
	}
	return nil, false, fmt.Errorf("expected an identifier but got %s", arg)
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
