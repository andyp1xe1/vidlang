package main

import (
	"fmt"
	"log"
	"os"

	"github.com/andyp1xe1/vidlang/language/interpreter"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: vidlang <script.vl>")
		os.Exit(1)
	}

	scriptPath := os.Args[1]

	// Read the script file
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		log.Fatalf("Error reading script file: %v", err)
	}

	if err := interpreter.Interpret(string(content), true); err != nil {
		log.Fatalf("Error executing script: %v", err)
	}

	fmt.Println("Script executed successfully")
}
