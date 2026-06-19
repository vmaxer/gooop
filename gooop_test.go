package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func transpile(input string) string {
	tokens := NewLexer(input).Tokenize()
	file := NewParser(tokens).Parse()
	return NewGenerator().Generate(file)
}

func TestPassthrough(t *testing.T) {
	input := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	out := transpile(input)
	if !strings.Contains(out, `fmt.Println("hello")`) {
		t.Fatalf("passthrough failed:\n%s", out)
	}
}

func TestSimpleClass(t *testing.T) {
	input := `package main

class Point {
	X int
	Y int

	new(x int, y int) {
		this.X = x
		this.Y = y
	}

	func Add(other *Point) *Point {
		return NewPoint(this.X+other.X, this.Y+other.Y)
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "type Point struct {") {
		t.Fatal("missing struct")
	}
	if !strings.Contains(out, "func NewPoint(x int, y int) *Point {") {
		t.Fatal("missing constructor")
	}
	if !strings.Contains(out, "func (p *Point) Add(other *Point) *Point {") {
		t.Fatal("missing method")
	}
	if strings.Contains(out, "this") {
		t.Fatal("this not replaced")
	}
}

func TestInheritance(t *testing.T) {
	input := `package main

class Base {
	ID int

	new(id int) {
		this.ID = id
	}
}

class Child extends Base {
	Name string

	new(id int, name string) {
		super(id)
		this.Name = name
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "type Child struct {\n\tBase\n") {
		t.Fatalf("missing embedded parent:\n%s", out)
	}
	if !strings.Contains(out, "c.Base = *NewBase(id)") {
		t.Fatalf("super not transformed:\n%s", out)
	}
}

func TestMultipleFields(t *testing.T) {
	input := `package main

class Vec3 {
	X, Y, Z float64

	new(x, y, z float64) {
		this.X = x
		this.Y = y
		this.Z = z
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "X, Y, Z float64") {
		t.Fatalf("multi-field failed:\n%s", out)
	}
}

func TestAtSyntax(t *testing.T) {
	input := `package main

class Greeter {
	Name string

	new(name string) {
		@Name = name
	}

	func Hello() string {
		return "hi " + @Name
	}
}
`
	out := transpile(input)
	if strings.Contains(out, "@") {
		t.Fatalf("@ not replaced:\n%s", out)
	}
	if !strings.Contains(out, "g.Name = name") {
		t.Fatalf("@ in constructor failed:\n%s", out)
	}
	if !strings.Contains(out, `"hi " + g.Name`) {
		t.Fatalf("@ in method failed:\n%s", out)
	}
}

func TestNamedConstructor(t *testing.T) {
	input := `package main

class Rect {
	W, H float64

	Rect(w, h float64) {
		this.W = w
		this.H = h
	}

	func Area() float64 {
		return this.W * this.H
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "func NewRect(w, h float64) *Rect {") {
		t.Fatalf("named constructor failed:\n%s", out)
	}
	if strings.Contains(out, "this") {
		t.Fatal("this not replaced")
	}
}

func TestPrivateFields(t *testing.T) {
	input := `package main

class Foo {
	private ID int
	private URL string
	Name string

	new(id int) {
		this.ID = id
		this.URL = "http"
		this.Name = "x"
	}

	func GetID() int {
		return this.ID
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "\tid int\n") {
		t.Fatalf("private ID not lowered:\n%s", out)
	}
	if !strings.Contains(out, "\turl string\n") {
		t.Fatalf("private URL not lowered:\n%s", out)
	}
	if !strings.Contains(out, "f.id = id") {
		t.Fatalf("private field access not rewritten:\n%s", out)
	}
}

func TestStaticMethod(t *testing.T) {
	input := `package main

class Math {
	static func Square(x int) int {
		return x * x
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "func MathSquare(x int) int {") {
		t.Fatalf("static method failed:\n%s", out)
	}
}

func TestSuperMethod(t *testing.T) {
	input := `package main

class Parent {
	func Hello() string {
		return "hello"
	}
}

class Child extends Parent {
	func Hello() string {
		return super.Hello() + " world"
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "c.Parent.Hello()") {
		t.Fatalf("super.Method not rewritten:\n%s", out)
	}
}

func TestDataClass(t *testing.T) {
	input := `package main

data class Pair {
	A, B int
}
`
	out := transpile(input)
	if !strings.Contains(out, "func NewPair(a int, b int) *Pair {") {
		t.Fatalf("data constructor failed:\n%s", out)
	}
	if !strings.Contains(out, "func (p *Pair) Equal(other *Pair) bool {") {
		t.Fatalf("data Equal missing:\n%s", out)
	}
	if !strings.Contains(out, "func (p *Pair) Copy() *Pair {") {
		t.Fatalf("data Copy missing:\n%s", out)
	}
	if !strings.Contains(out, "func (p *Pair) String() string {") {
		t.Fatalf("data String missing:\n%s", out)
	}
}

func TestOperatorOverload(t *testing.T) {
	input := `package main

class Num {
	V int

	new(v int) {
		this.V = v
	}

	func +(other *Num) *Num {
		return NewNum(this.V + other.V)
	}

	func ==(other *Num) bool {
		return this.V == other.V
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "func (n *Num) Add(other *Num) *Num {") {
		t.Fatalf("operator + not mapped to Add:\n%s", out)
	}
	if !strings.Contains(out, "func (n *Num) Eq(other *Num) bool {") {
		t.Fatalf("operator == not mapped to Eq:\n%s", out)
	}
}

func TestImplements(t *testing.T) {
	input := `package main

class Dog implements Stringer {
	Name string

	new(name string) {
		this.Name = name
	}

	func String() string {
		return this.Name
	}
}
`
	out := transpile(input)
	if !strings.Contains(out, "var _ Stringer = (*Dog)(nil)") {
		t.Fatalf("implements assertion missing:\n%s", out)
	}
}

func TestIncludes(t *testing.T) {
	input := `package main

class Mixin {
	Tag string
}

class Thing includes Mixin {
	Name string
}
`
	out := transpile(input)
	if !strings.Contains(out, "type Thing struct {\n\tMixin\n\tName string\n}") {
		t.Fatalf("includes embedding failed:\n%s", out)
	}
}

func TestExamplesCompile(t *testing.T) {
	examples, _ := filepath.Glob("examples/*.goo")
	if len(examples) == 0 {
		t.Skip("no examples found")
	}
	for _, ex := range examples {
		t.Run(filepath.Base(ex), func(t *testing.T) {
			src, _ := os.ReadFile(ex)
			out := transpile(string(src))
			tmp := t.TempDir()
			goFile := filepath.Join(tmp, "main.go")
			os.WriteFile(goFile, []byte(out), 0644)
			cmd := exec.Command("go", "run", goFile)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("compile failed for %s:\n%s\n\nGenerated:\n%s", ex, output, out)
			}
		})
	}
}
