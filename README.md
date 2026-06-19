# gooop

A transpiler from Go++ (`.goo`) to Go. Adds classes with full OOP to Go.

## Install

```
go install github.com/vmaxer/gooop@latest
```

## Usage

```
gooop input.goo > output.go
cat input.goo | gooop > output.go
```

## Syntax

```go
package main

import "fmt"

abstract class Shape {
    Name string

    new(name string) {
        this.Name = name
    }

    abstract func Area() float64

    func Describe() string {
        return fmt.Sprintf("%s: area=%.2f", this.Name, this.Area())
    }
}

class Circle extends Shape implements ShapeInterface {
    Radius float64

    new(r float64) {
        super("Circle")
        this.Radius = r
    }

    func Area() float64 {
        return 3.14 * @Radius * @Radius
    }
}

data class Point {
    X, Y float64
}

class Vec includes Serializable {
    X, Y float64

    new(x, y float64) {
        @X = x
        @Y = y
    }

    func +(other *Vec) *Vec {
        return NewVec(@X+other.X, @Y+other.Y)
    }

    static func Zero() *Vec {
        return NewVec(0, 0)
    }
}
```

## Features

| Go++ | Go output |
|------|-----------|
| `class Name { ... }` | `type Name struct { ... }` + methods |
| `extends Parent` | Embedded struct |
| `implements Iface` | `var _ Iface = (*Name)(nil)` assertion |
| `includes A, B` | Multiple embedded structs (mixins) |
| `abstract class` | Struct + `NameInterface` + virtual dispatch |
| `abstract func F()` | Interface method (no body) |
| `data class` | Auto `New`, `String`, `Equal`, `Copy` |
| `new(params) { ... }` | `func NewName(params) *Name` |
| `Name(params) { ... }` | `func NewName(params) *Name` |
| `this` / `@Field` | Single-letter receiver |
| `super(args)` | Parent constructor call |
| `super.Method()` | Parent method call |
| `private Field` | Unexported (lowercase) field |
| `static func F()` | `func NameF()` package-level function |
| `func +(other) T` | `func Add(other) T` (operator methods) |

### Operator mapping

| Operator | Method |
|----------|--------|
| `+` `-` `*` `/` `%` | `Add` `Sub` `Mul` `Div` `Mod` |
| `==` `!=` `<` `<=` `>` `>=` | `Eq` `Neq` `Lt` `Lte` `Gt` `Gte` |
| `[]` `<<` `>>` `&` `\|` `^` | `At` `Shl` `Shr` `And` `Or` `Xor` |

All non-class Go code passes through unchanged.

## Test

```
go test ./...
```

## General

* Version: 1.0.0
* License: The Unlicense
