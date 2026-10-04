package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

var moduleNames = map[string]struct{}{
	"identity": {}, "players": {}, "venues": {}, "matches": {}, "payments": {},
	"notifications": {}, "communication": {}, "recommendations": {}, "moderation": {},
}

func TestModuleBoundaries(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate architecture test")
	}
	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	violations, err := inspectModules(filepath.Join(backendRoot, "internal", "modules"))
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Fatalf("module boundary violations:\n%s", strings.Join(violations, "\n"))
	}
}

func inspectModules(root string) ([]string, error) {
	var violations []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || (filepath.Ext(path) != ".go" && filepath.Ext(path) != ".sql") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		owner := strings.Split(filepath.ToSlash(relative), "/")[0]
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		searchable := contents
		if filepath.Ext(path) == ".go" {
			imports, err := goImports(path, contents)
			if err != nil {
				return err
			}
			for _, imported := range imports {
				match := regexp.MustCompile(`^badmintonhub/internal/modules/([^/]+)/internal(?:/|$)`).FindStringSubmatch(imported)
				if len(match) > 0 && match[1] != owner {
					violations = append(violations, fmt.Sprintf("%s: %s imports private code owned by %s", relative, owner, match[1]))
				}
			}
			literals, err := goStringLiterals(path, contents)
			if err != nil {
				return err
			}
			var sqlLiterals []string
			for _, literal := range literals {
				if looksLikeSQL(literal) {
					sqlLiterals = append(sqlLiterals, literal)
				}
			}
			searchable = []byte(strings.Join(sqlLiterals, "\n"))
		}
		for schema := range moduleNames {
			if schema == owner {
				continue
			}
			pattern := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(schema) + `\s*\.`)
			if pattern.Match(searchable) {
				violations = append(violations, fmt.Sprintf("%s: %s references schema owned by %s", relative, owner, schema))
			}
		}
		return nil
	})
	return violations, err
}

var sqlKeyword = regexp.MustCompile(`(?i)\b(SELECT|INSERT|UPDATE|DELETE|FROM|JOIN|CREATE|ALTER|DROP|TRUNCATE)\b`)

func looksLikeSQL(value string) bool {
	return sqlKeyword.MatchString(value)
}

func goStringLiterals(filename string, contents []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, contents, 0)
	if err != nil {
		return nil, err
	}
	var values []string
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err == nil {
			values = append(values, value)
		}
		return true
	})
	return values, nil
}

func goImports(filename string, contents []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, contents, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		imports = append(imports, value)
	}
	return imports, nil
}

func TestInspectorDetectsForbiddenPrivateImportAndSchema(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "matches")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package matches
import _ "badmintonhub/internal/modules/payments/internal/postgres"
const query = "SELECT * FROM payments.receipts"
`
	if err := os.WriteFile(filepath.Join(path, "bad.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	violations, err := inspectModules(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 {
		t.Fatalf("violations=%v, want private import and cross-schema SQL", violations)
	}
}
