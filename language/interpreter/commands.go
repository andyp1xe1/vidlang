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
	"gamma":      cmdGamma,
	"cut":        cmdTrim,
	"concat":     cmdConcat,
	"hue":        cmdHue,
	"flip":       cmdFlip,
	"stack":      cmdStack,
}

func cmdTrim(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("trim: %v\n", args)
	}

	if len(args) != 2 {
		return nil, canCopy, fmt.Errorf("command trim requires exactly 2 arguments (start and end)")
	}

	start, err := getArg(ctx, args[0], ValueNumber)
	if err != nil {
		return nil, canCopy, fmt.Errorf("trim command requires a number for start time but: %v", err)
	}

	end, err := getArg(ctx, args[1], ValueNumber)
	if err != nil {
		return nil, canCopy, fmt.Errorf("trim command requires a number for end time but: %v", err)
	}

	startVal := boxToPrimitive(start).(float64)
	endVal := boxToPrimitive(end).(float64)

	v := input.FFStream.Trim(ffmpeg.KwArgs{
		"start": startVal,
		"end":   endVal,
	}).Filter("setpts", ffmpeg.Args{"PTS-STARTPTS"}) //.Filter("fps", ffmpeg.Args{"30"})

	return &Stream{FFStream: v}, canCopy, err
}

func cmdConcat(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("concat: %v\n", args)
	}

	if len(args) == 0 {
		return nil, canCopy, fmt.Errorf("concat command requires at least one stream argument")
	}

	// // Collect all streams to concatenate
	// streams := []*ffmpeg.Stream{input.FFStream}

	streams := make([]*ffmpeg.Stream, 0)

	for _, arg := range args {
		stream, _, err := getStreamArg(ctx, arg)
		if err != nil {
			return nil, canCopy, fmt.Errorf("concat argument must be a stream but: %v", err)
		}

		streamList := entryToList(stream)
		if len(streamList) != 1 {
			return nil, canCopy, fmt.Errorf("concat currently only supports single streams per argument")
		}

		streams = append(streams, streamList[0].FFStream)
	}

	normalizedStreams := make([]*ffmpeg.Stream, len(streams))
	for i, stream := range streams {
		// normalize everything: resolution, framerate, aspect ratio
		normalizedStreams[i] = stream.
			Filter("scale", ffmpeg.Args{"1920:1080:force_original_aspect_ratio=decrease"}).
			Filter("pad", ffmpeg.Args{"1920:1080:(ow-iw)/2:(oh-ih)/2"}).
			Filter("setpts", ffmpeg.Args{"PTS-STARTPTS"})
	}

	concatStream := ffmpeg.Concat(normalizedStreams)

	return &Stream{
		FFStream: concatStream,
	}, canCopy, nil
}

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

func cmdHue(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("hue: %v\n", args)
	}
	if len(args) != 1 {
		return nil, canCopy, fmt.Errorf("command hue requires exactly 1 argument")
	}
	hue, err := getArg(ctx, args[0], ValueNumber)
	if err != nil {
		return nil, canCopy, fmt.Errorf(
			"command hue requires a number argument but: %v", err)
	}

	return &Stream{
		FFStream: input.FFStream.Hue(ffmpeg.KwArgs{"h": boxToPrimitive(hue)}),
	}, canCopy, nil
}

func cmdFlip(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("flip: %v\n", args)
	}
	if len(args) != 1 {
		return nil, canCopy, fmt.Errorf("command flip  exactly 1 argument")
	}
	flip, err := getArg(ctx, args[0], ValueString)
	if err != nil {
		return nil, canCopy, fmt.Errorf(
			"command flip requires a string argument but: %v", err)
	}

	if strings.Compare(boxToPrimitive(flip).(string), "h") != 0 && strings.Compare(boxToPrimitive(flip).(string), "v") != 0 {
		return nil, false, fmt.Errorf("command requires either `h` or `v` as arguments")
	}

	if strings.Compare(boxToPrimitive(flip).(string), "h") == 0 {
		return &Stream{
			FFStream: input.FFStream.VFlip(),
		}, canCopy, nil
	} else {
		return &Stream{
			FFStream: input.FFStream.HFlip(),
		}, canCopy, nil
	}
}

