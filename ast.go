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
}

type Field struct {
	Names   []string
	Type    []Token
	Private bool
}

type Constructor struct {
	Params []Token
	Body   []Token
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
}

func (RawDecl) declNode()   {}
func (ClassDecl) declNode() {}
