package main

import (
	"fmt"
	"strings"
	"unicode"
)

type Generator struct {
	buf  strings.Builder
	file string
}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) SetFile(name string) {
	g.file = name
}

func (g *Generator) lineDirective(line int) {
	if g.file != "" && line > 0 {
		fmt.Fprintf(&g.buf, "//line %s:%d\n", g.file, line)
	}
}

func (g *Generator) Generate(f *File) string {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case RawDecl:
			g.emitRaw(d)
		case ClassDecl:
			g.emitClass(d)
		}
	}
	return g.buf.String()
}

func (g *Generator) emitRaw(d RawDecl) {
	anchored := false
	for _, t := range d.Tokens {
		if !anchored && t.Kind != TokNewline && t.Line > 0 {
			g.lineDirective(t.Line)
			anchored = true
		}
		g.buf.WriteString(t.PreWS)
		g.buf.WriteString(t.Value)
	}
}

func (g *Generator) emitClass(c ClassDecl) {
	recv := chooseReceiver(c)

	abstractNames := abstractMethodNames(c)

	if c.IsAbstract && len(abstractNames) > 0 {
		g.emitAbstractInterface(c)
	}

	g.buf.WriteString("\n")
	g.lineDirective(c.Line)
	g.buf.WriteString(fmt.Sprintf("type %s struct {\n", c.Name))
	if c.Parent != "" {
		g.buf.WriteString(fmt.Sprintf("\t%s\n", c.Parent))
	}
	for _, inc := range c.Includes {
		g.buf.WriteString(fmt.Sprintf("\t%s\n", inc))
	}
	if c.IsAbstract && len(abstractNames) > 0 {
		g.buf.WriteString(fmt.Sprintf("\t_iface %sInterface\n", c.Name))
	}
	for _, f := range c.Fields {
		names := fieldNames(f)
		g.buf.WriteString(fmt.Sprintf("\t%s %s\n", names, tokensToString(f.Type)))
	}
	g.buf.WriteString("}\n")

	for _, iface := range c.Implements {
		g.buf.WriteString(fmt.Sprintf("\nvar _ %s = (*%s)(nil)\n", iface, c.Name))
	}

	if c.Ctor != nil {
		g.emitConstructor(c, recv)
	} else if c.IsData {
		g.emitDataConstructor(c, recv)
	}

	if c.IsData {
		g.emitDataString(c, recv)
		g.emitDataEqual(c, recv)
		g.emitDataCopy(c, recv)
	}

	for _, m := range c.Methods {
		if m.IsAbstract {
			continue
		}
		if m.IsStatic {
			g.emitStaticMethod(c.Name, m)
		} else {
			g.emitMethod(c.Name, recv, c.Parent, m, c.Fields, abstractNames)
		}
	}
}

func (g *Generator) emitAbstractInterface(c ClassDecl) {
	g.buf.WriteString(fmt.Sprintf("\ntype %sInterface interface {\n", c.Name))
	for _, m := range c.Methods {
		if m.IsStatic {
			continue
		}
		params := tokensToString(m.Params)
		results := tokensToString(m.Results)
		if results != "" {
			results = " " + results
		}
		g.buf.WriteString(fmt.Sprintf("\t%s(%s)%s\n", m.Name, params, results))
	}
	g.buf.WriteString("}\n")
}

func (g *Generator) emitConstructor(c ClassDecl, recv string) {
	params := tokensToString(c.Ctor.Params)
	g.buf.WriteString("\n")
	g.lineDirective(c.Ctor.Line)
	g.buf.WriteString(fmt.Sprintf("func New%s(%s) *%s {\n", c.Name, params, c.Name))
	g.buf.WriteString(fmt.Sprintf("\t%s := &%s{}\n", recv, c.Name))

	body := rewriteBody(c.Ctor.Body, recv, c.Parent, c.Fields, nil)
	g.writeBodyTokens(body)

	parentAbstract := findParentAbstract(c)
	if parentAbstract != "" {
		g.buf.WriteString(fmt.Sprintf("\t%s._iface = %s\n", recv, recv))
	}

	g.buf.WriteString(fmt.Sprintf("\treturn %s\n", recv))
	g.buf.WriteString("}\n")
}

func (g *Generator) emitDataConstructor(c ClassDecl, recv string) {
	var params []string
	var assignments []string
	for _, f := range c.Fields {
		typStr := tokensToString(f.Type)
		for _, n := range f.Names {
			p := unexport(n) + " " + typStr
			params = append(params, p)
			assignments = append(assignments, fmt.Sprintf("\t%s.%s = %s\n", recv, exportName(n, f.Private), unexport(n)))
		}
	}
	g.buf.WriteString(fmt.Sprintf("\nfunc New%s(%s) *%s {\n", c.Name, strings.Join(params, ", "), c.Name))
	g.buf.WriteString(fmt.Sprintf("\t%s := &%s{}\n", recv, c.Name))
	for _, a := range assignments {
		g.buf.WriteString(a)
	}
	g.buf.WriteString(fmt.Sprintf("\treturn %s\n", recv))
	g.buf.WriteString("}\n")
}

