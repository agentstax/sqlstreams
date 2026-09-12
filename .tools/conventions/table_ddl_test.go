package conventions

// Walks every baseline CREATE TABLE literal under client/ and pkg/ plus
// the per-stream table-name funcs in pkg/stream and enforces the mechanical half of
// CONVENTIONS.md ## Tables naming rules [0611][0613]: table names end in a
// known kind, TIMESTAMPTZ columns end _at/_after, duration columns are
// BIGINT nanoseconds ending _ns, every _config table carries created_at and
// updated_at, every _cursor table opens with its own id, every index is named
// for its table then its leading columns. Judgment rules (root wording, prefix choice) stay review-time.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

type tableStatement struct {
	Position string             // file:line of the CREATE TABLE line
	Name     string             // "" when the name is a %s placeholder (per-stream)
	Columns  []columnDefinition // empty for PARTITION OF statements
}

type columnDefinition struct {
	Position string // file:line of the column's own line
	Name     string
	Type     string
}

// tableKinds is [0611]'s registry: the trailing word every table name ends in.
var tableKinds = map[string]bool{
	"config":   true,
	"log":      true,
	"queue":    true,
	"lease":    true,
	"instance": true,
	"cursor":   true,
	"head":     true,
}

// tableKindExceptions are [0611]'s deliberate leftovers outside the kind set.
var tableKindExceptions = map[string]bool{
	"idempotency_key": true,
}

func TestTableNamesEndInAKnownKind(t *testing.T) {
	names := make(map[string]string) // name -> position

	for _, statement := range baselineTableStatements(t) {
		if statement.Name != "" {
			names[statement.Name] = statement.Position
		}
	}
	for name, position := range perStreamTableNames(t) {
		names[name] = position
	}

	for name, position := range names {
		if tableKindExceptions[name] {
			continue
		}
		kind := name[strings.LastIndex(name, "_")+1:]
		if !tableKinds[kind] {
			t.Errorf("%s table %q ends in %q, not a known kind [0611]", position, name, kind)
		}
	}
}

func TestTimestamptzColumnsEndAtOrAfter(t *testing.T) {
	for _, statement := range baselineTableStatements(t) {
		for _, column := range statement.Columns {
			if column.Type != "TIMESTAMPTZ" {
				continue
			}
			if !strings.HasSuffix(column.Name, "_at") && !strings.HasSuffix(column.Name, "_after") {
				t.Errorf("%s TIMESTAMPTZ column %q must end _at or _after [0613]", column.Position, column.Name)
			}
		}
	}
}

// configTimestampColumns is [0667]'s rule: the pair every _config table
// declares.
var configTimestampColumns = []string{"created_at", "updated_at"}

func TestConfigTablesCarryCreatedAtAndUpdatedAt(t *testing.T) {
	for _, statement := range baselineTableStatements(t) {
		if !strings.HasSuffix(statement.Name, "_config") {
			continue
		}
		declared := make(map[string]bool)
		for _, column := range statement.Columns {
			declared[column.Name] = true
		}
		for _, name := range configTimestampColumns {
			if !declared[name] {
				t.Errorf("%s _config table %q lacks %s [0667]", statement.Position, statement.Name, name)
			}
		}
	}
}

// TestCursorTablesCarryTheirOwnId is [0668]'s rule: a _cursor table's first
// column is its own sequence id, never the owner's id.
func TestCursorTablesCarryTheirOwnId(t *testing.T) {
	for _, statement := range baselineTableStatements(t) {
		if !strings.HasSuffix(statement.Name, "_cursor") || len(statement.Columns) == 0 {
			continue
		}
		first := statement.Columns[0]
		if first.Name != "id" || first.Type != "BIGSERIAL" {
			t.Errorf("%s _cursor table %q opens with %s %s, not id BIGSERIAL [0668]", first.Position, statement.Name, first.Name, first.Type)
		}
	}
}

// indexStatement is one CREATE INDEX literal: the name after its table
// prefix and the columns it covers, in order.
type indexStatement struct {
	Position string
	Name     string   // the full name as written
	Suffix   string   // Name minus "<table>_" (or "%[2]s_" per-stream)
	Columns  []string // the parenthesised column list, in order
}

