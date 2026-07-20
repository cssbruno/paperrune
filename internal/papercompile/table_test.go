package papercompile

import (
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/paperlang"
	"github.com/cssbruno/paperrune/layout"
)

const paperTableSource = "document @report:\n  page @sheet:\n    body @body:\n      table @ledger:\n        caption: \"Ledger\"\n        repeat-header: true\n        split: \"rows\"\n        table-column @name-column:\n          width: 60pt\n        table-column @value-column:\n          width: 40pt\n        table-header @head:\n          table-row @head-row:\n            cell @name-head:\n              text: \"Name\"\n            cell @value-head:\n              text: \"Value\"\n        table-row @body-row:\n          cell @name:\n            text: \"Alpha\"\n          cell @value:\n            colspan: 1\n            paragraph:\n              text: \"10\"\n"

func TestCompileReadableTable(t *testing.T) {
	parsed := paperlang.Parse("table.paper", paperTableSource)
	result := Compile(parsed.AST)
	if !parsed.OK() || !result.OK() {
		t.Fatalf("diagnostics=%#v/%#v", parsed.Diagnostics, result.Diagnostics)
	}
	table := result.Document.Body[0].(layout.TableBlock)
	if table.Caption != "Ledger" || !table.Style.RepeatHeader || len(table.Columns) != 2 || len(table.Header) != 1 || len(table.Body) != 1 || !table.Header[0].Cells[0].Header {
		t.Fatalf("table=%#v", table)
	}
}

func TestCompileTableRetainsRowOrphansAndWidows(t *testing.T) {
	source := strings.Replace(paperTableSource, "table-row @body-row:\n", "table-row @body-row:\n          orphans: 2\n          widows: 3\n", 1)
	parsed := paperlang.Parse("table-pagination.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics=%#v/%#v", parsed.Diagnostics, compiled.Diagnostics)
	}
	row := compiled.Document.Body[0].(layout.TableBlock).Body[0]
	if row.Orphans != 2 || row.Widows != 3 {
		t.Fatalf("row pagination = %#v", row)
	}
}

func TestCompileTableCellHeaderAttributeDoesNotCollideWithHeaderRegion(t *testing.T) {
	source := strings.Replace(paperTableSource, "cell @name:\n", "cell @name:\n            header-cell: true\n", 1)
	parsed := paperlang.Parse("header-cell.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics=%#v/%#v", parsed.Diagnostics, compiled.Diagnostics)
	}
	if !compiled.Document.Body[0].(layout.TableBlock).Body[0].Cells[0].Header {
		t.Fatal("header-cell attribute was not retained")
	}
}

func TestCompileScenarioRendersDirectTableCellBinding(t *testing.T) {
	const source = `document @report:
  schema invoice:
    string item
  scenario @sample:
    value @item: "Bound item"
  page:
    body:
      table:
        table-column:
          width: 100%
        table-row:
          cell @bound-cell:
            bind: "item"
            text: "Placeholder"
`
	parsed := paperlang.Parse("bound-cell.paper", source)
	compiled := CompileScenario(parsed.AST, "sample")
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics=%#v/%#v", parsed.Diagnostics, compiled.Diagnostics)
	}
	cell := compiled.Document.Body[0].(layout.TableBlock).Body[0].Cells[0]
	if len(cell.Blocks) != 1 || layout.TextSegmentsPlainText(cell.Blocks[0].(layout.ParagraphBlock).Segments) != "Bound item" {
		t.Fatalf("bound cell = %#v", cell)
	}
}

func TestCompileTablePreservesContainerRelativeTracks(t *testing.T) {
	source := strings.Replace(paperTableSource, "width: 60pt", "width: 50%", 1)
	source = strings.Replace(source, "width: 40pt", "width: 50%", 1)
	parsed := paperlang.Parse("responsive-table.paper", source)
	result := Compile(parsed.AST)
	if !parsed.OK() || !result.OK() {
		t.Fatalf("diagnostics=%#v/%#v", parsed.Diagnostics, result.Diagnostics)
	}
	table := result.Document.Body[0].(layout.TableBlock)
	if table.Columns[0].Width != 0 || table.Columns[0].WidthPercent != 50_000_000 ||
		table.Columns[1].Width != 0 || table.Columns[1].WidthPercent != 50_000_000 {
		t.Fatalf("columns = %#v", table.Columns)
	}
}

func TestCompileTablePreservesExplicitEmptyCells(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      table:\n        table-column:\n          width: 50%\n        table-column:\n          width: 50%\n        table-row:\n          cell:\n            colspan: 1\n          cell:\n            text: \"value\"\n"
	parsed := paperlang.Parse("empty-cell.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	table := compiled.Document.Body[0].(layout.TableBlock)
	if len(table.Body) != 1 || len(table.Body[0].Cells) != 2 || len(table.Body[0].Cells[0].Blocks) != 0 {
		t.Fatalf("table = %#v", table)
	}
}
