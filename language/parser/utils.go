package parser

import "fmt"

func PrintTree(n interface{}, indent string) {
	switch node := n.(type) {

	case nodeAssign:
		fmt.Printf("%sAssignment:\n", indent)
		fmt.Printf("%sDest:\n", indent+"  ")
		// node.dest is a nodeList[nodeIdent]
		for _, dest := range node.dest {
			PrintTree(dest, indent+"    ")
		}
		fmt.Printf("%sValue:\n", indent+"  ")
		PrintTree(node.value, indent+"    ")

	case nodeExprMath:
		fmt.Printf("%sMath Expression: (Operator: %s)\n", indent, node.op)
		fmt.Printf("%sLeft:\n", indent+"  ")
		PrintTree(node.left, indent+"    ")
		fmt.Printf("%sRight:\n", indent+"  ")
		PrintTree(node.right, indent+"    ")

	case nodeExpr:
		fmt.Printf("%sExpression:\n", indent)
		if node.input != nil {
			fmt.Printf("%sInput:\n", indent+"  ")
			PrintTree(node.input, indent+"    ")
		}
		if len(node.pipeline) > 0 {
			fmt.Printf("%sPipeline:\n", indent+"  ")
			for i, cmd := range node.pipeline {
				fmt.Printf("%sCommand %d:\n", indent+"    ", i)
				fmt.Printf("%s  Name: %s\n", indent+"      ", cmd.name)
				if len(cmd.args) > 0 {
					fmt.Printf("%s  Args:\n", indent+"      ")
					for _, arg := range cmd.args {
						PrintTree(arg, indent+"        ")
					}
				}
			}
		}

	case nodeSubExpr:
		fmt.Printf("%sSub-Expression:\n", indent)
		fmt.Printf("%sParams:\n", indent+"  ")
		for _, param := range node.params {
			PrintTree(param, indent+"    ")
		}
		fmt.Printf("%sBody:\n", indent+"  ")
		PrintTree(node.body, indent+"    ")

	case nodeList[nodeValue]:
		fmt.Printf("%sList (length %d):\n", indent, len(node))
		for _, item := range node {
			PrintTree(item, indent+"  ")
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
