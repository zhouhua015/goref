//
// Don't tell me I can't use int as base and height!
//

package shape

import (
	"fmt"
)

type Shape interface {
	Draw()
	Area() int
}

type Triangle struct {
	Base   int
	Height int
}

func NewTriangle(b int, h int) *Triangle {
	return &Triangle{Base: b, Height: h}
}

func (t *Triangle) Draw() {
	fmt.Println("I'm triangle, picture me by yourself!")
}

func (t *Triangle) Area() int {
	return t.Base * t.Height / 2
}

type Parallelogram struct {
	Base   int
	Height int
}

func NewParallelogram(b int, h int) *Parallelogram {
	return &Parallelogram{b, h}
}

func (p *Parallelogram) Draw() {
	fmt.Println("I'm parallelogram, picture me by yourself!")
}

func (p *Parallelogram) Area() int {
	return p.Base * p.Height
}

type Rectangle struct {
	Parallelogram
}

func NewRectangle(b int, h int) *Rectangle {
	r := &Rectangle{}
	r.Base = b
	r.Height = h
	return r
}

func (r *Rectangle) Draw() {
	fmt.Println("I'm rectangle, picture me by yourself!")
}

type Square struct {
	Rectangle
}

func NewSquare(b int) *Square {
	s := &Square{}
	s.Base = b
	s.Height = b
	return s
}

func (s *Square) Draw() {
	fmt.Println("I'm square, picture me by yourself!")
}
