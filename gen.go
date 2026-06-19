package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var atFieldRe = regexp.MustCompile(`@(\w)`)

type Generator struct {
	buf strings.Builder
}

func NewGenerator() *Generator {
	return &Generator{}
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
	for _, t := range d.Tokens {
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

	g.buf.WriteString(fmt.Sprintf("\ntype %s struct {\n", c.Name))
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
	g.buf.WriteString(fmt.Sprintf("\nfunc New%s(%s) *%s {\n", c.Name, params, c.Name))
	g.buf.WriteString(fmt.Sprintf("\t%s := &%s{}\n", recv, c.Name))

	body := tokensToString(c.Ctor.Body)
	body = rewriteSelf(body, recv)
	body = rewritePrivateFields(body, recv, c.Fields)

	if c.Parent != "" {
		body = transformSuper(body, c.Parent, recv)
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		g.buf.WriteString("\t" + trimmed + "\n")
	}

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
	g.buf.WriteString(fmt.Sprintf("\nfunc %s%s(%s)%s {\n", className, m.Name, params, results))
	body := tokensToString(m.Body)
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		g.buf.WriteString("\t" + trimmed + "\n")
	}
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

	g.buf.WriteString(fmt.Sprintf("\nfunc (%s *%s) %s%s(%s)%s {\n",
		recv, className, m.Name, typeParams, params, results))

	body := tokensToString(m.Body)
	body = rewriteSelf(body, recv)
	body = rewritePrivateFields(body, recv, fields)
	if parent != "" {
		body = rewriteSuperMethod(body, parent, recv)
	}
	body = rewriteAbstractCalls(body, recv, abstractNames)

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		g.buf.WriteString("\t" + trimmed + "\n")
	}
	g.buf.WriteString("}\n")
}

func transformSuper(body, parent, recv string) string {
	for {
		idx := strings.Index(body, "super(")
		if idx < 0 {
			break
		}
		end := idx + 6
		depth := 1
		for end < len(body) && depth > 0 {
			if body[end] == '(' {
				depth++
			} else if body[end] == ')' {
				depth--
			}
			end++
		}
		args := body[idx+6 : end-1]
		replacement := fmt.Sprintf("%s.%s = *New%s(%s)", recv, parent, parent, args)
		body = body[:idx] + replacement + body[end:]
	}
	return body
}

var superMethodRe = regexp.MustCompile(`super\.(\w+)`)

func rewriteSuperMethod(body, parent, recv string) string {
	return superMethodRe.ReplaceAllString(body, recv+"."+parent+".${1}")
}

func rewriteSelf(body, recv string) string {
	body = atFieldRe.ReplaceAllString(body, recv+".${1}")
	body = strings.ReplaceAll(body, "this.", recv+".")
	body = strings.ReplaceAll(body, "this", recv)
	return body
}

func rewritePrivateFields(body, recv string, fields []Field) string {
	for _, f := range fields {
		if !f.Private {
			continue
		}
		for _, n := range f.Names {
			lowered := unexport(n)
			if lowered != n {
				body = strings.ReplaceAll(body, recv+"."+n, recv+"."+lowered)
			}
		}
	}
	return body
}

func rewriteAbstractCalls(body, recv string, abstractNames []string) string {
	for _, name := range abstractNames {
		old := recv + "." + name + "("
		new := recv + "._iface." + name + "("
		body = strings.ReplaceAll(body, old, new)
	}
	return body
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
