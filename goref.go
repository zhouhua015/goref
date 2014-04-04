package main

import (
	"code.google.com/p/rog-go/exp/go/ast"
	"code.google.com/p/rog-go/exp/go/token"
	"code.google.com/p/rog-go/exp/go/types"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var offset = flag.Int("o", -1, "file offset of identifier in stdin")
var fflag = flag.String("f", "", "Go source filename")
var rflag = flag.Bool("R", false, "recurse into sub-directories of given path")
var verbose = flag.Bool("v", false, "show matched line")
var debug = flag.Bool("debug", false, "debug mode")
var typdebug = flag.Bool("typdebug", false,
	"turn on type debug mode too, must be used with debug mode")

var Debug bool = false
var Verbose bool = false

func debugp(f string, a ...interface{}) {
	if Debug {
		log.Printf(f, a...)
	}
}

func fail(s string, a ...interface{}) {
	fmt.Fprint(os.Stderr, "goref: "+fmt.Sprintf(s, a...)+"\n")
	os.Exit(2)
}

func init() {
	// take GOPATH, set types.GoPath to it  if it's not empty.
	p := os.Getenv("GOPATH")
	if p == "" {
		return
	}
	gopath := strings.Split(p, ":")
	for i, d := range gopath {
		gopath[i] = filepath.Join(d, "src")
	}
	r := runtime.GOROOT()
	if r != "" {
		gopath = append(gopath, r+"/src/pkg")
	}
	types.GoPath = gopath
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: goref [flags] PATH\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	Debug = *debug
	searchPos := *offset
	fileName := *fflag
	recurse := *rflag
	types.Debug = *debug && *typdebug
	if flag.NArg() != 1 || searchPos == -1 || fileName == "" {
		flag.Usage()
		os.Exit(2)
	}
	path := flag.Args()[0]
	Verbose = *verbose

	// pre-process parameters
	absFileName, err := filepath.Abs(fileName)
	if err != nil {
		fail("cannot get absolute path for file %s, %v", fileName, err)
	}
	fileName, err = filepath.EvalSymlinks(absFileName)
	if err != nil {
		fail("cannot resolve symlinks for file %s, %v", fileName, err)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		fail("cannot get absolute path for %s, %v", path, err)
	}
	path, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		fail("cannot resolve symlinks for path %s, %v", path, err)
	}

	filenames, err := getFileNames(recurse, path)
	if err != nil {
		fail("cannot find any go file in %s, %v", path, err)
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = ""
	}

	context := NewContext(fileName, searchPos, path)
	context.RefPrinter = func(n ast.Expr) {
		position := context.WhereIs(n)
		printRefPosition(wd, position)
	}

	err = context.ParseSubject()
	if err != nil {
		fail("parse identifier failed, %s", fmt.Sprintf("%v", err))
	}

	if position, ok := context.SubjectDeclPos(); ok {
		printRefPosition(wd, position)
	}
	err = context.ScanFiles(filenames)
	if err != nil {
		fail(err.Error())
	}
}

func isGoFile(d os.FileInfo) bool {
	return strings.HasSuffix(d.Name(), ".go") &&
		!strings.HasPrefix(d.Name(), ".")
}

func getFileNames(recurse bool, path string) ([]string, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	list, err := fd.Readdir(-1)
	if err != nil {
		return nil, err
	}

	filenames := make([]string, 0)
	n := 0
	for i := 0; i < len(list); i++ {
		d := list[i]
		absPath := filepath.Join(path, d.Name())

		switch {
		case d.IsDir() && recurse:
			fns, err := getFileNames(recurse, absPath)
			if err != nil {
				return nil, err
			}

			filenames = append(filenames, fns...)
			n += len(fns)
		case isGoFile(d):
			filenames = append(filenames, absPath)
			n++
		}
	}

	return filenames[:n], nil
}

func printRefPosition(base string, pos token.Position) {
	refPosition := fmt.Sprintf("%s:%d:%d",
		processFilePath(pos.Filename, base),
		pos.Line,
		pos.Column)
	if Verbose {
		line := readFileLine(pos)
		refPosition += fmt.Sprintf("\n%s", line)
	}
	fmt.Println(refPosition)
}

func processFilePath(path string, base string) string {
	if strings.HasPrefix(path, base) {
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return path
		}

		return rel
	}
	return path
}

func readFileLine(position token.Position) string {
	src, err := ioutil.ReadFile(position.Filename)
	if err != nil {
		return ""
	}

	var start int
	for offset := position.Offset; offset >= 0; offset-- {
		if src[offset] == '\n' {
			start = offset + 1
			break
		}
	}
	if start < 0 {
		return ""
	}

	var end int
	for offset := position.Offset; offset < len(src); offset++ {
		if src[offset] == '\n' {
			end = offset
			break
		}
	}
	if end <= start {
		return ""
	}

	return string(src[start:end])
}
