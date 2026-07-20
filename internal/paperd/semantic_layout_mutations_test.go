// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/cssbruno/paperrune/internal/paperedit"
)

const boxMutationFixture = "document @report:\n" +
	"  page @sheet:\n" +
	"    body @body:\n" +
	"      paragraph @box:\n" +
	"        padding: 4pt\n" +
	"        text @copy: \"Box\"\n"

const invalidFontMutationFixture = "document @report:\n" +
	"  page @sheet:\n" +
	"    body @body:\n" +
	"      paragraph @copy:\n" +
	"        font: \"Unavailable Sans\"\n" +
	"        text: \"Strict font\"\n"

const layoutMutationFixture = "document @report:\n" +
	"  page @sheet:\n" +
	"    body @body:\n" +
	"      row @grid:\n" +
	"        gap: 4pt\n" +
	"        paragraph @left:\n" +
	"          width: 40pt\n" +
	"          text @left-copy: \"Left\"\n" +
	"        paragraph @right:\n" +
	"          width: 1fr\n" +
	"          text @right-copy: \"Right\"\n"

const pageMarginMutationFixture = "document @report:\n  page @sheet:\n    width: 200pt\n    height: 120pt\n    margin: 8pt\n    body @body:\n      paragraph @copy:\n        text: \"Page\"\n"

const canvasMutationFixture = "document @report:\n  page @sheet:\n    body @body:\n      canvas @diagram:\n        width: 160pt\n        height: 80pt\n        anchor @base:\n          width: 40pt\n          height: 20pt\n          left: \"canvas.left\"\n          top: \"canvas.top\"\n        anchor @badge:\n          width: 24pt\n          height: 12pt\n          left: \"@base.right\"\n          top: \"@base.top\"\n"

const listMutationFixture = "document @report:\n  page @sheet:\n    body @body:\n      list @steps:\n        ordered: false\n        marker: \"dash\"\n        item @first:\n          text: \"Measure twice\"\n"

const headingMutationFixture = "document @report:\n  page @sheet:\n    body @body:\n      heading @title:\n        level: 1\n        text: \"Report\"\n"

func TestPaperSetCanvasItemIsReadableTransitiveAndAuthorized(t *testing.T) {
	workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
	guard, _, opened := mutationGuard(t, workspace, canvasMutationFixture, "@badge", "canvas-anchor", CapabilityEdit)
	fingerprint, err := paperedit.FingerprintNode("test.paper", canvasMutationFixture, "@diagram")
	if err != nil {
		t.Fatal(err)
	}
	instance, err := paperedit.SourceInstance("test.paper", canvasMutationFixture, "@diagram")
	if err != nil {
		t.Fatal(err)
	}
	guard.TargetPreconditions = []paperedit.TargetPrecondition{{Target: "@diagram", ExpectedFingerprint: fingerprint, ExpectedInstance: instance}}
	guard.Authority = grantMutationAuthority(t, workspace, opened, "studio:canvas", []MutationOperation{MutationSetCanvasItem}, []string{"@badge", "@diagram"}, nil)
	result, err := workspace.PaperSetCanvasItem(PaperSetCanvasItemRequest{Guard: guard, Property: string(PaperCanvasLeft), Reference: "@base", TargetAnchor: PaperCanvasRight, Offset: 8})
	if err != nil || !result.Authorization.Allowed || result.Authorization.Operation != MutationSetCanvasItem ||
		result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 1 || !strings.Contains(result.Revision.Source, `left: "@base.right + 8pt"`) {
		t.Fatalf("canvas anchor = %#v, %v", result, err)
	}
}

func TestPaperSetPageMarginIsReadableMinimalAndAuthorized(t *testing.T) {
	workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
	guard, _, opened := mutationGuard(t, workspace, pageMarginMutationFixture, "@sheet", "page-margin", CapabilityEdit)
	guard.Authority = grantMutationAuthority(t, workspace, opened, "studio:page-master", []MutationOperation{MutationSetPageMargin}, []string{"@sheet"}, nil)
	result, err := workspace.PaperSetPageMargin(PaperSetPageMarginRequest{Guard: guard, Property: PaperPageMarginLeft, Points: 16})
	if err != nil || !result.Authorization.Allowed || result.Authorization.Operation != MutationSetPageMargin ||
		result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 1 || !strings.Contains(result.Revision.Source, "margin-left: 16pt") {
		t.Fatalf("page margin = %#v, %v", result, err)
	}
}

