package interpreter

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/andyp1xe1/vidlang/language/parser"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type cmdHandler func(*Context, *Stream, []parser.NodeValue) (*Stream, bool, error)

var handlerMap = map[string]cmdHandler{
	// "open":       cmdOpen,
	"export":     cmdExport,
	"contrast":   cmdContrast,
	"brightness": cmdBrightness,
	"saturation": cmdSaturation,
	"gamma": cmdGamma,
	"cut": cmdCut,
	"concat": cmdConcat,
}

func cmdCut(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) { panic("unimplemented") }
func cmdConcat(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) { panic("unimplemented") }

func cmdSaturation(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("saturation: %v\n", args)
	}
	if len(args) != 1 {
		return nil, canCopy, fmt.Errorf("command saturation requires exactly 1 argument")
	}
	saturation, err := getArg(ctx, args[0], ValueNumber)
	if err != nil {
		return nil, canCopy, fmt.Errorf(
			"command saturation requires a number argument but: %v", err)
	}

	return &Stream{
		FFStream: input.FFStream.Filter(
			"eq", ffmpeg.Args{fmt.Sprintf("saturation=%v", boxToPrimitive(saturation))}),
	}, canCopy, nil
}

func cmdGamma(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("gamma: %v\n", args)
	}
	if len(args) != 1 {
		return nil, canCopy, fmt.Errorf("command gamma requires exactly 1 argument")
	}
	gamma, err := getArg(ctx, args[0], ValueNumber)
	if err != nil {
		return nil, canCopy, fmt.Errorf(
			"command gamma requires a number argument but: %v", err)
	}

	return &Stream{
		FFStream: input.FFStream.Filter(
			"eq", ffmpeg.Args{fmt.Sprintf("gamma=%v", boxToPrimitive(gamma))}),
	}, canCopy, nil
}

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

	return &Stream{
		FFStream: input.FFStream.Filter(
			"eq", ffmpeg.Args{fmt.Sprintf("contrast=%v", boxToPrimitive(contrast))}),
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

	return &Stream{
		FFStream: input.FFStream.Filter(
			"eq", ffmpeg.Args{fmt.Sprintf("brightness=%v", boxToPrimitive(brightness))}),
	}, canCopy, nil
}

// cmdOpen implements the 'open' command, not a handler
func cmdOpen(ctx *Context, args []parser.NodeValue) ([]*Stream, error) {

	if len(args) != 1 {
		return nil, fmt.Errorf("open command requires exactly one argument")
	}

	var path string

	if val, err := getArg(ctx, args[0], ValueString); err != nil {
		return nil, fmt.Errorf("open command requires a string argument but: %v", err)
	} else {
		path = boxToPrimitive(val).(string)
	}

	fileInfo, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("path not found: %s", path)
	}

	if fileInfo.IsDir() {
		return openDirectory(ctx, path)
	}

	ffStream := ffmpeg.Input(path)

	stream := &Stream{
		FFStream: ffStream,
	}

	if ctx.debug {
		fmt.Printf("Opened file: %s\n", path)
	}

	return []*Stream{stream}, nil
}

func openDirectory(ctx *Context, dirPath string) ([]*Stream, error) {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %v", err)
	}

	streams := make([]*Stream, 0)
	
	// Scan for video files
	for _, file := range files {
		if file.IsDir() {
			continue // Skip subdirectories
		}
		
		filename := file.Name()
		ext := strings.ToLower(getFileExtension(filename))
		
		// Check if it's a video file
		if ext == ".mp4" || ext == ".mkv" {
			fullPath := fmt.Sprintf("%s/%s", dirPath, filename)
			
			ffStream := ffmpeg.Input(fullPath)
			stream := &Stream{
				FFStream: ffStream,
			}
			
			streams = append(streams, stream)
			
			if ctx.debug {
				fmt.Printf("Opened file from directory: %s\n", fullPath)
			}
		}
	}
	
	if len(streams) == 0 {
		return nil, fmt.Errorf("no video files found in directory: %s", dirPath)
	}
	
	return streams, nil
}

func getFileExtension(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx == -1 {
		return ""
	}
	return filename[idx:]
}


// cmdExport implements the 'export' command
func cmdExport(env *Context, _ *Stream, args []parser.NodeValue) (*Stream, bool, error) {

	if env.debug { 
		log.Println("exporting")
	}

	if len(args) != 2 {
		return nil, false, fmt.Errorf("export command requires exactly two arguments")
	}

	var outputFile string
	var inputStream interface{}
	var canCopy bool
	var err error

	// Get the stream argument
	if inputStream, canCopy, err = getStreamArg(env, args[0]); err != nil {
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

	streams := entryToList(inputStream)
	if len(streams) == 0 {
		return nil, false, fmt.Errorf("no streams to export")
	}

	// If multiple streams, append index to filename
	var lastStream *Stream
	for i, stream := range streams {
		lastStream = stream
		currentOutput := outputFile
		if len(streams) > 1 {
			ext := getFileExtension(outputFile)
			base := outputFile[:len(outputFile)-len(ext)]
			currentOutput = fmt.Sprintf("%s_%d%s", base, i, ext)
		}

		ffargs := make(ffmpeg.KwArgs)
		ffStreamArgs := make(ffmpeg.KwArgs)
		if canCopy {
			ffargs["c"] = "copy"
			ffStreamArgs["c"] = "copy"
		}

		ffStreamArgs["tune"] = "zerolatency"
		ffStreamArgs["preset"] = "ultrahigh"
		ffStreamArgs["f"] = "mpegts"

		split := stream.FFStream.Split()
		outputStream := split.Get("0").Output(currentOutput, ffargs)
		udpStream := split.Get("1").Output("udp://127.0.0.1:1234", ffStreamArgs)
		final := ffmpeg.MergeOutputs(outputStream, udpStream)

		var errBuf strings.Builder
		err = final.ErrorToStdOut().WithErrorOutput(&errBuf).Run()
		if err != nil {
			ffmpegOutput := errBuf.String()
			return nil, canCopy, fmt.Errorf("export failed: %w\nFFmpeg output:\n%s", err, ffmpegOutput)
		}

		if env.debug {
			fmt.Printf("Export completed successfully: %s\n", currentOutput)
		}
	}

	return lastStream, canCopy, nil
}

func getStreamArg(env *Context, arg parser.NodeValue) (interface{}, bool, error) {
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
