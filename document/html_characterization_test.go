// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestHTMLCharacterizationBaselineProjection(t *testing.T) {
	first, err := htmlCharacterizationJSON()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := htmlCharacterizationJSON()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("inventory JSON is nondeterministic")
	}
	sum := sha256.Sum256(first)
	got := hex.EncodeToString(sum[:])
	const want = "fdbf8dff0c7c046ad844636622b8de7e2dcc5e72c024d24da095d1a504e7b340"
	if got != want {
		t.Fatalf("HTML characterization drift: hash=%s\n%s", got, first)
	}
	inventory := htmlCharacterization()
	if !sort.StringsAreSorted(inventory.EntryPoints) || !sort.StringsAreSorted(inventory.RecognizedTags) || !sort.StringsAreSorted(inventory.RecognizedCSSProperties) {
		t.Fatal("inventory slices are not canonical")
	}
}

func TestHTMLCharacterizationPublicEntryPointsMatchAST(t *testing.T) {
	want := htmlCharacterization().EntryPoints
	set := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	files := make([]*ast.File, 0)
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasPrefix(entry.Name(), "html") && entry.Name() != "facade.go") || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(set, entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if file.Name.Name == "document" {
			files = append(files, file)
		}
	}
	got := make([]string, 0)
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !fn.Name.IsExported() {
				continue
			}
			receiver := ""
			if fn.Recv != nil {
				receiver = htmlCharacterizationReceiver(fn.Recv.List[0].Type)
			}
			include := receiver == "HTML" || receiver == "CompiledHTML" || (receiver == "Document" && strings.Contains(fn.Name.Name, "HTML")) || receiver == "" && (strings.Contains(fn.Name.Name, "HTML") || fn.Name.Name == "RenderHTMLTemplate")
			if !include {
				continue
			}
			name := fn.Name.Name
			if receiver != "" {
				name = "(*" + receiver + ")." + name
			}
			got = append(got, name)
		}
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("public HTML AST drift\ngot  %v\nwant %v", got, want)
	}
}

func htmlCharacterizationReceiver(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if name, ok := expr.(*ast.Ident); ok {
		return name.Name
	}
	return ""
}

func TestHTMLCharacterizationFixturesExerciseEveryClassification(t *testing.T) {
	seen := map[string]bool{}
	for _, fixture := range htmlCharacterization().Fixtures {
		if len(fixture.Source) > 4096 {
			t.Fatalf("fixture %s is unbounded", fixture.Name)
		}
		seen[fixture.Classification] = true
		compiled, err := compileHTML(fixture.Source)
		if err != nil {
			t.Fatalf("%s compile: %v", fixture.Name, err)
		}
		switch fixture.Classification {
		case "malformed-recovered":
			if len(compiled.RecoveryIssues()) == 0 {
				t.Fatalf("%s has no recovery evidence", fixture.Name)
			}
			pdf := characterizationPDF()
			html := pdf.htmlNew()
			if err := html.WriteContext(context.Background(), 10, fixture.Source); err == nil || !strings.Contains(err.Error(), "recovered HTML") {
				t.Fatalf("%s recovery handling = %v, want strict unified rejection", fixture.Name, err)
			}
		case "diagnostic-unsupported":
			pdf := characterizationPDF()
			html := pdf.htmlNew()
			messages := html.validateHTML(fixture.Source)
			if len(messages) == 0 {
				t.Fatalf("%s has no unsupported diagnostic", fixture.Name)
			}
		case "rejected-by-policy":
			pdf := characterizationPDF()
			html := pdf.htmlNew()
			if err := html.WriteContext(context.Background(), 10, fixture.Source); err == nil {
				t.Fatalf("%s was not rejected", fixture.Name)
			}
		case "strict-unified-plannable":
			planner := mustNewPDFDocument(WithUnit(UnitPoint))
			if plan, err := planner.planCompiledHTML(10, compiled); err != nil || plan.Hash() == "" {
				t.Fatalf("%s strict plan=%q %v", fixture.Name, plan.Hash(), err)
			}
		default:
			pdf := characterizationPDF()
			html := pdf.htmlNew()
			if err := html.WriteContext(context.Background(), 10, fixture.Source); err != nil {
				t.Fatalf("%s render: %v", fixture.Name, err)
			}
		}
	}
	for _, class := range htmlCharacterization().BehaviorClasses {
		if !seen[class] {
			t.Fatalf("classification %q has no fixture", class)
		}
	}
}

