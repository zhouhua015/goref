package sbj

import (
	"code.google.com/p/rog-go/exp/go/ast"
	"code.google.com/p/rog-go/exp/go/parser"
	"code.google.com/p/rog-go/exp/go/token"
	"code.google.com/p/rog-go/exp/go/types"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

type Subject interface {
	IsMe(ast.Expr, *ast.Package) bool
	ObjDeclPos() token.Pos
	Toast()
}

type SelectorSub struct {
	Self *ast.SelectorExpr

	Typ types.Type
	Obj *ast.Object

	DeclPos token.Position

	Recv types.Type // receiver _type_
}

func (subject *SelectorSub) IsMe(e ast.Expr, pkg *ast.Package) (found bool) {
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
		if subject.Recv.Kind != ast.Pkg {
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
		found = pkgNameOfIdent == pkgNameOfPos(subject.ObjDeclPos())
	}

	return
}

func (subject *SelectorSub) samePosWithSelf(n *ast.Ident) bool {
	identPos := types.FileSet.Position(n.Pos())

	debugp("SelectorSub ident position %v", identPos)
	return identPos.IsValid() &&
		identPos.Filename == subject.DeclPos.Filename &&
		identPos.Offset == subject.DeclPos.Offset
}

func (subject *SelectorSub) hasSameName(e *ast.Ident) bool {
	return e.Name == subject.Self.Sel.Name
}

func (subject *SelectorSub) hasSameRecvTyp(typ types.Type) bool {
	if typ.Kind == ast.Bad {
		return false
	}

	// local recv might got a empty package name
	regainPkgName(&typ, typ.Node.Pos())
	debugp("SelectorSub.hasSameRecvTyp() matching recv type %v", typ)

	return sameDeclType(typ, subject.Recv)
}

func (subject *SelectorSub) ObjDeclPos() token.Pos {
	return types.DeclPos(subject.Obj)
}

func (subject *SelectorSub) Toast() {
	regainPkgName(&subject.Typ, subject.ObjDeclPos())
	regainPkgName(&subject.Recv, subject.Recv.Node.Pos())
	subject.DeclPos = types.FileSet.Position(subject.ObjDeclPos())
}

func (subject *SelectorSub) String() string {
	s := fmt.Sprintf("SelectorSub, self %v", subject.Self)
	s += fmt.Sprintf(" typ: %v", subject.Typ)
	s += fmt.Sprintf(" obj: %v", subject.Obj)
	s += fmt.Sprintf(" recv typ: %v", subject.Recv)
	s += fmt.Sprintf(" decl pos: %v", subject.DeclPos)

	return s
}

type IdentSub struct {
	Self *ast.Ident

	Typ types.Type
	Obj *ast.Object

	DeclPos token.Position
}

func (subject *IdentSub) IsMe(e ast.Expr, pkg *ast.Package) (found bool) {
	switch n := e.(type) {
	default:
		found = false
	case *ast.Ident:
		if n.Name != subject.Self.Name {
			return false
		}

		obj, ityp := types.ExprType(n, types.DefaultImporter)
		regainPkgName(&ityp, n.Pos())
		debugp("IdentSub.IsMe() matching node type %v", ityp)
		if !isIdenticalTyp(ityp, subject.Typ) {
			return false
		}

		nPos := types.FileSet.Position(types.DeclPos(obj))
		debugp("IdentSub.IsMe()  n decl pos %v", nPos)
		found = subject.sameDeclPos(nPos)
	case *ast.SelectorExpr:
		// This case is supposed to handle package level
		// variable/struct/interface/function names
		//
		// Of SelectorExpr, only check Sel part,
		// n.X will be checked by case *ast.Ident branch
		if n.Sel.Name != subject.Self.Name {
			return false
		}

		recvTyp := typeOf(n.X)
		debugp("IdentSub.IsMe() recv type %v", recvTyp)
		if recvTyp.Kind != ast.Pkg {
			return false
		}

		found = pkgNameOfPos(subject.ObjDeclPos()) == typNodeName(recvTyp.Node)
	}

	return
}

func (subject *IdentSub) sameDeclPos(nPos token.Position) bool {
	return nPos.IsValid() &&
		nPos.Filename == subject.DeclPos.Filename &&
		nPos.Offset == subject.DeclPos.Offset
}

func (subject *IdentSub) ObjDeclPos() token.Pos {
	return types.DeclPos(subject.Obj)
}

func (subject *IdentSub) Toast() {
	regainPkgName(&subject.Typ, types.DeclPos(subject.Obj))

	subject.DeclPos = types.FileSet.Position(subject.ObjDeclPos())
}

func (subject *IdentSub) String() string {
	s := fmt.Sprintf("IdentSub, self %v", subject.Self)
	s += fmt.Sprintf(" typ: %v", subject.Typ)
	s += fmt.Sprintf(" obj: %v", subject.Obj)
	s += fmt.Sprintf(" decl pos: %v", subject.DeclPos)

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

func pkgNameOfFile(filename string) string {
	fset := token.NewFileSet()
	prog, _ := parser.ParseFile(fset, filename, nil, parser.PackageClauseOnly, nil)
	if prog == nil {
		return ""
	}

	dir := filepath.Dir(filename)
	if filepath.Base(dir) != prog.Name.Name {
		return prog.Name.Name
	}

	for _, p := range types.GoPath {
		p = p + "/"
		if strings.HasPrefix(dir, p) {
			return strings.TrimPrefix(dir, p)
		}
	}

	return ""
}

func typeOf(n ast.Expr) types.Type {
	_, t := types.ExprType(n, types.DefaultImporter)
	if t.Kind == ast.Bad || t.Node == nil {
		return types.Type{Kind: ast.Bad}
	}

	return t
}

var Debug bool = false

func debugp(f string, a ...interface{}) {
	if Debug {
		log.Printf(f, a...)
	}
}
