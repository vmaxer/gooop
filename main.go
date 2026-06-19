package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

const versionString = "Go++ (Go + OOP) 1.0.0"

func main() {
	outFile := flag.String("o", "", "output file (default: stdout)")
	versionFlag := flag.Bool("version", false, "display the current version")
	versionShort := flag.Bool("v", false, "display the current version")
	flag.Parse()

	if *versionFlag || *versionShort {
		fmt.Println(versionString)
		return
	}

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
	gen := NewGenerator()
	if flag.NArg() > 0 {
		gen.SetFile(flag.Arg(0))
	}
	out := gen.Generate(file)

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
