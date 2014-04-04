package main

import (
	"github.com/zhouhua015/goref/tests/pkg/shape"
	"fmt"
)

func doWork(s *shape.ShapeDecorator) {
	(*s).Draw()
	fmt.Printf("%s area %d\n", (*s).Description(), (*s).Area())
}

func main() {
	var s shape.ShapeDecorator
	s = shape.NewShape(shape.Tria, 5, 6)
	doWork(&s)
	s = shape.NewShape(shape.Para, 7, 8)
	doWork(&s)
	s = shape.NewShape(shape.Rect, 9, 10)
	doWork(&s)
	s = shape.NewShape(shape.Squa, 11)
	doWork(&s)

	fmt.Println("Done.")
}
