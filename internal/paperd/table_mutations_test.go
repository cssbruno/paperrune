package paperd

import (
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/cssbruno/paperrune/internal/paperedit"
)

const tableMutationFixture = "document @report:\n  page @sheet:\n    body @body:\n      table @ledger:\n        repeat-header: true\n        split: \"rows\"\n        table-column @name-column:\n          width: 60pt\n        table-row @body-row:\n          cell @name:\n            text: \"Alpha\"\n"

const componentTableMutationFixture = "document @report:\n  component @table-card:\n    table @template-table:\n      split: \"rows\"\n      table-column @template-column:\n        width: 60pt\n      table-row @template-row:\n        cell @template-cell:\n          text: \"Alpha\"\n  page @sheet:\n    body @body:\n      use @instance:\n        component: \"@table-card\"\n"

func TestPaperSetTablePropertyUsesExactTableGuardAndMinimalPatch(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, tableMutationFixture, "@name-column", "table-column", CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	result, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableColumnWidth, Points: 72})
	if err != nil || !result.Revision.CompileOK || result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 1 || !strings.Contains(result.Revision.Source, "width: 72pt") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestPaperSetTablePropertyAcceptsContainerRelativeTrackWidth(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, tableMutationFixture, "@name-column", "table-responsive-column", CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	result, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableColumnWidth, Length: "50%"})
	if err != nil || !result.Revision.CompileOK || !strings.Contains(result.Revision.Source, "width: 50%") {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestPaperSetTablePropertyEditsCaptionAndCellAlignment(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	captionGuard, _, _ := mutationGuard(t, workspace, tableMutationFixture, "@ledger", "table-caption", CapabilityEdit)
	caption, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: captionGuard, Property: PaperTableCaption, Text: "Quarterly results"})
	if err != nil || !caption.Revision.CompileOK || !strings.Contains(caption.Revision.Source, "caption: \"Quarterly results\"") {
		t.Fatalf("caption=%#v err=%v", caption, err)
	}

	cellGuard, _, _ := mutationGuard(t, workspace, tableMutationFixture, "@name", "cell-align", CapabilityEdit)
	cellGuard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	cell, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: cellGuard, Property: PaperTableCellAlign, Kind: "middle"})
	if err != nil || !cell.Revision.CompileOK || !strings.Contains(cell.Revision.Source, "vertical-align: \"middle\"") {
		t.Fatalf("cell=%#v err=%v", cell, err)
	}
}

func TestPaperSetTablePropertyEditsRowPaginationAndCellSpans(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, tableMutationFixture, "@body-row", "row-orphans", CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	row, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableRowOrphans, Count: 3})
	if err != nil || !strings.Contains(row.Revision.Source, "orphans: 3") {
		t.Fatalf("row pagination = %#v, %v", row, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, tableMutationFixture, "@name", "cell-colspan", CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	cell, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableCellColSpan, Count: 2})
	if err != nil || !strings.Contains(cell.Revision.Source, "colspan: 2") {
		t.Fatalf("cell span = %#v, %v", cell, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, tableMutationFixture, "@name", "cell-colspan-invalid", CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	if _, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableCellColSpan, Count: 1025}); errorCode(err) != "INVALID_TABLE_VALUE" {
		t.Fatalf("oversized span error = %v", err)
	}
}

func TestPaperSetTablePropertyRejectsMissingStaleAndAdversarialGuards(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, created, _ := mutationGuard(t, workspace, tableMutationFixture, "@name-column", "table-invalid", CapabilityEdit)
	if _, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableColumnWidth, Points: 72}); errorCode(err) != "TRANSITIVE_PRECONDITION_REQUIRED" {
		t.Fatalf("missing guard=%v", err)
	}
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	guard.TargetPreconditions[0].ExpectedFingerprint = paperedit.NodeFingerprint(strings.Repeat("0", 64))
	if _, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableColumnWidth, Points: 72}); errorCode(err) != "TRANSITIVE_PRECONDITION_CONFLICT" {
		t.Fatalf("stale guard=%v", err)
	}
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	if _, err := workspace.PaperSetTableProperty(PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableColumnWidth, Points: math.NaN()}); err == nil {
		t.Fatal("NaN accepted")
	}
	if candidate, _ := workspace.Candidate(created.Candidate.Handle); candidate.Head != created.Revision.Handle {
		t.Fatal("invalid mutation advanced head")
	}
}

func TestPaperSetTablePropertyIdempotentRace(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, tableMutationFixture, "@name-column", "table-race", CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", tableMutationFixture, "@ledger")}
	request := PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableColumnWidth, Points: 72}
	var wait sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wait.Add(1)
		go func() { defer wait.Done(); _, err := workspace.PaperSetTableProperty(request); errors <- err }()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("race=%v", err)
		}
	}
}

func TestPaperSetTablePropertyAuthorizesProtectedComponentBlastRadius(t *testing.T) {
	workspace := authorizationWorkspace(t, WorkspaceOptions{ProtectedNodeIDs: []string{"@table-card"}})
	guard, _, opened := mutationGuard(t, workspace, componentTableMutationFixture, "@template-column", "table-protected", CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", componentTableMutationFixture, "@template-table")}
	request := PaperSetTablePropertyRequest{Guard: guard, Property: PaperTableColumnWidth, Points: 72}
	request.Guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:table", []MutationOperation{MutationSetTableProperty}, []string{"@table-card", "@instance"}, nil)
	if _, err := workspace.PaperSetTableProperty(request); errorCode(err) != "PROTECTED_NODE_DENIED" {
		t.Fatalf("missing protected grant=%v", err)
	}
	request.Guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:table", []MutationOperation{MutationSetTableProperty}, []string{"@table-card", "@instance"}, []string{"@table-card"})
	result, err := workspace.PaperSetTableProperty(request)
	if err != nil || !result.Authorization.Allowed || len(result.Authorization.Effects) < 2 {
		t.Fatalf("authorized=%#v %v", result.Authorization, err)
	}
}