func (g *Generator) emitDataString(c ClassDecl, recv string) {
	g.buf.WriteString(fmt.Sprintf("\nfunc (%s *%s) String() string {\n", recv, c.Name))
	var parts []string
	var args []string
	for _, f := range c.Fields {
		for _, n := range f.Names {
			parts = append(parts, exportName(n, f.Private)+": %v")
			args = append(args, recv+"."+exportName(n, f.Private))
		}
	}
	format := c.Name + "{" + strings.Join(parts, ", ") + "}"
	g.buf.WriteString(fmt.Sprintf("\treturn fmt.Sprintf(\"%s\", %s)\n", format, strings.Join(args, ", ")))
	g.buf.WriteString("}\n")
}

func (g *Generator) emitDataEqual(c ClassDecl, recv string) {
	g.buf.WriteString(fmt.Sprintf("\nfunc (%s *%s) Equal(other *%s) bool {\n", recv, c.Name, c.Name))
	var conds []string
	for _, f := range c.Fields {
		for _, n := range f.Names {
			fn := exportName(n, f.Private)
			conds = append(conds, fmt.Sprintf("%s.%s == other.%s", recv, fn, fn))
		}
	}
	if len(conds) == 0 {
		g.buf.WriteString("\treturn true\n")
	} else {
		g.buf.WriteString("\treturn " + strings.Join(conds, " && ") + "\n")
	}
	g.buf.WriteString("}\n")
}

func (g *Generator) emitDataCopy(c ClassDecl, recv string) {
	g.buf.WriteString(fmt.Sprintf("\nfunc (%s *%s) Copy() *%s {\n", recv, c.Name, c.Name))
	g.buf.WriteString(fmt.Sprintf("\tcopy := *%s\n", recv))
	g.buf.WriteString("\treturn &copy\n")
	g.buf.WriteString("}\n")
}

func (g *Generator) emitStaticMethod(className string, m Method) {
	params := tokensToString(m.Params)
	results := tokensToString(m.Results)
	if results != "" {
		results = " " + results
	}
	g.buf.WriteString("\n")
	g.lineDirective(m.Line)
	g.buf.WriteString(fmt.Sprintf("func %s%s(%s)%s {\n", className, m.Name, params, results))
	g.writeBodyTokens(m.Body)
	g.buf.WriteString("}\n")
}

func (g *Generator) emitMethod(className, recv, parent string, m Method, fields []Field, abstractNames []string) {
	typeParams := ""
	if len(m.TypeParams) > 0 {
		typeParams = "[" + tokensToString(m.TypeParams) + "]"
	}
	params := tokensToString(m.Params)
	results := tokensToString(m.Results)
	if results != "" {
		results = " " + results
	}

	g.buf.WriteString("\n")
	g.lineDirective(m.Line)
	g.buf.WriteString(fmt.Sprintf("func (%s *%s) %s%s(%s)%s {\n",
		recv, className, m.Name, typeParams, params, results))

	body := rewriteBody(m.Body, recv, parent, fields, abstractNames)
	g.writeBodyTokens(body)
	g.buf.WriteString("}\n")
}

func (g *Generator) writeBodyTokens(toks []Token) {
	var line []Token
	flush := func() {
		s := strings.TrimSpace(tokensToString(line))
		if s != "" {
			g.lineDirective(firstLineOf(line))
			g.buf.WriteString("\t" + s + "\n")
		}
		line = line[:0]
	}
	for _, t := range toks {
		if t.Kind == TokNewline {
			flush()
			continue
		}
		line = append(line, t)
	}
	flush()
}

func firstLineOf(toks []Token) int {
	for _, t := range toks {
		if t.Line > 0 {
			return t.Line
		}
	}
	return 0
}

