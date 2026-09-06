package conventions

// Walks every baseline CREATE TABLE literal under pkg/ plus the per-topic
// table-name funcs in pkg/topic and enforces the mechanical half of
// CONVENTIONS.md ## Tables naming rules [0611][0613]: table names end in a
// known kind, TIMESTAMPTZ columns end _at/_after, duration columns are
// BIGINT nanoseconds ending _ns, every _config table carries created_at and
// updated_at. Judgment rules (root wording, prefix choice) stay review-time.

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
	Name     string             // "" when the name is a %s placeholder (per-topic)
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
	for name, position := range perTopicTableNames(t) {
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
	err := filepath.WalkDir(filepath.Join(root, "pkg"), func(path string, entry fs.DirEntry, walkErr error) error {
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
		// a per-topic literal names its table only through the Sprintf's
		// table-name call (topic.BindingConfigTable(id)), so the call is
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
				// the kind check reads partition names through pkg/topic's funcs
				if statement.Name == "" && len(statement.Columns) > 0 {
					statement.Name = perTopicTableNameFromCall(node)
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

// perTopicTableNameFromCall reads the table name a per-topic CREATE TABLE
// literal is filled with: the Sprintf's [2] value is a pkg/topic table-name
// call, and the func's name minus its Table suffix is the table's root in
// CamelCase (BindingConfigTable -> binding_config). "" when the shape differs.
func perTopicTableNameFromCall(call *ast.CallExpr) string {
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

// schemaQualifier is what every SQL literal writes ahead of a table name.
// Trimming it is what keeps a shared table's name visible to the kind check:
// left on, the name still holds a %% and reads as a per-topic placeholder, so
// the check walks nothing.
const schemaQualifier = schemaVerb + "."

// parseCreateTable reads one CREATE TABLE literal into its name and column
// lines. startLine is the literal's opening line in its file, so each parsed
// line can carry a real file:line.
func parseCreateTable(text string, file string, startLine int) tableStatement {
	statement := tableStatement{Position: file + ":" + strconv.Itoa(startLine)}
	lines := strings.Split(text, "\n")

	// the CREATE line names the table; a %s placeholder means per-topic --
	// those names are checked through pkg/topic's funcs instead
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

// perTopicTableNames reads pkg/topic's table-name funcs: every Sprintf
// format shaped <name>_%d is a per-topic table name.
func perTopicTableNames(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	path := filepath.Join(root, "pkg", "topic", "tables.go")

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	perTopicName := regexp.MustCompile(`^([a-z_]+)_%d$`)
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
		match := perTopicName.FindStringSubmatch(text)
		if match == nil {
			return true
		}
		names[match[1]] = "pkg/topic/tables.go:" + strconv.Itoa(fileSet.Position(literal.Pos()).Line)
		return true
	})
	return names
}