func cmdStack(ctx *Context, input *Stream, args []parser.NodeValue) (*Stream, bool, error) {
	canCopy := false
	if ctx.debug {
		fmt.Printf("stack: %v\n", args)
	}

	if len(args) < 2 {
		return nil, canCopy, fmt.Errorf("stack command requires at least 2 arguments (direction and 1+ streams)")
	}

	// Get direction argument (first arg)
	direction, err := getArg(ctx, args[0], ValueString)
	if err != nil {
		return nil, canCopy, fmt.Errorf("stack command requires a string for direction but: %v", err)
	}

	directionStr := boxToPrimitive(direction).(string)
	if directionStr != "h" && directionStr != "v" {
		return nil, canCopy, fmt.Errorf("stack direction must be 'h' (horizontal) or 'v' (vertical)")
	}

	// Collect all streams to stack (starting from second arg)
	streams := make([]*ffmpeg.Stream, 0)

	// Add the input stream first
	streams = append(streams, input.FFStream)

	// Add all the streams from arguments
	for i := 1; i < len(args); i++ {
		stream, _, err := getStreamArg(ctx, args[i])
		if err != nil {
			return nil, canCopy, fmt.Errorf("stack argument %d must be a stream but: %v", i+1, err)
		}

		streamList := entryToList(stream)
		if len(streamList) != 1 {
			return nil, canCopy, fmt.Errorf("stack currently only supports single streams per argument")
		}

		streams = append(streams, streamList[0].FFStream)
	}

	// Normalize all streams to same dimensions to avoid squashing
	normalizedStreams := make([]*ffmpeg.Stream, len(streams))

	if directionStr == "h" {
		// For horizontal stacking, normalize to same height (1080p), keep aspect ratio
		for i, stream := range streams {
			normalizedStreams[i] = stream.
				Filter("scale", ffmpeg.Args{"-1:1080:force_original_aspect_ratio=decrease"}).
				Filter("pad", ffmpeg.Args{"iw:1080:(iw-ow)/2:(ih-oh)/2"}).
				Filter("setsar", ffmpeg.Args{"1:1"})
		}
	} else {
		// For vertical stacking, normalize to same width (1920px), keep aspect ratio
		for i, stream := range streams {
			normalizedStreams[i] = stream.
				Filter("scale", ffmpeg.Args{"1920:-1:force_original_aspect_ratio=decrease"}).
				Filter("pad", ffmpeg.Args{"1920:ih:(iw-ow)/2:(ih-oh)/2"}).
				Filter("setsar", ffmpeg.Args{"1:1"})
		}
	}

	// Apply the appropriate stack filter
	var stackedStream *ffmpeg.Stream
	if directionStr == "h" {
		// Horizontal stack (side by side)
		stackedStream = ffmpeg.Filter(normalizedStreams, "hstack", ffmpeg.Args{fmt.Sprintf("inputs=%d", len(normalizedStreams))})
	} else {
		// Vertical stack (one above the other)
		stackedStream = ffmpeg.Filter(normalizedStreams, "vstack", ffmpeg.Args{fmt.Sprintf("inputs=%d", len(normalizedStreams))})
	}

	return &Stream{
		FFStream: stackedStream,
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

		if !canCopy {
			ffargs["c:v"] = "libx264"
			ffargs["c:a"] = "aac"
		} else if !env.preview {
			ffargs["c"] = "copy"
			ffStreamArgs["c"] = "copy"
		}

		ffStreamArgs["tune"] = "zerolatency"
		ffStreamArgs["preset"] = "ultrafast"
		ffStreamArgs["f"] = "mpegts"
		ffStreamArgs["r"] = "24"
		//ffStreamArgs["s"] = "1280x720"

		ffargs["r"] = "30"
		ffargs["s"] = "1920x1080"
		ffargs["fflags"] = "+genpts"
		ffargs["y"] = ""

		var outputs []*ffmpeg.Stream = make([]*ffmpeg.Stream, 0)
		if env.preview {

			if err := env.StartPreviewPlayer(); err != nil {
				log.Printf("Warning: Failed to start preview player: %v", err)
			}

			split := stream.FFStream.Split()
			outputStream := split.Get("0").Output(currentOutput, ffargs)
			udpStream := split.Get("1").Output("udp://127.0.0.1:1234", ffStreamArgs)
			outputs = append(outputs, outputStream, udpStream)
		} else {
			out := stream.FFStream.Output(currentOutput, ffargs)
			outputs = append(outputs, out)
		}

		final := ffmpeg.MergeOutputs(outputs...)

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
	// log.Println("Type: ", arg.ValueType())
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
