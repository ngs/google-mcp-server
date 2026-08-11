package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The MCP server speaks JSON-RPC over stdout (see server.MCPServer.Start, which
// wires the protocol stream to os.Stdin/os.Stdout). Anything else written to
// stdout is interleaved into that stream and corrupts the protocol, so
// diagnostics must go to stderr via the log package instead.
//
// These tests encode that invariant at the source level: they fail if any
// non-test file reintroduces an implicit stdout write. A runtime test cannot
// cover this, because most of the offending call sites are error paths that
// only fire against a live Google API.

// stdoutAllowedFiles lists the only files permitted to reference os.Stdout, and
// why. Everything else must log to stderr.
var stdoutAllowedFiles = map[string]string{
	"server/mcp.go": "owns the JSON-RPC stream itself",
	"main.go":       "prints --version before the stream starts, then exits",
}

// forbiddenFmtFuncs are the fmt helpers that write to stdout implicitly.
var forbiddenFmtFuncs = map[string]bool{
	"Print":   true,
	"Printf":  true,
	"Println": true,
}

// TestNoImplicitStdoutWrites fails if any non-test file calls fmt.Print,
// fmt.Printf or fmt.Println, which would write into the JSON-RPC stream.
func TestNoImplicitStdoutWrites(t *testing.T) {
	var violations []string

	forEachSourceFile(t, func(path string, file *ast.File, fset *token.FileSet) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "fmt" || !forbiddenFmtFuncs[sel.Sel.Name] {
				return true
			}

			pos := fset.Position(call.Pos())
			violations = append(violations, path+":"+strconv.Itoa(pos.Line)+": fmt."+sel.Sel.Name)
			return true
		})
	})

	for _, v := range violations {
		t.Errorf("%s writes to stdout and would corrupt the JSON-RPC stream; use log.Printf instead", v)
	}
}

// TestStdoutIsNotReferencedOutsideAllowlist fails if a package other than the
// protocol stream owner reaches for os.Stdout directly, which would bypass the
// fmt.Print guard above.
func TestStdoutIsNotReferencedOutsideAllowlist(t *testing.T) {
	forEachSourceFile(t, func(path string, file *ast.File, fset *token.FileSet) {
		if _, allowed := stdoutAllowedFiles[filepath.ToSlash(path)]; allowed {
			return
		}

		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Stdout" {
				return true
			}

			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "os" {
				return true
			}

			pos := fset.Position(sel.Pos())
			t.Errorf("%s:%d references os.Stdout, which carries the JSON-RPC stream; write to stderr instead",
				path, pos.Line)
			return true
		})
	})
}

// forEachSourceFile parses every non-test Go file in the module and hands it to
// fn with a repo-relative path.
func forEachSourceFile(t *testing.T, fn func(path string, file *ast.File, fset *token.FileSet)) {
	t.Helper()

	fset := token.NewFileSet()

	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip hidden and vendored trees
			if path != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", path, err)
		}

		fn(filepath.ToSlash(path), parsed, fset)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk source tree: %v", err)
	}
}