func TestPaperSetTextPropertyEditsOnlyHeadingLevelWithinBounds(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, headingMutationFixture, "@title", "heading-level", CapabilityEdit)
	result, err := workspace.PaperSetTextProperty(PaperSetTextPropertyRequest{Guard: guard, Property: PaperTextLevel, Count: 6})
	if err != nil || !strings.Contains(result.Revision.Source, "level: 6") {
		t.Fatalf("heading level = %#v, %v", result, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, headingMutationFixture, "@title", "heading-level-invalid", CapabilityEdit)
	if _, err := workspace.PaperSetTextProperty(PaperSetTextPropertyRequest{Guard: guard, Property: PaperTextLevel, Count: 7}); errorCode(err) != "INVALID_TEXT_STYLE_VALUE" {
		t.Fatalf("invalid heading level error = %v", err)
	}
}

func TestPaperSetPageSizeWritesTwoExactDimensions(t *testing.T) {
	workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
	guard, _, opened := mutationGuard(t, workspace, pageMarginMutationFixture, "@sheet", "page-size", CapabilityEdit)
	guard.Authority = grantMutationAuthority(t, workspace, opened, "studio:page-size", []MutationOperation{MutationSetPageSize}, []string{"@sheet"}, nil)
	result, err := workspace.PaperSetPageSize(PaperSetPageSizeRequest{Guard: guard, WidthPoints: 595.275590551, HeightPoints: 841.88976378})
	if err != nil || !result.Authorization.Allowed || result.Authorization.Operation != MutationSetPageSize ||
		result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 2 || !strings.Contains(result.Revision.Source, "width: 595.275590551pt") || !strings.Contains(result.Revision.Source, "height: 841.88976378pt") {
		t.Fatalf("page size = %#v, %v", result, err)
	}
}

func TestPaperSetPageRegionGuardsGoverningPage(t *testing.T) {
	source := "document @report:\n  page @sheet:\n    header @head:\n      paragraph @copy:\n        text: \"Header\"\n    body @body:\n      paragraph @main:\n        text: \"Body\"\n"
	workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
	guard, _, opened := mutationGuard(t, workspace, source, "@head", "page-region", CapabilityEdit)
	fingerprint, _ := paperedit.FingerprintNode("test.paper", source, "@sheet")
	instance, _ := paperedit.SourceInstance("test.paper", source, "@sheet")
	guard.TargetPreconditions = []paperedit.TargetPrecondition{{Target: "@sheet", ExpectedFingerprint: fingerprint, ExpectedInstance: instance}}
	guard.Authority = grantMutationAuthority(t, workspace, opened, "studio:region", []MutationOperation{MutationSetPageRegion}, []string{"@head", "@sheet"}, nil)
	result, err := workspace.PaperSetPageRegion(PaperSetPageRegionRequest{Guard: guard, Property: "background", Color: "#AABBCC"})
	if err != nil || result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 1 || !strings.Contains(result.Revision.Source, `background: "#aabbcc"`) {
		t.Fatalf("page region = %#v, %v", result, err)
	}
}

func TestPaperSetListPropertyKeepsOrderingAndMarkerConsistent(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, listMutationFixture, "@steps", "ordered-list", CapabilityEdit)
	ordered, err := workspace.PaperSetListProperty(PaperSetListPropertyRequest{Guard: guard, Property: PaperListOrdered, Bool: true})
	if err != nil || !ordered.Revision.CompileOK || !strings.Contains(ordered.Revision.Source, "ordered: true") || !strings.Contains(ordered.Revision.Source, `marker: "decimal"`) || len(ordered.Edit.Diff.Patches) != 2 {
		t.Fatalf("ordered list = %#v, %v", ordered, err)
	}

	guard, _, _ = mutationGuard(t, workspace, listMutationFixture, "@steps", "asterisk-list", CapabilityEdit)
	asterisk, err := workspace.PaperSetListProperty(PaperSetListPropertyRequest{Guard: guard, Property: PaperListMarker, Marker: "asterisk"})
	if err != nil || !asterisk.Revision.CompileOK || !strings.Contains(asterisk.Revision.Source, `marker: "asterisk"`) || strings.Contains(asterisk.Revision.Source, "ordered: true") {
		t.Fatalf("asterisk list = %#v, %v", asterisk, err)
	}
}

