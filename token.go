package main

type TokenKind int

const (
	TokEOF TokenKind = iota
	TokIdent
	TokInt
	TokFloat
	TokString
	TokRune
	TokLBrace
	TokRBrace
	TokLParen
	TokRParen
	TokLBrack
	TokRBrack
	TokComma
	TokSemicolon
	TokDot
	TokEllipsis
	TokOp
	TokNewline
	TokRawChar
)

type Token struct {
	Kind  TokenKind
	Value string
	Line  int
	Col   int
	PreWS string
}
