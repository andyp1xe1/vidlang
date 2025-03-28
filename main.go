package main

import (
	"fmt"
	"os"
)

// printTree recursively prints the AST node n with indentation.
func printTree(n interface{}, indent string) {
	switch node := n.(type) {

	case nodeAssign:
		fmt.Printf("%sAssignment:\n", indent)
		fmt.Printf("%sDest:\n", indent+"  ")
		// node.dest is a nodeList[nodeIdent]
		for _, dest := range node.dest {
			printTree(dest, indent+"    ")
		}
		fmt.Printf("%sValue:\n", indent+"  ")
		printTree(node.value, indent+"    ")

	case nodeExprMath:
		fmt.Printf("%sMath Expression: (Operator: %s)\n", indent, node.op)
		fmt.Printf("%sLeft:\n", indent+"  ")
		printTree(node.left, indent+"    ")
		fmt.Printf("%sRight:\n", indent+"  ")
		printTree(node.right, indent+"    ")

	case nodeExpr:
		fmt.Printf("%sExpression:\n", indent)
		if node.input != nil {
			fmt.Printf("%sInput:\n", indent+"  ")
			printTree(node.input, indent+"    ")
		}
		if len(node.pipeline) > 0 {
			fmt.Printf("%sPipeline:\n", indent+"  ")
			for i, cmd := range node.pipeline {
				fmt.Printf("%sCommand %d:\n", indent+"    ", i)
				fmt.Printf("%s  Name: %s\n", indent+"      ", cmd.name)
				if len(cmd.args) > 0 {
					fmt.Printf("%s  Args:\n", indent+"      ")
					for _, arg := range cmd.args {
						printTree(arg, indent+"        ")
					}
				}
			}
		}

	case nodeSubExpr:
		fmt.Printf("%sSub-Expression:\n", indent)
		fmt.Printf("%sParams:\n", indent+"  ")
		for _, param := range node.params {
			printTree(param, indent+"    ")
		}
		fmt.Printf("%sBody:\n", indent+"  ")
		printTree(node.body, indent+"    ")

	case nodeList[nodeValue]:
		fmt.Printf("%sList (length %d):\n", indent, len(node))
		for _, item := range node {
			printTree(item, indent+"  ")
		}

	// For literal values and identifiers
	case nodeLiteralString, nodeLiteralNumber, nodeLiteralBool, nodeIdent, nodeSelfStar:
		fmt.Printf("%s%v\n", indent, node)

	default:
		// If the node implements nodeValue, use its String method.
		if nv, ok := n.(nodeValue); ok {
			fmt.Printf("%s%s\n", indent, nv.String())
		} else {
			fmt.Printf("%sUnknown node: %#v\n", indent, n)
		}
	}
}

func main() {
	testScript := `
#!/bin/venv vidlang

videoTrack, audioTrack := open "video.mp4"
introVid, introAud := open "intro.mp4"
outro, outroAud := open "outro.mp4"
audioTrack |> volume 1.5

# stream is a global variable representing the latest pipeline result

audioTrack = [stream, *]
		|> crossfade 0.5
		|> pitch 1.5

videoTrack =*
    |> brightness 1.3
    |> contrast 1.1

audSequence := [introAud, audioTrack, outroAud]
    |> map [i, el] ( el |> volume 0.5*i+1 )
vidSequence :=  [intoVid, videoTrack, outro]
trackline audSequence vidSequence
# or trackline audSequence [introVid, videoTrack, outro]
export "final.mp4"
`

	l := lex(testScript)

	for {
		item := <-l.items
		if item.typ == itemEOF {
			break
		}
		fmt.Printf("%#v\n", item)
		if item.typ == itemError {
			os.Exit(1)
		}
	}

	p := parse(testScript)
	for expr := range p.expressions {
		printTree(expr, "")
		//printNode(expr)
	}

}