func TestPaperTypographyAndBoxControlsCoverTableCellsAndCanvasItems(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	cellGuard, _, _ := mutationGuard(t, workspace, tableMutationFixture, "@name", "cell-typography", CapabilityEdit)
	cell, err := workspace.PaperSetTextProperty(PaperSetTextPropertyRequest{Guard: cellGuard, Property: PaperTextAlign, Kind: "right"})
	if err != nil || !cell.Revision.CompileOK || !strings.Contains(cell.Revision.Source, `align: "right"`) {
		t.Fatalf("cell typography = %#v, %v", cell, err)
	}

	anchorGuard, _, _ := mutationGuard(t, workspace, canvasMutationFixture, "@badge", "anchor-box", CapabilityEdit)
	anchor, err := workspace.PaperSetBoxProperty(PaperSetBoxPropertyRequest{Guard: anchorGuard, Property: PaperBoxPadding, Points: 2})
	if err != nil || !anchor.Revision.CompileOK || !strings.Contains(anchor.Revision.Source, "padding: 2pt") {
		t.Fatalf("canvas item box = %#v, %v", anchor, err)
	}
}

func TestPaperSetBoxPropertyIsTypedMinimalAndAuthorized(t *testing.T) {
	tests := []struct {
		name     string
		property PaperBoxProperty
		points   float64
		color    string
		want     string
	}{
		{"margin", PaperBoxMarginLeft, 6, "", "margin-left: 6pt"},
		{"padding", PaperBoxPadding, 8.5, "", "padding: 8.5pt"},
		{"border", PaperBoxBorderWidth, 1.25, "", "border-width: 1.25pt"},
		{"radius", PaperBoxRadius, 4, "", "border-radius: 4pt"},
		{"background", PaperBoxBackground, 0, "#AABBCC", `background: "#aabbcc"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
			guard, _, opened := mutationGuard(t, workspace, boxMutationFixture, "@box", "box-"+test.name, CapabilityEdit)
			guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:box", []MutationOperation{MutationSetBoxProperty}, []string{"@box"}, nil)
			result, err := workspace.PaperSetBoxProperty(PaperSetBoxPropertyRequest{Guard: guard, Property: test.property, Points: test.points, Color: test.color})
			if err != nil {
				t.Fatalf("PaperSetBoxProperty() error = %v", err)
			}
			if !result.Revision.CompileOK || result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 1 || !strings.Contains(result.Revision.Source, test.want) {
				t.Fatalf("box mutation = %#v", result)
			}
			if !result.Authorization.Explicit || !result.Authorization.Allowed || result.Authorization.Operation != MutationSetBoxProperty {
				t.Fatalf("authorization = %#v", result.Authorization)
			}
		})
	}
}

func TestPaperSetTextPropertyAuthorsCompleteTypography(t *testing.T) {
	tests := []struct {
		name    string
		request PaperSetTextPropertyRequest
		want    string
	}{
		{name: "size", request: PaperSetTextPropertyRequest{Property: PaperTextSize, Length: "12pt"}, want: "size: 12pt"},
		{name: "line-height", request: PaperSetTextPropertyRequest{Property: PaperTextLineHeight, Points: 15}, want: "line-height: 15pt"},
		{name: "color", request: PaperSetTextPropertyRequest{Property: PaperTextColor, Color: "#123ABC"}, want: `color: "#123abc"`},
		{name: "align", request: PaperSetTextPropertyRequest{Property: PaperTextAlign, Kind: "justify"}, want: `align: "justify"`},
		{name: "bold", request: PaperSetTextPropertyRequest{Property: PaperTextBold, Bool: true}, want: "bold: true"},
		{name: "italic", request: PaperSetTextPropertyRequest{Property: PaperTextItalic, Bool: true}, want: "italic: true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
			guard, _, opened := mutationGuard(t, workspace, boxMutationFixture, "@box", "text-"+test.name, CapabilityEdit)
			guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:text", []MutationOperation{MutationSetTextProperty}, []string{"@box"}, nil)
			test.request.Guard = guard
			result, err := workspace.PaperSetTextProperty(test.request)
			if err != nil || !result.Revision.CompileOK || !strings.Contains(result.Revision.Source, test.want) {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestPaperSetTextPropertyExplicitlyRepairsUnavailableFont(t *testing.T) {
	workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
	guard, created, opened := mutationGuard(t, workspace, invalidFontMutationFixture, "@copy", "font-replacement", CapabilityEdit)
	if created.Revision.CompileOK {
		t.Fatal("unavailable font unexpectedly compiled")
	}
	guard.Authority = grantMutationAuthority(t, workspace, opened, "studio:font-replacement", []MutationOperation{MutationSetTextProperty}, []string{"@copy"}, nil)
	result, err := workspace.PaperSetTextProperty(PaperSetTextPropertyRequest{Guard: guard, Property: PaperTextFont, Text: "Helvetica"})
	if err != nil || !result.Revision.CompileOK || result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 1 ||
		!strings.Contains(result.Revision.Source, `font: "Helvetica"`) || result.Authorization.Operation != MutationSetTextProperty {
		t.Fatalf("font replacement = %#v, %v", result, err)
	}

	workspace = mustWorkspace(t, Limits{})
	invalidGuard, invalidCreated, _ := mutationGuard(t, workspace, invalidFontMutationFixture, "@copy", "font-invalid", CapabilityEdit)
	if _, err := workspace.PaperSetTextProperty(PaperSetTextPropertyRequest{Guard: invalidGuard, Property: PaperTextFont, Text: "Another Missing Font"}); err == nil {
		t.Fatal("unsupported replacement unexpectedly succeeded")
	}
	if candidate, _ := workspace.Candidate(invalidCreated.Candidate.Handle); candidate.Head != invalidCreated.Revision.Handle {
		t.Fatal("invalid replacement advanced candidate")
	}
}

func TestPaperSetBoxPropertyRejectsAdversarialPayloadsBeforePublication(t *testing.T) {
	tests := []PaperSetBoxPropertyRequest{
		{Property: PaperBoxProperty("padding\nowned"), Points: 1},
		{Property: PaperBoxPadding, Points: math.NaN()},
		{Property: PaperBoxPadding, Points: -1},
		{Property: PaperBoxPadding, Points: 1_000_001},
		{Property: PaperBoxBackground, Color: "red"},
		{Property: PaperBoxBackground, Color: "#112233\nowned"},
	}
	for index, request := range tests {
		workspace := mustWorkspace(t, Limits{})
		guard, created, _ := mutationGuard(t, workspace, boxMutationFixture, "@box", "box-invalid-"+string(rune('a'+index)), CapabilityEdit)
		request.Guard = guard
		if _, err := workspace.PaperSetBoxProperty(request); err == nil {
			t.Fatalf("request %d unexpectedly succeeded", index)
		}
		candidate, err := workspace.Candidate(created.Candidate.Handle)
		if err != nil || candidate.Head != created.Revision.Handle {
			t.Fatalf("invalid request advanced candidate: %#v, %v", candidate, err)
		}
	}
}

func layoutItemRequest(t *testing.T, workspace *Workspace, key string) (PaperSetLayoutItemRequest, PaperCreateResult, PaperOpenSnapshot) {
	t.Helper()
	guard, created, opened := mutationGuard(t, workspace, layoutMutationFixture, "@left", key, CapabilityEdit)
	guard.TargetPreconditions = []paperedit.TargetPrecondition{exactTargetPrecondition(t, "mutation.paper", layoutMutationFixture, "@grid")}
	return PaperSetLayoutItemRequest{Guard: guard, Property: PaperLayoutItemWidth, Points: 48}, created, opened
}

func TestPaperSetLayoutContainerAuthorsEveryReadableControl(t *testing.T) {
	tests := []struct {
		name     string
		property PaperLayoutContainerProperty
		points   float64
		length   string
		kind     string
		boolean  bool
		want     string
	}{
		{name: "gap", property: PaperLayoutGap, points: 8, want: "gap: 8pt"},
		{name: "line-gap", property: PaperLayoutLineGap, length: "4pt", want: "line-gap: 4pt"},
		{name: "height", property: PaperLayoutHeight, points: 80, want: "height: 80pt"},
		{name: "wrap", property: PaperLayoutWrap, kind: "wrap-reverse", want: `wrap: "wrap-reverse"`},
		{name: "justify", property: PaperLayoutJustifyContent, kind: "space-between", want: `justify-content: "space-between"`},
		{name: "items", property: PaperLayoutAlignItems, kind: "center", want: `align-items: "center"`},
		{name: "lines", property: PaperLayoutAlignContent, kind: "space-around", want: `align-content: "space-around"`},
		{name: "reverse", property: PaperLayoutReverse, boolean: true, want: "reverse: true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
			guard, _, opened := mutationGuard(t, workspace, layoutMutationFixture, "@grid", "layout-container-"+test.name, CapabilityEdit)
			guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:layout", []MutationOperation{MutationSetLayoutContainer}, []string{"@grid"}, nil)
			result, err := workspace.PaperSetLayoutContainer(PaperSetLayoutContainerRequest{
				Guard: guard, Property: test.property, Points: test.points, Length: test.length, Kind: test.kind, Bool: test.boolean,
			})
			if err != nil || !result.Revision.CompileOK || !strings.Contains(result.Revision.Source, test.want) || result.Authorization.Operation != MutationSetLayoutContainer {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestPaperSetLayoutContainerRejectsWrongAxisAndNonPhysicalDimensions(t *testing.T) {
	for index, request := range []PaperSetLayoutContainerRequest{
		{Property: PaperLayoutWidth, Points: 80},
		{Property: PaperLayoutHeight, Length: "50%"},
		{Property: PaperLayoutGap, Length: "auto"},
		{Property: PaperLayoutWrap, Kind: "sometimes"},
	} {
		workspace := mustWorkspace(t, Limits{})
		guard, created, _ := mutationGuard(t, workspace, layoutMutationFixture, "@grid", fmt.Sprintf("layout-container-invalid-%d", index), CapabilityEdit)
		request.Guard = guard
		if _, err := workspace.PaperSetLayoutContainer(request); errorCode(err) != "INVALID_LAYOUT_CONTAINER_VALUE" {
			t.Fatalf("request %d = %v", index, err)
		}
		if candidate, _ := workspace.Candidate(created.Candidate.Handle); candidate.Head != created.Revision.Handle {
			t.Fatalf("request %d advanced candidate", index)
		}
	}
}

func TestPaperSetLayoutItemRequiresExactTransitiveGuardAndCompilesBeforeCommit(t *testing.T) {
	workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
	request, created, opened := layoutItemRequest(t, workspace, "layout-normal")
	request.Guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:layout", []MutationOperation{MutationSetLayoutItem}, []string{"@grid"}, nil)
	result, err := workspace.PaperSetLayoutItem(request)
	if err != nil {
		t.Fatalf("PaperSetLayoutItem() error = %v", err)
	}
	if !result.Revision.CompileOK || result.Edit.Diff == nil || len(result.Edit.Diff.Patches) != 1 || !strings.Contains(result.Revision.Source, "width: 48pt") {
		t.Fatalf("layout mutation = %#v", result)
	}
	if result.Authorization.Operation != MutationSetLayoutItem || len(result.Authorization.DirectTargets) != 2 {
		t.Fatalf("authorization = %#v", result.Authorization)
	}

	workspace = mustWorkspace(t, Limits{})
	missing, missingCreated, _ := layoutItemRequest(t, workspace, "layout-missing-parent")
	missing.Guard.TargetPreconditions = nil
	if _, err := workspace.PaperSetLayoutItem(missing); errorCode(err) != "TRANSITIVE_PRECONDITION_REQUIRED" {
		t.Fatalf("missing parent guard = %v", err)
	}
	if candidate, _ := workspace.Candidate(missingCreated.Candidate.Handle); candidate.Head != missingCreated.Revision.Handle {
		t.Fatal("missing parent guard advanced candidate")
	}

	workspace = mustWorkspace(t, Limits{})
	stale, staleCreated, _ := layoutItemRequest(t, workspace, "layout-stale-parent")
	stale.Guard.TargetPreconditions[0].ExpectedFingerprint = paperedit.NodeFingerprint(strings.Repeat("0", 64))
	if _, err := workspace.PaperSetLayoutItem(stale); errorCode(err) != "TRANSITIVE_PRECONDITION_CONFLICT" {
		t.Fatalf("stale parent guard = %v", err)
	}
	if candidate, _ := workspace.Candidate(staleCreated.Candidate.Handle); candidate.Head != staleCreated.Revision.Handle {
		t.Fatal("stale parent guard advanced candidate")
	}

	workspace = mustWorkspace(t, Limits{})
	invalid, invalidCreated, _ := layoutItemRequest(t, workspace, "layout-invalid-candidate")
	invalid.Property, invalid.Length, invalid.Points = PaperLayoutItemWidth, "1.5fr", 0
	if _, err := workspace.PaperSetLayoutItem(invalid); errorCode(err) != "INVALID_LAYOUT_ITEM_VALUE" {
		t.Fatalf("invalid compiled candidate = %v", err)
	}
	if candidate, _ := workspace.Candidate(invalidCreated.Candidate.Handle); candidate.Head != invalidCreated.Revision.Handle {
		t.Fatal("invalid compiled candidate advanced head")
	}
	_ = created
}

func TestPaperSetLayoutItemAcceptsResponsiveAndAutomaticSizes(t *testing.T) {
	for _, test := range []struct {
		name   string
		length string
		want   string
	}{{"percentage", "50%", "width: 50%"}, {"automatic", "auto", `width: "auto"`}, {"fraction", "2fr", "width: 2fr"}} {
		t.Run(test.name, func(t *testing.T) {
			workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
			request, _, opened := layoutItemRequest(t, workspace, "layout-responsive-"+test.name)
			request.Points, request.Length = 0, test.length
			request.Guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:layout", []MutationOperation{MutationSetLayoutItem}, []string{"@grid"}, nil)
			result, err := workspace.PaperSetLayoutItem(request)
			if err != nil || !result.Revision.CompileOK || !strings.Contains(result.Revision.Source, test.want) {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}
}

func TestPaperSetLayoutItemAuthorsFlexFactorsAndCrossAxisConstraints(t *testing.T) {
	tests := []struct {
		name     string
		property PaperLayoutItemProperty
		length   string
		kind     string
		factor   float64
		want     string
	}{
		{name: "grow", property: PaperLayoutItemFlexGrow, factor: 1.5, want: "flex-grow: 1.5"},
		{name: "shrink-zero", property: PaperLayoutItemFlexShrink, factor: 0, want: "flex-shrink: 0"},
		{name: "height", property: PaperLayoutItemHeight, length: "50%", want: "height: 50%"},
		{name: "min-height", property: PaperLayoutItemMinHeight, length: "20%", want: "min-height: 20%"},
		{name: "max-height", property: PaperLayoutItemMaxHeight, length: "80%", want: "max-height: 80%"},
		{name: "align-self", property: PaperLayoutItemAlignSelf, kind: "stretch", want: `align-self: "stretch"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			workspace := authorizationWorkspace(t, WorkspaceOptions{RequireMutationAuthority: true})
			request, _, opened := layoutItemRequest(t, workspace, "layout-flex-"+test.name)
			request.Property, request.Points, request.Length, request.Kind, request.Factor = test.property, 0, test.length, test.kind, test.factor
			request.Guard.Authority = grantMutationAuthority(t, workspace, opened, "agent:layout", []MutationOperation{MutationSetLayoutItem}, []string{"@grid"}, nil)
			result, err := workspace.PaperSetLayoutItem(request)
			if err != nil || !result.Revision.CompileOK || !strings.Contains(result.Revision.Source, test.want) {
				t.Fatalf("result=%#v err=%v", result, err)
			}
		})
	}

	workspace := mustWorkspace(t, Limits{})
	invalid, created, _ := layoutItemRequest(t, workspace, "layout-flex-invalid")
	invalid.Property, invalid.Points, invalid.Factor = PaperLayoutItemFlexGrow, 0, 0.1234567
	if _, err := workspace.PaperSetLayoutItem(invalid); errorCode(err) != "INVALID_LAYOUT_ITEM_VALUE" {
		t.Fatalf("invalid factor = %v", err)
	}
	if candidate, _ := workspace.Candidate(created.Candidate.Handle); candidate.Head != created.Revision.Handle {
		t.Fatal("invalid flex factor advanced candidate")
	}
}

func TestPaperSetLayoutItemIdempotentRace(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	request, _, _ := layoutItemRequest(t, workspace, "layout-race")
	const workers = 8
	results := make(chan PaperMutationResult, workers)
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := workspace.PaperSetLayoutItem(request)
			results <- result
			errors <- err
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("concurrent mutation error = %v", err)
		}
	}
	var revision paperedit.Revision
	for result := range results {
		if revision == "" {
			revision = result.Revision.Revision
		} else if result.Revision.Revision != revision {
			t.Fatalf("concurrent result revision = %q, want %q", result.Revision.Revision, revision)
		}
	}
}
