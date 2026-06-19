package main

import "unicode"

type Lexer struct {
	src  []rune
	pos  int
	line int
	col  int
}

func NewLexer(src string) *Lexer {
	return &Lexer{src: []rune(src), line: 1, col: 1}
}

func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		t := l.next()
		tokens = append(tokens, t)
		if t.Kind == TokEOF {
			break
		}
	}
	return tokens
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) advance() rune {
	r := l.src[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) next() Token {
	ws := l.consumeWS()
	if l.pos >= len(l.src) {
		return Token{Kind: TokEOF, PreWS: ws}
	}
	line, col := l.line, l.col
	r := l.peek()

	if r == '\n' {
		l.advance()
		return Token{Kind: TokNewline, Value: "\n", Line: line, Col: col, PreWS: ws}
	}

	if r == '/' && l.pos+1 < len(l.src) {
		if l.src[l.pos+1] == '/' {
			start := l.pos
			for l.pos < len(l.src) && l.peek() != '\n' {
				l.advance()
			}
			return Token{Kind: TokOp, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
		}
		if l.src[l.pos+1] == '*' {
			start := l.pos
			l.advance()
			l.advance()
			for l.pos+1 < len(l.src) {
				if l.peek() == '*' && l.src[l.pos+1] == '/' {
					l.advance()
					l.advance()
					break
				}
				l.advance()
			}
			return Token{Kind: TokOp, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
		}
	}

	if r == '"' || r == '`' {
		return l.scanString(ws, line, col)
	}
	if r == '\'' {
		return l.scanRune(ws, line, col)
	}

	if unicode.IsLetter(r) || r == '_' {
		return l.scanIdent(ws, line, col)
	}
	if unicode.IsDigit(r) {
		return l.scanNumber(ws, line, col)
	}

	l.advance()
	switch r {
	case '{':
		return Token{Kind: TokLBrace, Value: "{", Line: line, Col: col, PreWS: ws}
	case '}':
		return Token{Kind: TokRBrace, Value: "}", Line: line, Col: col, PreWS: ws}
	case '(':
		return Token{Kind: TokLParen, Value: "(", Line: line, Col: col, PreWS: ws}
	case ')':
		return Token{Kind: TokRParen, Value: ")", Line: line, Col: col, PreWS: ws}
	case '[':
		return Token{Kind: TokLBrack, Value: "[", Line: line, Col: col, PreWS: ws}
	case ']':
		return Token{Kind: TokRBrack, Value: "]", Line: line, Col: col, PreWS: ws}
	case ',':
		return Token{Kind: TokComma, Value: ",", Line: line, Col: col, PreWS: ws}
	case ';':
		return Token{Kind: TokSemicolon, Value: ";", Line: line, Col: col, PreWS: ws}
	case '.':
		if l.pos+1 < len(l.src) && l.peek() == '.' && l.src[l.pos+1] == '.' {
			l.advance()
			l.advance()
			return Token{Kind: TokEllipsis, Value: "...", Line: line, Col: col, PreWS: ws}
		}
		return Token{Kind: TokDot, Value: ".", Line: line, Col: col, PreWS: ws}
	}

	start := l.pos - 1
	ops := "+-*/%&|^<>=!~:?"
	for l.pos < len(l.src) && containsRune(ops, l.peek()) {
		l.advance()
	}
	return Token{Kind: TokOp, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
}

func (l *Lexer) consumeWS() string {
	start := l.pos
	for l.pos < len(l.src) && (l.peek() == ' ' || l.peek() == '\t' || l.peek() == '\r') {
		l.advance()
	}
	return string(l.src[start:l.pos])
}

func (l *Lexer) scanIdent(ws string, line, col int) Token {
	start := l.pos
	for l.pos < len(l.src) && (unicode.IsLetter(l.peek()) || unicode.IsDigit(l.peek()) || l.peek() == '_') {
		l.advance()
	}
	return Token{Kind: TokIdent, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
}

func (l *Lexer) scanNumber(ws string, line, col int) Token {
	start := l.pos
	kind := TokInt
	if l.peek() == '0' && l.pos+1 < len(l.src) && (l.src[l.pos+1] == 'x' || l.src[l.pos+1] == 'X') {
		l.advance()
		l.advance()
		for l.pos < len(l.src) && isHexDigit(l.peek()) {
			l.advance()
		}
		return Token{Kind: kind, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
	}
	for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
		l.advance()
	}
	if l.pos < len(l.src) && l.peek() == '.' {
		kind = TokFloat
		l.advance()
		for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	if l.pos < len(l.src) && (l.peek() == 'e' || l.peek() == 'E') {
		kind = TokFloat
		l.advance()
		if l.pos < len(l.src) && (l.peek() == '+' || l.peek() == '-') {
			l.advance()
		}
		for l.pos < len(l.src) && unicode.IsDigit(l.peek()) {
			l.advance()
		}
	}
	return Token{Kind: kind, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
}

func (l *Lexer) scanString(ws string, line, col int) Token {
	quote := l.advance()
	start := l.pos - 1
	if quote == '`' {
		for l.pos < len(l.src) && l.peek() != '`' {
			l.advance()
		}
		if l.pos < len(l.src) {
			l.advance()
		}
	} else {
		for l.pos < len(l.src) && l.peek() != '"' {
			if l.peek() == '\\' {
				l.advance()
			}
			l.advance()
		}
		if l.pos < len(l.src) {
			l.advance()
		}
	}
	return Token{Kind: TokString, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
}

func (l *Lexer) scanRune(ws string, line, col int) Token {
	start := l.pos
	l.advance()
	for l.pos < len(l.src) && l.peek() != '\'' {
		if l.peek() == '\\' {
			l.advance()
		}
		l.advance()
	}
	if l.pos < len(l.src) {
		l.advance()
	}
	return Token{Kind: TokRune, Value: string(l.src[start:l.pos]), Line: line, Col: col, PreWS: ws}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

func isHexDigit(r rune) bool {
	return unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
