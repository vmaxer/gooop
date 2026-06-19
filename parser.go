package main

var operatorNames = map[string]string{
	"+":  "Add",
	"-":  "Sub",
	"*":  "Mul",
	"/":  "Div",
	"%":  "Mod",
	"==": "Eq",
	"!=": "Neq",
	"<":  "Lt",
	"<=": "Lte",
	">":  "Gt",
	">=": "Gte",
	"[]": "At",
	"<<": "Shl",
	">>": "Shr",
	"&":  "And",
	"|":  "Or",
	"^":  "Xor",
}

type Parser struct {
	tokens    []Token
	pos       int
	className string
}

func NewParser(tokens []Token) *Parser {
	return &Parser{tokens: tokens}
}

func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Kind: TokEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *Parser) skipNewlines() {
	for p.peek().Kind == TokNewline {
		p.advance()
	}
}

func (p *Parser) Parse() *File {
	f := &File{}
	for p.peek().Kind != TokEOF {
		if p.isClassStart() {
			f.Decls = append(f.Decls, p.parseClass())
		} else {
			f.Decls = append(f.Decls, p.parseRaw())
		}
	}
	return f
}

func (p *Parser) isClassStart() bool {
	if p.peek().Kind != TokIdent {
		return false
	}
	v := p.peek().Value
	if v == "class" || v == "data" || v == "abstract" {
		return true
	}
	return false
}

func (p *Parser) parseClass() ClassDecl {
	c := ClassDecl{Line: p.peek().Line}

	if p.peek().Value == "abstract" {
		c.IsAbstract = true
		p.advance()
		p.skipNewlines()
	}
	if p.peek().Value == "data" {
		c.IsData = true
		p.advance()
		p.skipNewlines()
	}

	p.advance()
	p.skipNewlines()
	c.Name = p.advance().Value
	p.className = c.Name
	p.skipNewlines()

	if p.peek().Kind == TokIdent && p.peek().Value == "extends" {
		p.advance()
		p.skipNewlines()
		c.Parent = p.advance().Value
		p.skipNewlines()
	}

	if p.peek().Kind == TokIdent && p.peek().Value == "implements" {
		p.advance()
		p.skipNewlines()
		c.Implements = p.parseIdentList()
		p.skipNewlines()
	}

	if p.peek().Kind == TokIdent && p.peek().Value == "includes" {
		p.advance()
		p.skipNewlines()
		c.Includes = p.parseIdentList()
		p.skipNewlines()
	}

	p.expect(TokLBrace)
	p.skipNewlines()

	for p.peek().Kind != TokRBrace && p.peek().Kind != TokEOF {
		p.skipNewlines()
		if p.peek().Kind == TokRBrace {
			break
		}
		if p.peek().Kind == TokIdent && p.peek().Value == "static" {
			p.advance()
			p.skipNewlines()
			m := p.parseMethodOrOperator()
			m.IsStatic = true
			c.Methods = append(c.Methods, m)
		} else if p.peek().Kind == TokIdent && p.peek().Value == "abstract" {
			p.advance()
			p.skipNewlines()
			m := p.parseAbstractMethod()
			c.Methods = append(c.Methods, m)
		} else if p.peek().Kind == TokIdent && p.peek().Value == "func" {
			c.Methods = append(c.Methods, p.parseMethodOrOperator())
		} else if p.peek().Kind == TokIdent && (p.peek().Value == "new" || p.peek().Value == c.Name) && p.lookaheadParen() {
			c.Ctor = p.parseConstructor()
		} else if p.peek().Kind == TokIdent && p.peek().Value == "private" {
			p.advance()
			p.skipNewlines()
			f := p.parseField()
			f.Private = true
			c.Fields = append(c.Fields, f)
		} else {
			c.Fields = append(c.Fields, p.parseField())
		}
		p.skipNewlines()
	}
	p.expect(TokRBrace)
	p.className = ""
	return c
}

func (p *Parser) parseIdentList() []string {
	var names []string
	names = append(names, p.advance().Value)
	for p.peek().Kind == TokComma {
		p.advance()
		p.skipNewlines()
		names = append(names, p.advance().Value)
	}
	return names
}

