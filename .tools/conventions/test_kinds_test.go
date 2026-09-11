package conventions

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// unitTestRoots hold tests beside the code: a _test.go under one of them is
// a unit test and runs with nothing installed.
var unitTestRoots = []string{"pkg", "otel", "cmd"}

// connectVerbs are the calls that open a database connection, by import
// path. NewPostgresPool is also matched as a bare call inside its own
// package.
var connectVerbs = map[string][]string{
	"github.com/jackc/pgx/v5":                        {"Connect", "ConnectConfig", "ConnectWithOptions"},
	"github.com/jackc/pgx/v5/pgconn":                 {"Connect", "ConnectConfig", "ConnectWithOptions"},
	"github.com/jackc/pgx/v5/pgxpool":                {"New", "NewWithConfig"},
	"github.com/agentstax/sqlstreams/pkg/sqlstreams": {"NewPostgresPool"},
}

// testDatabaseSeam is the one reader of the SQLSTREAMS_TEST_* variables.
const testDatabaseSeam = ".tests/integration/postgres"

// privateHelperNames are the panic-and-recover helpers an e2e program
// must not declare: a failed step is a returned error, never a panic.
var privateHelperNames = map[string]bool{"must": true, "die": true, "assert": true, "testFailure": true}

// A unit test runs with nothing installed, so no _test.go beside the code
// opens a connection. A test that needs Postgres is an integration test
// under .tests/integration/, where the container is the test's own.
func TestUnitTestsOpenNoConnection(t *testing.T) {
	files := testFiles(t, unitTestRoots...)
	for _, file := range files {
		parsed, fileSet := parseGoFile(t, file.Path)
		imports := importedPaths(parsed)
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			path, name := calledFunction(call, imports)
			for _, verb := range connectVerbs[path] {
				if verb == name {
					t.Errorf("%s:%d opens a connection with %s -- a test that needs Postgres lives under .tests/integration/", file.Relative, fileSet.Position(call.Pos()).Line, name)
				}
			}
			return true
		})
	}
	if len(files) == 0 {
		t.Fatal("no unit test reached the walk -- check unitTestRoots")
	}
}

// A unit test never waits on the clock: time.Sleep appears only inside a
// testing/synctest bubble, where the clock is the bubble's own.
func TestUnitTestsNeverSleep(t *testing.T) {
	files := testFiles(t, unitTestRoots...)
	for _, file := range files {
		parsed, fileSet := parseGoFile(t, file.Path)
		imports := importedPaths(parsed)
		if imports["synctest"] == "testing/synctest" {
			continue
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if path, name := calledFunction(call, imports); path == "time" && name == "Sleep" {
				t.Errorf("%s:%d sleeps -- expiry is an UPDATE on expires_at, in-process timing is a synctest bubble", file.Relative, fileSet.Position(call.Pos()).Line)
			}
			return true
		})
	}
	if len(files) == 0 {
		t.Fatal("no unit test reached the walk -- check unitTestRoots")
	}
}

// Setup goes through the real registration verbs: no test carries a copy of
// a table's DDL, which would go stale the day the baseline changes.
func TestTestsCarryNoTableDdl(t *testing.T) {
	files := testFiles(t, append(unitTestRoots, ".tests")...)
	for _, file := range files {
		if strings.Contains(strings.ToUpper(readFile(t, file.Path)), "CREATE TABLE") {
			t.Errorf("%s carries CREATE TABLE text -- register the system, stream, or consumer group instead", file.Relative)
		}
	}
	if len(files) == 0 {
		t.Fatal("no test reached the walk -- check unitTestRoots")
	}
}

// The test database variables are read only by the Docker seam, so every
// test reaches Postgres through the same container-or-URL choice.
func TestTestDatabaseVariablesReadOnlyThroughTheSeam(t *testing.T) {
	files := append(testFiles(t, unitTestRoots...), goFiles(t, ".tests")...)
	for _, file := range files {
		if strings.HasPrefix(file.Relative, testDatabaseSeam+"/") {
			continue
		}
		if strings.Contains(readFile(t, file.Path), "SQLSTREAMS_TEST_") {
			t.Errorf("%s reads a SQLSTREAMS_TEST_ variable -- only %s does; take the datastore from postgres.Start", file.Relative, testDatabaseSeam)
		}
	}
	if len(files) == 0 {
		t.Fatal("no file reached the walk -- check unitTestRoots")
	}
}

// No e2e program declares a must, die, or assert helper -- a failed step
// is a returned error, so deferred cleanup runs on the way out.
func TestE2eProgramsDeclareNoPrivateHelpers(t *testing.T) {
	files := goFiles(t, ".tests/e2e")
	walked := 0
	for _, file := range files {
		if strings.HasPrefix(file.Relative, ".tests/e2e/common/") {
			continue
		}
		walked++
		parsed, fileSet := parseGoFile(t, file.Path)
		for _, declaration := range parsed.Decls {
			name, position := declaredName(declaration, fileSet)
			if privateHelperNames[name] {
				t.Errorf("%s:%d declares %s -- take it from .tests/e2e/common", file.Relative, position, name)
			}
		}
	}
	if walked == 0 {
		t.Fatal("no e2e program reached the walk -- check .tests/e2e")
	}
}

// ***************
// *** HELPERS ***
// ***************

// sourceFile is one Go file with its repo-relative path.
type sourceFile struct {
	Path     string
	Relative string
}

// testFiles lists every _test.go under the roots.
func testFiles(t *testing.T, roots ...string) []sourceFile {
	t.Helper()
	var files []sourceFile
	for _, root := range roots {
		for _, file := range goFiles(t, root) {
			if strings.HasSuffix(file.Path, "_test.go") {
				files = append(files, file)
			}
		}
	}
	return files
}

// goFiles lists every .go file under the root.
func goFiles(t *testing.T, root string) []sourceFile {
	t.Helper()
	base := repoRoot(t)
	var files []sourceFile
	err := filepath.WalkDir(filepath.Join(base, root), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(base, path)
		if err != nil {
			return err
		}
		files = append(files, sourceFile{Path: path, Relative: filepath.ToSlash(relative)})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// parseGoFile parses one file with positions.
func parseGoFile(t *testing.T, path string) (*ast.File, *token.FileSet) {
	t.Helper()
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return parsed, fileSet
}

// readFile reads one file's text.
func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// importedPaths maps each import's local name to its path -- the alias
// when one is given, else the path's last segment.
func importedPaths(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = path
	}
	return imports
}

// calledFunction names a call's package path and function: the import a
// selector's package identifier resolves to, or "" for a bare call.
func calledFunction(call *ast.CallExpr, imports map[string]string) (string, string) {
	switch function := call.Fun.(type) {
	case *ast.SelectorExpr:
		if identifier, ok := function.X.(*ast.Ident); ok {
			return imports[identifier.Name], function.Sel.Name
		}
	case *ast.Ident:
		if function.Name == "NewPostgresPool" {
			return "github.com/agentstax/sqlstreams/pkg/sqlstreams", function.Name
		}
	}
	return "", ""
}

// declaredName is a top-level declaration's name and line: a func without
// a receiver, or a single type.
func declaredName(declaration ast.Decl, fileSet *token.FileSet) (string, int) {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		if typed.Recv == nil {
			return typed.Name.Name, fileSet.Position(typed.Pos()).Line
		}
	case *ast.GenDecl:
		for _, spec := range typed.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				return typeSpec.Name.Name, fileSet.Position(typeSpec.Pos()).Line
			}
		}
	}
	return "", 0
}
