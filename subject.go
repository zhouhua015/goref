package main

import (
	"code.google.com/p/rog-go/exp/go/ast"
	"code.google.com/p/rog-go/exp/go/token"
	"code.google.com/p/rog-go/exp/go/types"
	"fmt"
	"strings"
)

type Subject interface {
	IsMe(ast.Expr, *ast.Package) bool
	DeclPos() token.Pos
	Toast()
}

type selectorSub struct {
	self *ast.SelectorExpr

	typ types.Type
	obj *ast.Object

	declPos token.Position

	recv types.Type // receiver _type_
}

func (subject *selectorSub) IsMe(e ast.Expr, pkg *ast.Package) (found bool) {
	switch n := e.(type) {
	default:
		found = false
	case *ast.SelectorExpr:
		if !subject.hasSameName(n.Sel) {
			return false
		}

		found = subject.hasSameRecvTyp(typeOf(n.X))
		// TODO to identify `promoted` field or method
	case *ast.Ident:
		// This case is suppose to handle package level
		// variable/struct/interface/function names.
		if subject.recv.Kind != ast.Pkg {
			// This is NOT a package level selector
			return false
		}

		// Compare ident with self.Sel only.
		if !subject.hasSameName(n) {
			return false
		}

		identTyp := typeOf(n)
		debugp("Ident type %v", identTyp)
		if identTyp.Kind == ast.Bad {
			return false
		}

		if subject.samePosWithSelf(n) {
			// it's the select subject itself, at declaration place, it will be
			// outputed by context by another way
			return false
		}

		pkgNameOfIdent := pkgNameOfPos(identTyp.Node.Pos())
		found = pkgNameOfIdent == pkgNameOfPos(subject.DeclPos())
	}

	return
}

func (subject *selectorSub) samePosWithSelf(n *ast.Ident) bool {
	identPos := types.FileSet.Position(n.Pos())

	debugp("selectorSub ident position %v", identPos)
	return identPos.IsValid() &&
		identPos.Filename == subject.declPos.Filename &&
		identPos.Offset == subject.declPos.Offset
}

func (subject *selectorSub) hasSameName(e *ast.Ident) bool {
	return e.Name == subject.self.Sel.Name
}

func (subject *selectorSub) hasSameRecvTyp(typ types.Type) bool {
	if typ.Kind == ast.Bad {
		return false
	}

	// local recv might got a empty package name
	regainPkgName(&typ, typ.Node.Pos())
	debugp("selectorSub.hasSameRecvTyp() matching recv type %v", typ)

	return sameDeclType(typ, subject.recv)
}

func (subject *selectorSub) DeclPos() token.Pos {
	return types.DeclPos(subject.obj)
}

func (subject *selectorSub) Toast() {
	regainPkgName(&subject.typ, subject.DeclPos())
	regainPkgName(&subject.recv, subject.recv.Node.Pos())
	subject.declPos = types.FileSet.Position(subject.DeclPos())
}

func (subject *selectorSub) String() string {
	s := fmt.Sprintf("selectorSub, self %v", subject.self)
	s += fmt.Sprintf(" typ: %v", subject.typ)
	s += fmt.Sprintf(" obj: %v", subject.obj)
	s += fmt.Sprintf(" recv typ: %v", subject.recv)
	s += fmt.Sprintf(" decl pos: %v", subject.declPos)

	return s
}

type identSub struct {
	self *ast.Ident

	typ types.Type
	obj *ast.Object

	declPos token.Position
}

func (subject *identSub) IsMe(e ast.Expr, pkg *ast.Package) (found bool) {
	switch n := e.(type) {
	default:
		found = false
	case *ast.Ident:
		if n.Name != subject.self.Name {
			return false
		}

		obj, ityp := types.ExprType(n, types.DefaultImporter)
		regainPkgName(&ityp, n.Pos())
		debugp("identSub.IsMe() matching node type %v", ityp)
		if !isIdenticalTyp(ityp, subject.typ) {
			return false
		}

		nPos := types.FileSet.Position(types.DeclPos(obj))
		debugp("identSub.IsMe()  n decl pos %v", nPos)
		found = subject.sameDeclPos(nPos)
	case *ast.SelectorExpr:
		// This case is supposed to handle package level
		// variable/struct/interface/function names
		//
		// Of SelectorExpr, only check Sel part,
		// n.X will be checked by case *ast.Ident branch
		if n.Sel.Name != subject.self.Name {
			return false
		}

		recvTyp := typeOf(n.X)
		debugp("identSub.IsMe() recv type %v", recvTyp)
		if recvTyp.Kind != ast.Pkg {
			return false
		}

		found = pkgNameOfPos(subject.DeclPos()) == typNodeName(recvTyp.Node)
	}

	return
}

func (subject *identSub) sameDeclPos(nPos token.Position) bool {
	return nPos.IsValid() &&
		nPos.Filename == subject.declPos.Filename &&
		nPos.Offset == subject.declPos.Offset
}

func (subject *identSub) DeclPos() token.Pos {
	return types.DeclPos(subject.obj)
}

func (subject *identSub) Toast() {
	regainPkgName(&subject.typ, types.DeclPos(subject.obj))

	subject.declPos = types.FileSet.Position(subject.DeclPos())
}

func (subject *identSub) String() string {
	s := fmt.Sprintf("identSub, self %v", subject.self)
	s += fmt.Sprintf(" typ: %v", subject.typ)
	s += fmt.Sprintf(" obj: %v", subject.obj)
	s += fmt.Sprintf(" decl pos: %v", subject.declPos)

	return s
}

func pkgNameOfPos(pos token.Pos) string {
	if pos == token.NoPos {
		return ""
	}

	position := types.FileSet.Position(pos)
	if !position.IsValid() {
		return ""
	}

	return pkgNameOfFile(position.Filename)
}

func regainPkgName(t *types.Type, declPos token.Pos) {
	if (*t).Pkg == "" {
		(*t).Pkg = pkgNameOfPos(declPos)
	}
}

func sameDeclType(t1, t2 types.Type) bool {
	switch {
	case t1.Kind == ast.Typ || t2.Kind == ast.Typ:
		// For composite literal, recv type can be ast.Typ or ast.Var, so if one
		// of the types.Type is ast.Typ, do not compare the Kind anymore
		if t1.Pkg != t2.Pkg {
			return false
		}

		return sameNodeName(t1.Node, t2.Node)
	case t1.Kind == ast.Pkg || t2.Kind == ast.Pkg:
		// Packages do NOT need to be compared by types.Type.Pkg, it doesn't
		// matter.
		if t1.Kind != t2.Kind {
			return false
		}

		return sameNodeName(t1.Node, t2.Node)
	}

	return isIdenticalTyp(t1, t2)
}

func isIdenticalTyp(t1, t2 types.Type) bool {
	if t1.Pkg != t2.Pkg {
		return false
	}

	if t1.Kind != t2.Kind {
		return false
	}

	return sameNodeName(t1.Node, t2.Node)
}

func sameNodeName(n1, n2 ast.Node) bool {
	return typNodeName(n1) == typNodeName(n2)
}

func typNodeName(n ast.Node) string {
	switch n := n.(type) {
	case *ast.ImportSpec:
		return strings.Trim(n.Path.Value, "\"")
	case *ast.Ident:
		return n.Name
	case *ast.StarExpr:
		return typNodeName(n.X)
	}
	return ""
}
