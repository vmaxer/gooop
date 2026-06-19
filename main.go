package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	outFile := flag.String("o", "", "output file (default: stdout)")
	flag.Parse()

	var src []byte
	var err error

	if flag.NArg() > 0 {
		src, err = os.ReadFile(flag.Arg(0))
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

	if *outFile != "" {
		err = os.WriteFile(*outFile, []byte(out), 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		fmt.Print(out)
	}
}