func rewriteBody(toks []Token, recv, parent string, fields []Field, abstractNames []string) []Token {
	priv := privateFieldMap(fields)
	abs := nameSet(abstractNames)
	var out []Token
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t.Kind == TokString || t.Kind == TokRune || isComment(t):
			out = append(out, t)
		case t.Kind == TokOp && t.Value == "@" && i+1 < len(toks) && toks[i+1].Kind == TokIdent:
			name := toks[i+1].Value
			out = append(out,
				Token{Kind: TokIdent, Value: recv, PreWS: t.PreWS, Line: t.Line},
				Token{Kind: TokDot, Value: "."},
				Token{Kind: TokIdent, Value: mapName(name, priv)})
			i++
		case t.Kind == TokIdent && t.Value == "this" && i+2 < len(toks) && toks[i+1].Kind == TokDot && toks[i+2].Kind == TokIdent:
			name := toks[i+2].Value
			out = append(out, Token{Kind: TokIdent, Value: recv, PreWS: t.PreWS, Line: t.Line}, Token{Kind: TokDot, Value: "."})
			if abs[name] && i+3 < len(toks) && toks[i+3].Kind == TokLParen {
				out = append(out, Token{Kind: TokIdent, Value: "_iface"}, Token{Kind: TokDot, Value: "."})
			}
			out = append(out, Token{Kind: TokIdent, Value: mapName(name, priv)})
			i += 2
		case t.Kind == TokIdent && t.Value == "this":
			out = append(out, Token{Kind: TokIdent, Value: recv, PreWS: t.PreWS, Line: t.Line})
		case t.Kind == TokIdent && t.Value == "super" && parent != "" && i+1 < len(toks) && toks[i+1].Kind == TokDot:
			out = append(out,
				Token{Kind: TokIdent, Value: recv, PreWS: t.PreWS, Line: t.Line},
				Token{Kind: TokDot, Value: "."},
				Token{Kind: TokIdent, Value: parent})
		case t.Kind == TokIdent && t.Value == "super" && parent != "" && i+1 < len(toks) && toks[i+1].Kind == TokLParen:
			out = append(out,
				Token{Kind: TokIdent, Value: recv, PreWS: t.PreWS, Line: t.Line},
				Token{Kind: TokDot, Value: "."},
				Token{Kind: TokIdent, Value: parent},
				Token{Kind: TokOp, Value: "=", PreWS: " "},
				Token{Kind: TokOp, Value: "*", PreWS: " "},
				Token{Kind: TokIdent, Value: "New" + parent})
		default:
			out = append(out, t)
		}
	}
	return out
}

func isComment(t Token) bool {
	return t.Kind == TokOp && (strings.HasPrefix(t.Value, "//") || strings.HasPrefix(t.Value, "/*"))
}

func mapName(name string, priv map[string]string) string {
	if mapped, ok := priv[name]; ok {
		return mapped
	}
	return name
}

func privateFieldMap(fields []Field) map[string]string {
	m := map[string]string{}
	for _, f := range fields {
		if !f.Private {
			continue
		}
		for _, n := range f.Names {
			m[n] = unexport(n)
		}
	}
	return m
}

func nameSet(names []string) map[string]bool {
	m := map[string]bool{}
	for _, n := range names {
		m[n] = true
	}
	return m
}

func abstractMethodNames(c ClassDecl) []string {
	var names []string
	for _, m := range c.Methods {
		if m.IsAbstract {
			names = append(names, m.Name)
		}
	}
	return names
}

func chooseReceiver(c ClassDecl) string {
	used := collectBodyIdents(c)
	candidate := strings.ToLower(c.Name[:1])
	if !used[candidate] {
		return candidate
	}
	for i := 2; i <= len(c.Name); i++ {
		candidate = strings.ToLower(c.Name[:i])
		if !used[candidate] {
			return candidate
		}
	}
	return strings.ToLower(c.Name)
}

func collectBodyIdents(c ClassDecl) map[string]bool {
	idents := map[string]bool{}
	var allTokens []Token
	if c.Ctor != nil {
		allTokens = append(allTokens, c.Ctor.Body...)
	}
	for _, m := range c.Methods {
		allTokens = append(allTokens, m.Body...)
	}
	for i, t := range allTokens {
		if t.Kind != TokIdent {
			continue
		}
		if t.Value == "this" || t.Value == "super" {
			continue
		}
		for j := i + 1; j < len(allTokens); j++ {
			next := allTokens[j]
			if next.Kind == TokNewline || next.Kind == TokEOF {
				break
			}
			if next.Kind == TokOp && (next.Value == ":=" || next.Value == "=") {
				idents[t.Value] = true
				break
			}
			if next.Kind != TokIdent {
				break
			}
		}
		if i > 0 {
			prev := allTokens[i-1]
			if prev.Kind == TokComma || (prev.Kind == TokIdent && prev.Value == "range") {
				idents[t.Value] = true
			}
		}
	}
	return idents
}

func findParentAbstract(c ClassDecl) string {
	if c.Parent == "" {
		return ""
	}
	expected := c.Parent + "Interface"
	for _, iface := range c.Implements {
		if iface == expected {
			return c.Parent
		}
	}
	return ""
}

func tokensToString(toks []Token) string {
	var b strings.Builder
	for i, t := range toks {
		if i > 0 {
			b.WriteString(t.PreWS)
		}
		b.WriteString(t.Value)
	}
	return b.String()
}

func fieldNames(f Field) string {
	exported := make([]string, len(f.Names))
	for i, n := range f.Names {
		exported[i] = exportName(n, f.Private)
	}
	return strings.Join(exported, ", ")
}

func exportName(name string, private bool) string {
	if private {
		return unexport(name)
	}
	return name
}

func unexport(name string) string {
	if len(name) == 0 {
		return name
	}
	r := []rune(name)
	allUpper := true
	for _, c := range r {
		if !unicode.IsUpper(c) && !unicode.IsDigit(c) {
			allUpper = false
			break
		}
	}
	if allUpper {
		for i := range r {
			r[i] = unicode.ToLower(r[i])
		}
		return string(r)
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}