func TestRunHTMLCharacterizationRecordsDeterministicPlanEvidence(t *testing.T) {
	first, err := runHTMLCharacterization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	second, err := runHTMLCharacterization(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := first.CanonicalJSON()
	b, _ := second.CanonicalJSON()
	if !reflect.DeepEqual(a, b) || len(first.Fixtures) != len(htmlCharacterization().Fixtures) {
		t.Fatalf("HTML evidence is incomplete or nondeterministic:\n%s\n%s", a, b)
	}
	for _, fixture := range first.Fixtures {
		rendered := fixture.Outcome == "rendered" || fixture.Outcome == "planned"
		if rendered && fixture.Pages == 0 {
			t.Fatalf("rendered fixture lacks page evidence: %+v", fixture)
		}
		if !rendered && fixture.Pages != 0 {
			t.Fatalf("non-rendered fixture fabricated page evidence: %+v", fixture)
		}
		if fixture.Outcome == "planned" && len(fixture.ReadingRoles) == 0 {
			t.Fatalf("unified fixture lacks reading order: %+v", fixture)
		}
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := runHTMLCharacterization(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled HTML characterization = %v", err)
	}
}

func TestHTMLCharacterizationCursorLimitsCancellationAndConcurrentReuse(t *testing.T) {
	pdf := characterizationPDF()
	pdf.SetXY(24, 30)
	startPage, startY := pdf.PageCount(), pdf.GetY()
	html := pdf.htmlNew()
	if err := html.WriteContext(context.Background(), 10, "<p>cursor integration</p>"); err != nil || pdf.PageCount() < startPage || pdf.GetY() == startY {
		t.Fatalf("cursor pages=%d y=%g err=%v", pdf.PageCount(), pdf.GetY(), err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	other := characterizationPDF()
	otherHTML := other.htmlNew()
	if err := otherHTML.WriteContext(canceled, 10, "<p>cancel</p>"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation=%v", err)
	}
	limited := characterizationPDF()
	limitedHTML := limited.htmlNew()
	limitedHTML.MaxHTMLBytes = 8
	if err := limitedHTML.WriteContext(context.Background(), 10, "<p>too much content</p>"); !errors.Is(err, errHTMLLimitExceeded) {
		t.Fatalf("limit=%v", err)
	}
	tableLimited := characterizationPDF()
	tableHTML := tableLimited.htmlNew()
	tableHTML.MaxTableRows = 1
	if err := tableHTML.WriteContext(context.Background(), 10, "<table><tr><td>1</td></tr><tr><td>2</td></tr></table>"); err == nil || !strings.Contains(err.Error(), "row count exceeds") {
		t.Fatalf("table limit=%v", err)
	}
	compiled, err := compileHTML("<section><p>concurrent compiled reuse</p></section>")
	if err != nil {
		t.Fatal(err)
	}
	template, err := compileHTMLTemplate("<p>Hello {{name}}</p>")
	if err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	failures := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			target := characterizationPDF()
			renderer := target.htmlNew()
			renderer.WriteCompiled(10, compiled)
			if err := renderer.WriteTemplateContext(context.Background(), 10, template, htmlTemplateValues{"name": string(rune('A' + index%26))}); err != nil {
				failures <- err
				return
			}
			if err := target.Error(); err != nil {
				failures <- err
			}
		}(i)
	}
	wait.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
}

func TestHTMLCharacterizationRenderedCohortsMatchExplicitPlanCursor(t *testing.T) {
	for _, fixture := range htmlCharacterizationFixtures() {
		if fixture.Classification != "recognized-rendered" && fixture.Classification != "recognized-ignored-metadata" && fixture.Classification != "strict-unified-plannable" {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			compiled, err := compileHTML(fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			planner := newHTMLCharacterizationDocument(true)
			plannerHTML := planner.htmlNew()
			fragment, err := plannerHTML.planCompiledHTMLFragmentContext(t.Context(), 10, compiled)
			if err != nil {
				t.Fatal(err)
			}
			if fragment.plan.Hash() == "" || fragment.plan.PageCount() == 0 {
				t.Fatalf("characterization fragment plan is incomplete: hash=%q pages=%d", fragment.plan.Hash(), fragment.plan.PageCount())
			}
			direct := newHTMLCharacterizationDocument(true)
			renderer := direct.htmlNew()
			renderer.WriteCompiled(10, compiled)
			if err := direct.Error(); err != nil {
				t.Fatal(err)
			}
			if direct.PageCount() != fragment.final.page || direct.PageNo() != fragment.final.page ||
				direct.GetX() != fragment.final.x || direct.GetY() != fragment.final.y {
				t.Fatalf("direct cursor differs from immutable fragment plan: page=%d/%d xy=%.9f,%.9f want page=%d xy=%.9f,%.9f",
					direct.PageCount(), direct.PageNo(), direct.GetX(), direct.GetY(),
					fragment.final.page, fragment.final.x, fragment.final.y)
			}
		})
	}
}

func characterizationPDF() *pdfDocument {
	pdf := mustNewPDFDocument(WithUnit(UnitPoint), WithCustomPageSize(Size{Wd: 200, Ht: 160}), WithNoCompression())
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 10)
	return pdf
}
