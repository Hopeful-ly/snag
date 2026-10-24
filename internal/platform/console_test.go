package platform

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// On Windows a plainly started program flashes a console window, so every function that starts
// one must also use StartGroup, StartDetached, hideConsole or its own SysProcAttr.
func TestNoProgramStartsWithAConsoleWindow(t *testing.T) {
	root := filepath.Join("..", "..")
	var checked int
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			switch e.Name() {
			case "node_modules", "frontend", "extension", "build", "bin", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || notOnWindows(name) {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		if constrainedAwayFromWindows(file) {
			return nil
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			starts, hides := false, false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.SelectorExpr:
					if x, ok := n.X.(*ast.Ident); ok && x.Name == "exec" &&
						(n.Sel.Name == "Command" || n.Sel.Name == "CommandContext") {
						starts = true
					}
					switch n.Sel.Name {
					case "StartGroup", "StartDetached", "SysProcAttr", "hideConsole":
						hides = true
					}
				case *ast.Ident:
					if n.Name == "StartGroup" || n.Name == "StartDetached" || n.Name == "hideConsole" {
						hides = true
					}
				}
				return true
			})
			if starts {
				checked++
				if !hides {
					t.Errorf("%s: %s starts a program that shows a console window on Windows; "+
						"use platform.StartGroup", path, fn.Name.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked < 5 {
		t.Fatalf("found only %d functions that start programs: the check isn't reading Snag's code", checked)
	}
}

// notOnWindows reports whether a file's name keeps it out of Windows builds.
func notOnWindows(name string) bool {
	base := strings.TrimSuffix(name, ".go")
	for _, suffix := range []string{"_linux", "_darwin", "_unix", "_freebsd"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

// constrainedAwayFromWindows reports whether a file's build line keeps it out of Windows builds.
func constrainedAwayFromWindows(file *ast.File) bool {
	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}
		for _, c := range group.List {
			if line, ok := strings.CutPrefix(c.Text, "//go:build "); ok {
				return strings.Contains(line, "!windows") || !strings.Contains(line, "windows")
			}
		}
	}
	return false
}
