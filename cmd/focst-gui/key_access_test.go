package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestGUIKeyAccess_NoEnvAndExplicitFalse(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	if err != nil {
		t.Fatalf("parse dir: %v", err)
	}

	var guiPkg *ast.Package
	for name, pkg := range pkgs {
		if name == "main" {
			guiPkg = pkg
			break
		}
	}
	if guiPkg == nil {
		t.Fatalf("main package not found in %s", filepath.Base("."))
	}

	for filename, file := range guiPkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "auth" {
				return true
			}
			switch sel.Sel.Name {
			case "GetEnvKey":
				t.Fatalf("auth.GetEnvKey usage found in GUI: %s", filename)
			case "GetKey":
				if len(call.Args) != 2 {
					t.Fatalf("auth.GetKey must be called with allowEnv=false in GUI: %s", filename)
				}
				if ident, ok := call.Args[1].(*ast.Ident); !ok || ident.Name != "false" {
					t.Fatalf("auth.GetKey allowEnv must be false in GUI: %s", filename)
				}
			}
			return true
		})
	}
}
