package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"

	"github.com/andyp1xe1/vidlang/language/parser"
)

func main() {
	log.SetFlags(0)

	var fileName string
	var useStdin bool
	flag.StringVar(&fileName, "script", "", "script file to parse")
	flag.BoolVar(&useStdin, "stdin", false, "read script from stdin")

	flag.Parse()

	if len(fileName) == 0 && !useStdin {
		flag.Usage()
		os.Exit(1)
	}

	var script string
	var err error

	if useStdin {
		if script, err = readStdin(); err != nil {
			log.Fatalf("Failed to read stdin: %s", err)
		}
	} else {
		if script, err = readFile(fileName); err != nil {
			log.Fatalf("Failed to read script file: %s", err)
		}
	}

	p := parser.Parse(script, true)
	for expr := range p.Expressions {
		parser.PrintTree(expr, "")
	}
}

func readFile(fileName string) (string, error) {
	res, err := os.ReadFile(fileName)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func readStdin() (string, error) {
	var script string
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}
		script += line
	}

	return script, nil
}