// TestIndexNamesAreTableThenLeadingColumns is [0669]'s rule: an index is
// named for its table and then its leading columns, in column order, as
// many as it takes to be distinct on that table -- never for a purpose.
func TestIndexNamesAreTableThenLeadingColumns(t *testing.T) {
	for _, index := range baselineIndexStatements(t) {
		matched := false
		for count := 1; count <= len(index.Columns); count++ {
			if index.Suffix == strings.Join(index.Columns[:count], "_") {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("%s index %q: suffix %q is not the leading columns of (%s) [0669]", index.Position, index.Name, index.Suffix, strings.Join(index.Columns, ", "))
		}
	}
}

// durationWord marks a column as duration-shaped by a whole underscore-
// separated word, never a substring (settled_head contains "ttl").
var durationWord = regexp.MustCompile(`(^|_)(ttl|timeout|duration)(_|$)`)

func TestDurationColumnsAreBigintNs(t *testing.T) {
	for _, statement := range baselineTableStatements(t) {
		for _, column := range statement.Columns {
			if strings.HasSuffix(column.Name, "_ns") && column.Type != "BIGINT" {
				t.Errorf("%s column %q ends _ns but is %s, not BIGINT [0613]", column.Position, column.Name, column.Type)
			}
			if durationWord.MatchString(column.Name) && !strings.HasSuffix(column.Name, "_ns") {
				t.Errorf("%s duration column %q must end _ns [0613]", column.Position, column.Name)
			}
		}
	}
}

// ***************
// *** HELPERS ***
// ***************

// constraintKeywords open the table-level lines a column parse skips.
var constraintKeywords = map[string]bool{
	"PRIMARY":    true,
	"UNIQUE":     true,
	"CHECK":      true,
	"FOREIGN":    true,
	"CONSTRAINT": true,
}

func baselineTableStatements(t *testing.T) []tableStatement {
	t.Helper()
	root := repoRoot(t)

	var statements []tableStatement
	for _, tree := range []string{"client", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			fileSet := token.NewFileSet()
			parsed, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				relative = path
			}
			// a per-stream literal names its table only through the Sprintf's
			// table-name call (stream.BindingConfigTable(id)), so the call is
			// inspected first and the literal it holds is skipped on its own visit
			parsedLiterals := make(map[token.Pos]bool)
			ast.Inspect(parsed, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.CallExpr:
					literal, ok := sprintfCreateTableLiteral(node)
					if !ok {
						return true
					}
					parsedLiterals[literal.Pos()] = true
					statement, ok := parseTableLiteral(fileSet, literal, relative)
					if !ok {
						return true
					}
					// a PARTITION OF statement parses no columns and takes no name:
					// the kind check reads partition names through pkg/stream's funcs
					if statement.Name == "" && len(statement.Columns) > 0 {
						statement.Name = perStreamTableNameFromCall(node)
					}
					statements = append(statements, statement)
				case *ast.BasicLit:
					if parsedLiterals[node.Pos()] {
						return true
					}
					if statement, ok := parseTableLiteral(fileSet, node, relative); ok {
						statements = append(statements, statement)
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return statements
}

// parseTableLiteral reads one string literal into a table statement; false
// when the literal holds no CREATE TABLE.
func parseTableLiteral(fileSet *token.FileSet, literal *ast.BasicLit, file string) (tableStatement, bool) {
	if literal.Kind != token.STRING || !strings.Contains(literal.Value, "CREATE TABLE") {
		return tableStatement{}, false
	}
	text, err := strconv.Unquote(literal.Value)
	if err != nil {
		return tableStatement{}, false
	}
	startLine := fileSet.Position(literal.Pos()).Line
	return parseCreateTable(text, file, startLine), true
}

// sprintfCreateTableLiteral returns the format literal of a fmt.Sprintf call
// whose format holds a CREATE TABLE.
func sprintfCreateTableLiteral(call *ast.CallExpr) (*ast.BasicLit, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Sprintf" || len(call.Args) == 0 {
		return nil, false
	}
	if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "fmt" {
		return nil, false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || !strings.Contains(literal.Value, "CREATE TABLE") {
		return nil, false
	}
	return literal, true
}

// perStreamTableNameFromCall reads the table name a per-stream CREATE TABLE
// literal is filled with: the Sprintf's [2] value is a pkg/stream table-name
// call, and the func's name minus its Table suffix is the table's root in
// CamelCase (BindingConfigTable -> binding_config). "" when the shape differs.
func perStreamTableNameFromCall(call *ast.CallExpr) string {
	if len(call.Args) < 3 {
		return ""
	}
	nameCall, ok := call.Args[2].(*ast.CallExpr)
	if !ok {
		return ""
	}
	selector, ok := nameCall.Fun.(*ast.SelectorExpr)
	if !ok || !strings.HasSuffix(selector.Sel.Name, "Table") {
		return ""
	}
	return camelToSnake(strings.TrimSuffix(selector.Sel.Name, "Table"))
}

func camelToSnake(name string) string {
	var out strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out.WriteByte('_')
		}
		out.WriteRune(r | 0x20)
	}
	return out.String()
}

// createIndexLine reads the name, table, and column list off one CREATE
// INDEX line.
var createIndexLine = regexp.MustCompile(`CREATE (?:UNIQUE )?INDEX IF NOT EXISTS (\S+) ON (\S+) \(([^)]*)\)`)

func baselineIndexStatements(t *testing.T) []indexStatement {
	t.Helper()
	root := repoRoot(t)

	var indexes []indexStatement
	for _, tree := range []string{"client", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			fileSet := token.NewFileSet()
			parsed, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				relative = path
			}
			ast.Inspect(parsed, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING || !strings.Contains(literal.Value, "CREATE INDEX") && !strings.Contains(literal.Value, "CREATE UNIQUE INDEX") {
					return true
				}
				text, err := strconv.Unquote(literal.Value)
				if err != nil {
					return true
				}
				match := createIndexLine.FindStringSubmatch(text)
				if match == nil {
					return true
				}
				indexes = append(indexes, parseIndexLine(match, relative+":"+strconv.Itoa(fileSet.Position(literal.Pos()).Line)))
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return indexes
}

// parseIndexLine splits a matched CREATE INDEX into its parts. A shared
// table's index carries the bare table name as its prefix; a per-stream
// index carries the same %[2]s verb its table name is filled from.
func parseIndexLine(match []string, position string) indexStatement {
	name, table, columnList := match[1], match[2], match[3]
	table = strings.TrimPrefix(table, schemaQualifier)
	prefix := table + "_"
	if strings.Contains(name, "%") {
		prefix = name[:strings.Index(name, "s_")+2]
	}
	var columns []string
	for _, column := range strings.Split(columnList, ",") {
		columns = append(columns, strings.TrimSpace(column))
	}
	return indexStatement{
		Position: position,
		Name:     name,
		Suffix:   strings.TrimPrefix(name, prefix),
		Columns:  columns,
	}
}

// schemaQualifier is what every SQL literal writes ahead of a table name.
// Trimming it is what keeps a shared table's name visible to the kind check:
// left on, the name still holds a %% and reads as a per-stream placeholder, so
// the check walks nothing.
const schemaQualifier = schemaVerb + "."

// parseCreateTable reads one CREATE TABLE literal into its name and column
// lines. startLine is the literal's opening line in its file, so each parsed
// line can carry a real file:line.
func parseCreateTable(text string, file string, startLine int) tableStatement {
	statement := tableStatement{Position: file + ":" + strconv.Itoa(startLine)}
	lines := strings.Split(text, "\n")

	// the CREATE line names the table; a %s placeholder means per-stream --
	// those names are checked through pkg/stream's funcs instead
	createIndex := -1
	for i, line := range lines {
		after, found := strings.CutPrefix(strings.TrimSpace(line), "CREATE TABLE IF NOT EXISTS ")
		if !found {
			continue
		}
		statement.Position = file + ":" + strconv.Itoa(startLine+i)
		name := strings.Fields(after)[0]
		name = strings.TrimSuffix(name, "(")
		name = strings.TrimPrefix(name, schemaQualifier)
		if !strings.Contains(name, "%") {
			statement.Name = name
		}
		createIndex = i
		break
	}
	if createIndex < 0 || strings.Contains(text, "PARTITION OF") {
		return statement
	}

	for i, line := range lines[createIndex+1:] {
		if index := strings.Index(line, "--"); index >= 0 {
			line = line[:index]
		}
		line = strings.TrimSuffix(strings.TrimSpace(line), ",")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ")") {
			break
		}

		fields := strings.Fields(line)
		if len(fields) < 2 || constraintKeywords[fields[0]] {
			continue
		}
		statement.Columns = append(statement.Columns, columnDefinition{
			Position: file + ":" + strconv.Itoa(startLine+createIndex+1+i),
			Name:     fields[0],
			Type:     fields[1],
		})
	}
	return statement
}

// perStreamTableNames reads pkg/stream's table-name funcs: every Sprintf
// format shaped <name>_%d is a per-stream table name.
func perStreamTableNames(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	path := filepath.Join(root, "pkg", "stream", "tables.go")

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	perStreamName := regexp.MustCompile(`^([a-z_]+)_%d$`)
	names := make(map[string]string)
	ast.Inspect(parsed, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		text, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		match := perStreamName.FindStringSubmatch(text)
		if match == nil {
			return true
		}
		names[match[1]] = "pkg/stream/tables.go:" + strconv.Itoa(fileSet.Position(literal.Pos()).Line)
		return true
	})
	return names
}
