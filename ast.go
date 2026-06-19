package main

type File struct {
	Decls []Decl
}

type Decl interface{ declNode() }

type RawDecl struct {
	Tokens []Token
}

type ClassDecl struct {
	Name       string
	Parent     string
	Implements []string
	Includes   []string
	IsAbstract bool
	IsData     bool
	Fields     []Field
	Ctor       *Constructor
	Methods    []Method
	Line       int
}

type Field struct {
	Names   []string
	Type    []Token
	Private bool
}

type Constructor struct {
	Params []Token
	Body   []Token
	Line   int
}

type Method struct {
	Name       string
	TypeParams []Token
	Params     []Token
	Results    []Token
	Body       []Token
	IsStatic   bool
	IsAbstract bool
	Operator   string
	Line       int
}

func (RawDecl) declNode()   {}
func (ClassDecl) declNode() {}