func (p *Parser) parseField() Field {
	f := Field{}
	f.Names = append(f.Names, p.advance().Value)
	for p.peek().Kind == TokComma {
		p.advance()
		p.skipNewlines()
		f.Names = append(f.Names, p.advance().Value)
	}
	f.Type = p.collectUntilNewlineOrBrace()
	return f
}

func (p *Parser) collectUntilNewlineOrBrace() []Token {
	var toks []Token
	for p.peek().Kind != TokNewline && p.peek().Kind != TokRBrace && p.peek().Kind != TokEOF {
		toks = append(toks, p.advance())
	}
	return toks
}

func (p *Parser) parseConstructor() *Constructor {
	ctor := &Constructor{Line: p.peek().Line}
	p.advance()
	ctor.Params = p.collectBalanced(TokLParen, TokRParen)
	p.skipNewlines()
	ctor.Body = p.collectBalanced(TokLBrace, TokRBrace)
	return ctor
}

func (p *Parser) parseMethodOrOperator() Method {
	m := Method{Line: p.peek().Line}
	p.advance()
	p.skipNewlines()

	if p.peek().Kind == TokOp || (p.peek().Kind == TokLBrack && p.lookaheadCloseBrack()) {
		m.Operator = p.parseOperatorName()
		m.Name = operatorNames[m.Operator]
		if m.Name == "" {
			m.Name = "Op_" + m.Operator
		}
	} else {
		m.Name = p.advance().Value
	}

	if p.peek().Kind == TokLBrack && m.Operator == "" {
		m.TypeParams = p.collectBalanced(TokLBrack, TokRBrack)
	}

	m.Params = p.collectBalanced(TokLParen, TokRParen)
	m.Results = p.collectResults()
	p.skipNewlines()
	m.Body = p.collectBalanced(TokLBrace, TokRBrace)
	return m
}

func (p *Parser) parseOperatorName() string {
	if p.peek().Kind == TokLBrack {
		p.advance()
		p.advance()
		return "[]"
	}
	return p.advance().Value
}

func (p *Parser) lookaheadCloseBrack() bool {
	if p.pos+1 < len(p.tokens) {
		return p.tokens[p.pos+1].Kind == TokRBrack
	}
	return false
}

func (p *Parser) parseAbstractMethod() Method {
	m := Method{IsAbstract: true}
	p.advance()
	p.skipNewlines()
	m.Name = p.advance().Value
	m.Params = p.collectBalanced(TokLParen, TokRParen)
	m.Results = p.collectUntilNewlineOrBrace()
	return m
}

func (p *Parser) collectResults() []Token {
	var toks []Token
	for p.peek().Kind != TokLBrace && p.peek().Kind != TokNewline && p.peek().Kind != TokEOF {
		toks = append(toks, p.advance())
	}
	return toks
}

func (p *Parser) collectBalanced(open, close TokenKind) []Token {
	var toks []Token
	p.expect(open)
	depth := 1
	for depth > 0 && p.peek().Kind != TokEOF {
		t := p.advance()
		if t.Kind == open {
			depth++
		} else if t.Kind == close {
			depth--
			if depth == 0 {
				break
			}
		}
		toks = append(toks, t)
	}
	return toks
}

func (p *Parser) expect(kind TokenKind) {
	for p.peek().Kind == TokNewline {
		p.advance()
	}
	if p.peek().Kind == kind {
		p.advance()
	}
}

func (p *Parser) lookaheadParen() bool {
	for i := p.pos + 1; i < len(p.tokens); i++ {
		if p.tokens[i].Kind == TokNewline {
			continue
		}
		return p.tokens[i].Kind == TokLParen
	}
	return false
}

func (p *Parser) parseRaw() RawDecl {
	var toks []Token
	depth := 0
	for p.peek().Kind != TokEOF {
		if p.isClassStart() && depth == 0 {
			break
		}
		t := p.advance()
		if t.Kind == TokLBrace {
			depth++
		} else if t.Kind == TokRBrace {
			depth--
			if depth <= 0 {
				toks = append(toks, t)
				break
			}
		}
		toks = append(toks, t)
	}
	return RawDecl{Tokens: toks}
}
