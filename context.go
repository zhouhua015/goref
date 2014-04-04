package main

import (
	"bytes"
	"code.google.com/p/rog-go/exp/go/ast"
	"code.google.com/p/rog-go/exp/go/parser"
	"code.google.com/p/rog-go/exp/go/printer"
	"code.google.com/p/rog-go/exp/go/token"
	"code.google.com/p/rog-go/exp/go/types"
	"container/list"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var NoMorePkgFiles = errors.New("no more package files found")

type Context struct {
	FileName  string
	SearchPos int
	Path      string
	LocalPkg  *ast.Package

	Scope   ast.Node // search scope
	Subject Subject

	RefPrinter func(ast.Expr)
}

func NewContext(source string, pos int, path string) *Context {
	return &Context{FileName: source, SearchPos: pos, Path: path}
}

func (ctx *Context) String() string {
	s := fmt.Sprintf("subject: %v, ", ctx.Subject)
	if ctx.LocalPkg != nil {
		s += fmt.Sprintf("local package name: %v, ", ctx.LocalPkg.Name)
	}
	s += fmt.Sprintf("search scope: %T %v, ", ctx.Scope, ctx.Scope)

	return s
}

func (ctx *Context) ParseSubject() error {
	src, err := ioutil.ReadFile(ctx.FileName)
	if err != nil {
		return errorGenerator("cannot read %s: %v", ctx.FileName, err)
	}

	pkgScope := ast.NewScope(parser.Universe)
	f, err := parser.ParseFile(types.FileSet, ctx.FileName, src, 0, pkgScope)
	if f == nil {
		return errorGenerator("cannot parse %s: %v", ctx.FileName, err)
	}

	identifier, fdecl, err := findIdentifier(f, ctx.SearchPos)
	if err != nil {
		return err
	}
	debugp("target: %T %v\n", identifier, identifier)

	// add local package
	pkg, err := parseLocalPackage(ctx.FileName, f, pkgScope)
	if pkg == nil && err != NoMorePkgFiles {
		fmt.Fprint(os.Stderr, fmt.Sprintf("parseLocalPackage error: %v\n", err))
	}
	ctx.LocalPkg = pkg

	// and try again...
	obj, typ := types.ExprType(identifier, types.DefaultImporter)
	if obj == nil || typ.Kind == ast.Bad {
		return errorGenerator("identifier with nil object, %T %v\n", identifier, identifier)
	}
	debugp("source type %v, type node %s", typ, pretty(typ.Node))

	err = ctx.buildSubject(identifier, f, typ, obj)
	if err != nil {
		return err
	}

	// try to get scope
	declPos := ctx.Subject.DeclPos()
	if fdecl != nil && fdecl.Pos() <= declPos && declPos <= fdecl.End() {
		ctx.Scope = fdecl
	}

	// update subject package name, by the declaration position.
	// The types.Expr() might get the wrong package name if
	// it's parsing local package
	ctx.Subject.Toast()

	debugp("context after subject parsed %v", ctx)
	return nil
}

func (ctx *Context) buildSubject(identifier ast.Expr, f *ast.File, typ types.Type, obj *ast.Object) error {
	// try to get recv if the ident is field/function declaration
	switch t := identifier.(type) {
	case *ast.SelectorExpr:
		debugp("subject recv type: %v", typeOf(t.X))
		ctx.Subject = &selectorSub{self: t, typ: typ, obj: obj, recv: typeOf(t.X)}
	case *ast.Ident:
		// *ast.Ident might be
		//    1. method decl of struct
		//    2. field decl of struct
		// find recv for these 2 types
		switch d := obj.Decl.(type) {
		case *ast.FuncDecl:
			debugp("source object decl is a FuncDecl, name: %s", d.Name.Name)
			if d.Recv != nil {
				if len(d.Recv.List) != 1 {
					return errorGenerator("Invalid ident")
				}
				debugp("source object decl is a FuncDecl, recv: %v", d.Recv.List[0])

				/*
				 * TODO take care of the difference of
				 *        pointer-type method  v.s.  value-type method
				 * */
				e := &ast.SelectorExpr{X: d.Recv.List[0].Type, Sel: t}
				debugp("subject recv type: %v", typeOf(d.Recv.List[0].Type))
				ctx.Subject = &selectorSub{self: e,
					typ:  typ,
					obj:  obj,
					recv: typeOf(d.Recv.List[0].Type)}
			}
		case *ast.Field:
			debugp("source object decl is a Field, name: %v", d.Names)
			owner := findFieldOuter(d, f)
			if owner == nil {
				ctx.Subject = &identSub{self: t, typ: typ, obj: obj}
				break
			}
			e := &ast.SelectorExpr{X: owner, Sel: t}
			debugp("subject recv type: %v", typeOf(owner))
			ctx.Subject = &selectorSub{self: e, typ: typ, obj: obj, recv: typeOf(owner)}
		}

		// function without any recv have to be a ident subject
		if ctx.Subject == nil {
			ctx.Subject = &identSub{self: t, typ: typ, obj: obj}
		}
	}

	if ctx.Subject == nil {
		return errorGenerator("failed to parse subject")
	}
	debugp("subject %v", ctx.Subject)

	return nil
}

type inspector func(ast.Node) bool

func (ctx *Context) Scan(n ast.Node, pkg *ast.Package) {
	var visit inspector
	ok := true
	inCompositeLit := false
	compositeLitTypStack := list.New()
	visit = func(n ast.Node) bool {
		if !ok {
			return false
		}
		switch n := n.(type) {
		case *ast.ImportSpec:
			// If the file imports a package to ".", abort
			// because we don't support that (yet).
			if n.Name != nil && n.Name.Name == "." {
				//TODO ctxt.logf(n.Pos(), "import to . not supported")
				ok = false
				return false
			}
			return true
		case *ast.Ident:
			ok = ctx.visitExpr(n, pkg)
			return false
		case *ast.KeyValueExpr:
			// don't try to resolve the key part of a key-value
			// because it might be a map key which doesn't
			// need resolving, and we can't tell without being
			// complicated with types.
			ast.Inspect(n.Value, visit)
			return false
		case *ast.SelectorExpr:
			// don't visit expr of selector if we're in composite literal,
			// it has been visited in visitCompositeLit() function
			if !inCompositeLit {
				ast.Inspect(n.X, visit)
			}
			ok = ctx.visitExpr(n, pkg)
			return false
		case *ast.CompositeLit:
			inCompositeLit = true
			visitCompositeLit(n, compositeLitTypStack, visit)
			inCompositeLit = false
			return false
		case *ast.File:
			for _, d := range n.Decls {
				ast.Inspect(d, visit)
			}
			return false
		}

		return true
	}

	ast.Inspect(n, visit)
}

func (ctx *Context) ScanPkg(pkg *ast.Package) {
	for name, file := range pkg.Files {
		debugp("Scan file: %s", name)
		ctx.Scan(file, pkg)
	}
}

func (ctx *Context) ScanFiles(filenames []string) error {
	if ctx.Scope != nil {
		ctx.Scan(ctx.Scope, ctx.LocalPkg)
		return nil
	}

	pkgs, err := parser.ParseFiles(types.FileSet, filenames, 0)
	if err != nil {
		return errorGenerator("cannot parse files, %v", err)
	}
	if len(pkgs) == 0 {
		return errorGenerator("cannot find any packages in given files")
	}

	for _, pkg := range pkgs {
		ctx.ScanPkg(pkg)
	}
	return nil
}

func (ctx *Context) WhereIs(n ast.Expr) token.Position {
	switch n := n.(type) {
	default:
		return types.FileSet.Position(n.Pos())
	case *ast.SelectorExpr:
		return types.FileSet.Position(n.Sel.Pos())
	}
}

// To selector subject, the declaration position will not be visited as
// a selector, which require this method to let caller access the declaration
// position
func (ctx *Context) SubjectDeclPos() (token.Position, bool) {
	// TODO I'm a terrible workaround, fix me!
	if _, ok := ctx.Subject.(*selectorSub); ok {
		// if decl file is in search path, give decl position as
		// a reference, too.
		subPosition := types.FileSet.Position(ctx.Subject.DeclPos())
		dir := filepath.Dir(subPosition.Filename)
		if strings.HasPrefix(dir, ctx.Path) {
			return subPosition, true
		}
	}

	return types.FileSet.Position(token.NoPos), false
}

func (ctx *Context) visitExpr(n ast.Expr, pkg *ast.Package) bool {
	debugp("visit expr, %T %v", n, n)
	if ctx.Subject.IsMe(n, pkg) {
		ctx.RefPrinter(n)
	}

	return true
}

// ----------------------------------------------------------------------
func typeOf(n ast.Expr) types.Type {
	_, t := types.ExprType(n, types.DefaultImporter)
	if t.Kind == ast.Bad || t.Node == nil {
		return types.Type{Kind: ast.Bad}
	}

	return t
}

func errorGenerator(format string, a ...interface{}) error {
	return errors.New(fmt.Sprintf(format, a...))
}

func pretty(n ast.Node) string {
	var b bytes.Buffer
	var emptyFileSet = token.NewFileSet()
	printer.Fprint(&b, emptyFileSet, n)
	return b.String()
}

func findIdentifier(f *ast.File, searchpos int) (e ast.Expr, fdecl *ast.FuncDecl, err error) {
	found := false

	var visit inspector
	compositeLitTypStack := list.New()
	visit = func(n ast.Node) bool {
		if found {
			return false
		}
		var startPos token.Pos
		switch n := n.(type) {
		case *ast.FuncDecl:
			ast.Inspect(n.Name, visit)

			fdecl = n
			if n.Recv != nil {
				ast.Inspect(n.Recv, visit)
			}

			ast.Inspect(n.Type, visit)
			if n.Body != nil {
				ast.Inspect(n.Body, visit)
			}
			fdecl = nil
			return false
		case *ast.CompositeLit:
			visitCompositeLit(n, compositeLitTypStack, visit)
			return false
		case *ast.Ident:
			startPos = n.NamePos
			e = n
		case *ast.SelectorExpr:
			startPos = n.Sel.NamePos
			e = n
		default:
			return true
		}

		start := types.FileSet.Position(startPos).Offset
		end := start + int(n.End()-startPos)
		found = start <= searchpos && searchpos <= end

		return !found
	}
	ast.Inspect(f, visit)

	if !found {
		e = nil
		fdecl = nil
		err = errorGenerator("cannot find identifier")
	}

	return
}

func visitCompositeLit(n *ast.CompositeLit, compositeLitTypStack *list.List, visit func(n ast.Node) bool) {
	if n.Type != nil {
		compositeLitTyp := depointer(n.Type)
		ast.Inspect(n.Type, visit)

		// Get real type if this is an array composite literal
		if aryTyp, ok := n.Type.(*ast.ArrayType); ok {
			compositeLitTyp = depointer(aryTyp.Elt)
			debugp("composite literal is an array")
		}

		listElt := compositeLitTypStack.PushFront(compositeLitTyp)
		defer compositeLitTypStack.Remove(listElt)
	}

	listElt := compositeLitTypStack.Front()
	compositeLitTyp := listElt.Value.(ast.Expr)

	debugp("composite literal type: %T %v", compositeLitTyp, compositeLitTyp)
	if compositeLitTyp == nil {
		return
	}

	debugp("composite literal, elements len %d", len(n.Elts))
	for _, element := range n.Elts {
		if elt, ok := element.(*ast.KeyValueExpr); ok {
			if key, ok := elt.Key.(*ast.Ident); ok {
				e := &ast.SelectorExpr{compositeLitTyp, key}
				debugp("Inspect %T %v", e, e)
				ast.Inspect(e, visit)

				ast.Inspect(elt.Value, visit)
				continue
			}
		}
		ast.Inspect(element, visit)
	}
}

func depointer(n ast.Expr) ast.Expr {
	if v, ok := n.(*ast.StarExpr); ok {
		return v.X
	}

	return n
}

func findFieldOuter(field *ast.Field, f *ast.File) (outer *ast.Ident) {
	fldPos := field.Pos()
	fldStart := types.FileSet.Position(fldPos).Offset
	fldEnd := fldStart + int(field.End()-fldPos)

	found := false
	visit := func(n ast.Node) bool {
		var (
			recv *ast.StructType
			t    *ast.TypeSpec
			ok   bool
		)
		if found {
			return false
		}

		if t, ok = n.(*ast.TypeSpec); ok {
			if recv, ok = t.Type.(*ast.StructType); !ok {
				return true
			}
		}

		if recv == nil {
			return true
		}

		start := types.FileSet.Position(recv.Pos()).Offset
		end := start + int(recv.End()-recv.Pos())
		if start > fldStart || fldEnd > end || recv.Fields == nil {
			return true
		}

		for _, fld := range recv.Fields.List {
			if fld.Pos() == field.Pos() && fld.End() == field.End() {
				outer = t.Name
				found = true
				break
			}
		}

		return !found
	}
	ast.Inspect(f, visit)

	return
}

func parseLocalPackage(filename string, src *ast.File, pkgScope *ast.Scope) (*ast.Package, error) {
	pkg := &ast.Package{pkgNameOfFile(filename), pkgScope, nil, map[string]*ast.File{filename: src}}
	d, f := filepath.Split(filename)
	if d == "" {
		d = "./"
	}
	fd, err := os.Open(d)
	if err != nil {
		return nil, errorGenerator("open dir %s failed, %v", d, err)
	}
	defer fd.Close()

	list, err := fd.Readdirnames(-1)
	if err != nil {
		return nil, errorGenerator("read dir names failed, %v", err)
	}

	for _, pf := range list {
		file := filepath.Join(d, pf)
		if !strings.HasSuffix(pf, ".go") ||
			pf == f ||
			pkgNameOfFile(file) != pkg.Name {
			continue
		}
		src, err := parser.ParseFile(types.FileSet, file, nil, 0, pkg.Scope)
		if err == nil {
			pkg.Files[file] = src
		}
	}
	if len(pkg.Files) == 1 {
		return nil, NoMorePkgFiles
	}
	return pkg, nil
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
