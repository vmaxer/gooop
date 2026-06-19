package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	var src []byte
	var err error

	if len(os.Args) > 1 {
		src, err = os.ReadFile(os.Args[1])
	} else {
		src, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	tokens := NewLexer(string(src)).Tokenize()
	file := NewParser(tokens).Parse()
	out := NewGenerator().Generate(file)
	fmt.Print(out)
}
