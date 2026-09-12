package conventions

// Walks every struct reachable through sqlstreams that has a WithDefaults
// method and requires each field WithDefaults fills to carry a `Default:`
// line in its comment (CONVENTIONS.md ## Comments). Machinery configs
// below the closure are exempt: their WithDefaults only backstops a
// direct caller after the assembler resolved the whole config.

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

func TestReachableConfigFieldsStateTheirDefault(t *testing.T) {
	root := repoRoot(t)
	reachable := map[string]bool{}
	for _, reached := range clientClosure(t).reachable() {
		reachable[reached.Pkg().Path()+"."+reached.Name()] = true
	}

	for _, tree := range []string{"client", "pkg"} {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			packagePath := modulePath + "/" + filepath.ToSlash(filepath.Dir(relative))

			fileSet := token.NewFileSet()
			parsed, err := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}
			structs := structTypes(parsed)
			for _, declaration := range parsed.Decls {
				function, ok := declaration.(*ast.FuncDecl)
				if !ok || function.Name.Name != "WithDefaults" || function.Recv == nil || function.Body == nil {
					continue
				}
				receiverType, receiverName := receiverOf(function)
				structType, ok := structs[receiverType]
				if !ok || !reachable[packagePath+"."+receiverType] {
					continue
				}
				for _, field := range structType.Fields.List {
					for _, name := range field.Names {
						if !filledFields(function.Body, receiverName)[name.Name] || hasDefaultLine(field) {
							continue
						}
						position := relative + ":" + strconv.Itoa(fileSet.Position(name.Pos()).Line)
						t.Errorf("%s: %s.%s is filled by WithDefaults but its comment has no Default: line -- state the resolved value", position, receiverType, name.Name)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// ***************
// *** HELPERS ***
// ***************

// structTypes maps every struct type declared in file by name.
func structTypes(file *ast.File) map[string]*ast.StructType {
	structs := map[string]*ast.StructType{}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range generic.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				structs[typeSpec.Name.Name] = structType
			}
		}
	}
	return structs
}

// receiverOf returns the receiver's type name and variable name.
func receiverOf(function *ast.FuncDecl) (string, string) {
	receiver := function.Recv.List[0]
	receiverType := ""
	switch expression := receiver.Type.(type) {
	case *ast.StarExpr:
		if identifier, ok := expression.X.(*ast.Ident); ok {
			receiverType = identifier.Name
		}
	case *ast.Ident:
		receiverType = expression.Name
	}
	receiverName := ""
	if len(receiver.Names) > 0 {
		receiverName = receiver.Names[0].Name
	}
	return receiverType, receiverName
}

// filledFields collects every field body assigns through receiverName.
func filledFields(body *ast.BlockStmt, receiverName string) map[string]bool {
	filled := map[string]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, left := range assignment.Lhs {
			selector, ok := left.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if identifier, ok := selector.X.(*ast.Ident); ok && identifier.Name == receiverName {
				filled[selector.Sel.Name] = true
			}
		}
		return true
	})
	return filled
}

// hasDefaultLine reports whether the field's doc or trailing comment carries
// a Default: line.
func hasDefaultLine(field *ast.Field) bool {
	for _, group := range []*ast.CommentGroup{field.Doc, field.Comment} {
		if group == nil {
			continue
		}
		for _, comment := range group.List {
			if strings.Contains(comment.Text, "Default:") {
				return true
			}
		}
	}
	return false
}
