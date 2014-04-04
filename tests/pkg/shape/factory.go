//
// Don't you dare!
//
package shape

import "fmt"

type ShapeType int

const (
	Bad  ShapeType = iota
	Tria           // Triangle
	Para           // Parallelogram
	Rect           // Rectangle
	Squa           // Square
)

var ShapeNameTable = [...]string{
	Bad:  "Bad shape",
	Tria: "Triangle",
	Para: "Parallelogram",
	Rect: "Rectangle",
	Squa: "Square",
}

type ShapeDecorator struct {
	typ ShapeType
	Shape
}

func (d ShapeDecorator) String() string {
	return fmt.Sprintf("%d", d.typ)
}

func (d ShapeDecorator) Description() string {
	return ShapeNameTable[d.typ]
}

func NewShape(typ ShapeType, l ...int) (decorator ShapeDecorator) {
	switch typ {
	case Tria:
		decorator = ShapeDecorator{typ, NewTriangle(l[0], l[1])}
	case Para:
		decorator = ShapeDecorator{typ, NewParallelogram(l[0], l[1])}
	case Rect:
		decorator = ShapeDecorator{typ, NewRectangle(l[0], l[1])}
	case Squa:
		decorator = ShapeDecorator{typ, NewSquare(l[0])}
	}

	return decorator
}
