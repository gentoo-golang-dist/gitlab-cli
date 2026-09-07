//go:build !integration

package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeySchema_CoversEveryKeyReadByTheCodebase(t *testing.T) {
	t.Parallel()

	walkSourceFiles(t, func(path string, fset *token.FileSet, file *ast.File) {
		imports := importNames(file)

		ast.Inspect(file, func(n ast.Node) bool {
			key, ok := configKeyRead(n, imports)
			if !ok {
				return true
			}
			if findKeyDef(ConfigKeyEquivalence(key)) == nil {
				t.Errorf("%s: config key %q is read here but has no KeyDef in KeySchema; add one to internal/config/schema.go",
					fset.Position(n.Pos()), key)
			}
			return true
		})
	})
}

// GLAB_* and GITLAB_* are glab's own namespaces, so every variable in them
// that the CLI reads should turn up in the reference. Other prefixes are out
// of scope: CI_*, TERM, GOPATH and friends are probes of someone else's
// environment.
func TestGlabEnvVars_AreDeclaredOrDocumented(t *testing.T) {
	t.Parallel()

	documented := map[string]struct{}{}
	for _, ev := range EnvVars() {
		documented[ev.Name] = struct{}{}
	}

	walkSourceFiles(t, func(path string, fset *token.FileSet, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			name, ok := envVarRead(n)
			if !ok || !ownNamespace(name) {
				return true
			}
			if _, ok := documented[name]; ok {
				return true
			}
			t.Errorf("%s: %s is read here but is not in the environment variable reference; add a KeyDef in internal/config/schema.go or an entry in nonSchemaEnvVars in internal/config/envvars.go",
				fset.Position(n.Pos()), name)
			return true
		})
	})
}

// TestEnvVars_CoverEveryDocumentedName guards the other direction: an entry
// that no longer matches a real variable would document a name that does
// nothing.
func TestEnvVars_CoverEveryDocumentedName(t *testing.T) {
	t.Parallel()

	fromSchema := map[string]struct{}{}
	for _, kd := range KeySchema {
		for _, name := range EnvVarsForKey(kd) {
			fromSchema[name] = struct{}{}
		}
	}

	read := map[string]struct{}{}
	walkSourceFiles(t, func(path string, fset *token.FileSet, file *ast.File) {
		ast.Inspect(file, func(n ast.Node) bool {
			if name, ok := envVarRead(n); ok {
				read[name] = struct{}{}
			}
			return true
		})
	})

	for _, ev := range nonSchemaEnvVars {
		if _, ok := fromSchema[ev.Name]; ok {
			t.Errorf("%s is listed in nonSchemaEnvVars but is already a KeySchema env var; drop the hand-written entry", ev.Name)
		}
		if _, ok := read[ev.Name]; !ok && ev.Name != "GITLAB_GROUP" {
			// GITLAB_GROUP is resolved through flag binding rather than a
			// literal os.Getenv, so the AST walk cannot see it.
			t.Errorf("%s is documented in nonSchemaEnvVars but nothing reads it", ev.Name)
		}
	}
}

// predefinedCIVars are set by GitLab CI rather than by a glab user, so they
// belong in the CI documentation rather than in glab's own reference.
var predefinedCIVars = map[string]struct{}{
	"GITLAB_CI": {},
}

func ownNamespace(name string) bool {
	if _, ok := predefinedCIVars[name]; ok {
		return false
	}
	return strings.HasPrefix(name, "GLAB_") || strings.HasPrefix(name, "GITLAB_")
}

func walkSourceFiles(t *testing.T, fn func(path string, fset *token.FileSet, file *ast.File)) {
	t.Helper()

	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join("..", "..", dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "vendor" || d.Name() == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			fn(path, fset, file)
			return nil
		})
		require.NoError(t, err)
	}
}

func importNames(file *ast.File) map[string]struct{} {
	out := map[string]struct{}{}
	for _, imp := range file.Imports {
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		} else if path, err := strconv.Unquote(imp.Path.Value); err == nil {
			name = path[strings.LastIndex(path, "/")+1:]
		}
		if name != "" {
			out[name] = struct{}{}
			out[strings.TrimPrefix(name, "go-")] = struct{}{}
		}
	}
	return out
}

func configKeyRead(n ast.Node, imports map[string]struct{}) (string, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	if recv, ok := sel.X.(*ast.Ident); ok {
		if _, isPkg := imports[recv.Name]; isPkg {
			return "", false
		}
	}
	switch sel.Sel.Name {
	case "Get":
		if len(call.Args) != 2 {
			return "", false
		}
	case "GetWithSource":
		if len(call.Args) != 3 {
			return "", false
		}
	default:
		return "", false
	}
	return stringArg(call.Args[1])
}

func envVarRead(n ast.Node) (string, bool) {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	switch sel.Sel.Name {
	case "Getenv", "LookupEnv", "IsEnvVarEnabled":
	default:
		return "", false
	}
	return stringArg(call.Args[0])
}

func stringArg(arg ast.Expr) (string, bool) {
	lit, ok := arg.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return v, true
}
