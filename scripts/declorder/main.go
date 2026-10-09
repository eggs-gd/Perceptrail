// Command declorder checks that every Go file reads top-down, public first
// (AGENTS.md "Code Style"): public interfaces, public declarations, private
// declarations, public implementations, private implementations.
//
//	go run ./scripts/declorder gontroller perceplib perceptors
//
// It walks the given directories and prints every declaration that comes after one
// it should precede; the exit status is 1 when there is one. Test files and
// generated files are not checked.
//
// Not obvious:
//   - A method counts by its own name: Enqueue on a private type is a public
//     implementation (it implements an interface).
//   - A declaration block (`var ( … )`) counts by its first name.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The groups, in the order a file holds them
const (
	publicInterfaces = iota
	publicDeclarations
	privateDeclarations
	publicImplementations
	privateImplementations
)

var groupNames = []string{
	"public interface", "public declaration", "private declaration",
	"public implementation", "private implementation",
}

func main() {
	roots := os.Args[1:]
	if len(roots) == 0 {
		roots = []string{"."}
	}
	misplaced := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && (d.Name() == "testdata" || d.Name() == "node_modules" || strings.HasPrefix(d.Name(), ".")) && path != root {
				return filepath.SkipDir
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			found, err := check(path)
			misplaced += found
			return err
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
	if misplaced > 0 {
		fmt.Fprintf(os.Stderr, "%d declarations out of order: public interfaces, public declarations, private declarations, public implementations, private implementations\n", misplaced)
		os.Exit(1)
	}
}

// check: the declarations of one file that come after a later group's
func check(path string) (int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return 0, err
	}
	if ast.IsGenerated(file) {
		return 0, nil
	}
	misplaced, highest, highestName := 0, -1, ""
	for _, decl := range file.Decls {
		group, name, ok := groupOf(decl)
		if !ok {
			continue
		}
		if group < highest {
			fmt.Printf("%s: %s %s comes after %s %s\n", fset.Position(decl.Pos()), groupNames[group], name, groupNames[highest], highestName)
			misplaced++
			continue
		}
		highest, highestName = group, name
	}
	return misplaced, nil
}

// groupOf: the group a declaration belongs to and its name; imports have none
func groupOf(decl ast.Decl) (int, string, bool) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if ast.IsExported(d.Name.Name) {
			return publicImplementations, d.Name.Name, true
		}
		return privateImplementations, d.Name.Name, true
	case *ast.GenDecl:
		if d.Tok == token.IMPORT || len(d.Specs) == 0 {
			return 0, "", false
		}
		switch s := d.Specs[0].(type) {
		case *ast.TypeSpec:
			if !ast.IsExported(s.Name.Name) {
				return privateDeclarations, s.Name.Name, true
			}
			if _, ok := s.Type.(*ast.InterfaceType); ok {
				return publicInterfaces, s.Name.Name, true
			}
			return publicDeclarations, s.Name.Name, true
		case *ast.ValueSpec:
			if ast.IsExported(s.Names[0].Name) {
				return publicDeclarations, s.Names[0].Name, true
			}
			return privateDeclarations, s.Names[0].Name, true
		}
	}
	return 0, "", false
}
