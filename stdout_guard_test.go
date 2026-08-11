package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// The MCP server speaks JSON-RPC over stdout (see server.MCPServer.Start, which
// wires the protocol stream to os.Stdin/os.Stdout). Anything else written to
// stdout is interleaved into that stream and corrupts the protocol, so
// diagnostics must go to stderr via the log package instead.
//
// These tests encode that invariant at the source level: they fail if any
// non-test file reintroduces a stdout write. A runtime test cannot cover this,
// because most of the offending call sites are error paths that only fire
// against a live Google API.

// permittedStdoutUse describes the one expression in one file that is allowed
// to name os.Stdout. The allowlist is deliberately expression-level rather than
// file-level: allowing a whole file would let an unrelated
// fmt.Fprintln(os.Stdout, ...) or log.SetOutput(os.Stdout) slip in later.
type permittedStdoutUse struct {
	file   string // repo-relative, slash-separated
	callee string // the call os.Stdout must be an argument to
	reason string
}

var permittedStdoutUses = []permittedStdoutUse{
	{
		file:   "server/mcp.go",
		callee: "NewNewlineDelimitedStream",
		reason: "constructs the JSON-RPC stream itself",
	},
	{
		file:   "main.go",
		callee: "fmt.Fprintln",
		reason: "prints --version before the stream starts, then exits",
	},
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

			t.Errorf("%s:%d: fmt.%s writes to stdout and would corrupt the JSON-RPC stream; use log.Printf instead",
				path, fset.Position(call.Pos()).Line, sel.Sel.Name)
			return true
		})
	})
}

// TestStdoutIsReferencedOnlyByPermittedExpressions fails if os.Stdout is named
// anywhere other than the exact expressions listed in permittedStdoutUses. It
// also fails if a permitted expression stops matching or gains a duplicate, so
// the allowlist cannot silently go stale or be widened by copy-paste.
func TestStdoutIsReferencedOnlyByPermittedExpressions(t *testing.T) {
	matches := make(map[permittedStdoutUse]int)

	forEachSourceFile(t, func(path string, file *ast.File, fset *token.FileSet) {
		// Every os.Stdout mention in this file, by position.
		found := make(map[token.Pos]bool)
		ast.Inspect(file, func(n ast.Node) bool {
			if isOSStdout(n) {
				found[n.Pos()] = true
			}
			return true
		})
		if len(found) == 0 {
			return
		}

		// Mentions passed directly to a permitted callee are accounted for.
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			for _, permitted := range permittedStdoutUses {
				if permitted.file != path || permitted.callee != calleeName(call.Fun) {
					continue
				}
				for _, arg := range call.Args {
					if isOSStdout(arg) {
						delete(found, arg.Pos())
						matches[permitted]++
					}
				}
			}
			return true
		})

		for pos := range found {
			t.Errorf("%s:%d references os.Stdout, which carries the JSON-RPC stream; write to stderr instead",
				path, fset.Position(pos).Line)
		}
	})

	for _, permitted := range permittedStdoutUses {
		switch matches[permitted] {
		case 1:
			// Exactly the expression we expect.
		case 0:
			t.Errorf("allowlist is stale: expected %s to pass os.Stdout to %s (%s), but found no such call",
				permitted.file, permitted.callee, permitted.reason)
		default:
			t.Errorf("%s passes os.Stdout to %s %d times, expected exactly 1 (the only permitted use %s)",
				permitted.file, permitted.callee, matches[permitted], permitted.reason)
		}
	}
}

// TestLogOutputGoesToStderr fails if log.SetOutput is pointed anywhere but
// os.Stderr. This is the highest-leverage way to reintroduce the bug: a single
// log.SetOutput(os.Stdout) would redirect every diagnostic in the codebase back
// into the protocol stream at once.
func TestLogOutputGoesToStderr(t *testing.T) {
	seen := 0

	forEachSourceFile(t, func(path string, file *ast.File, fset *token.FileSet) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || calleeName(call.Fun) != "log.SetOutput" {
				return true
			}

			seen++
			line := fset.Position(call.Pos()).Line
			if len(call.Args) != 1 || !isOSSelector(call.Args[0], "Stderr") {
				t.Errorf("%s:%d: log.SetOutput must be given os.Stderr; anything else can route diagnostics into the JSON-RPC stream",
					path, line)
			}
			return true
		})
	})

	if seen == 0 {
		t.Error("expected log.SetOutput(os.Stderr) to be called during startup, but found no call at all")
	}
}

// isOSStdout reports whether n is the expression os.Stdout.
func isOSStdout(n ast.Node) bool {
	expr, ok := n.(ast.Expr)
	return ok && isOSSelector(expr, "Stdout")
}

// isOSSelector reports whether expr is os.<name>.
func isOSSelector(expr ast.Expr, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}

	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "os"
}

// calleeName renders a call target as "Func" or "pkg.Func", or "" if it is
// neither (a method value on an arbitrary expression, say).
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		if pkg, ok := f.X.(*ast.Ident); ok {
			return pkg.Name + "." + f.Sel.Name
		}
	}
	return ""
}

// forEachSourceFile parses every non-test Go file in the module and hands it to
// fn with a repo-relative path.
func forEachSourceFile(t *testing.T, fn func(path string, file *ast.File, fset *token.FileSet)) {
	t.Helper()

	fset := token.NewFileSet()
	parsed := 0

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

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("Failed to parse %s: %v", path, err)
		}

		parsed++
		fn(filepath.ToSlash(path), file, fset)
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to walk source tree: %v", err)
	}

	// Guard against the walk silently matching nothing and every check passing
	// vacuously.
	if parsed == 0 {
		t.Fatal("no non-test Go files were parsed; the source walk is broken")
	}
}
