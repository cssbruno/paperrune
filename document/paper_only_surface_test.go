// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0

package document

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestExportedHTMLNamesArePaperOutputsOnly(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	var got []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		file, err := parser.ParseFile(set, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if !value.Name.IsExported() || !strings.Contains(value.Name.Name, "HTML") {
					continue
				}
				name := value.Name.Name
				if value.Recv != nil {
					name = "(*" + surfaceReceiverName(value.Recv.List[0].Type) + ")." + name
				}
				got = append(got, name)
			case *ast.GenDecl:
				for _, specification := range value.Specs {
					switch spec := specification.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() && strings.Contains(spec.Name.Name, "HTML") {
							got = append(got, spec.Name.Name)
						}
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if name.IsExported() && strings.Contains(name.Name, "HTML") {
								got = append(got, name.Name)
							}
						}
					}
				}
			}
		}
	}
	sort.Strings(got)
	want := []string{"(*PaperPlan).ExportHTML", "(*PaperPlan).ExportHTMLContext"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("exported HTML surface drifted:\n got %v\nwant %v", got, want)
	}
}

func surfaceReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if name, ok := expression.(*ast.Ident); ok {
		return name.Name
	}
	return ""
}
