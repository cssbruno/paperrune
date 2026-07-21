// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"context"
	"testing"

	"github.com/cssbruno/paperrune/internal/layout"
	"github.com/cssbruno/paperrune/internal/layoutengine"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

func TestCompileRowColumnTracksAndSourceMappings(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      column @stack:\n        gap: 6pt\n        align-items: \"center\"\n        heading @title:\n          height: 20pt\n          level: 2\n          text @title-copy: \"Title\"\n        paragraph @body-copy:\n          height: 3fr\n          min-height: 12pt\n          align-self: \"end\"\n          text: \"Body\"\n"
	parsed := paperlang.Parse("column.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	block, ok := compiled.Document.Body[0].(layout.RowColumnBlock)
	if !ok || block.Direction != layout.ColumnDirection || block.Gap != 6 || block.CrossAlign != "center" || len(block.Items) != 2 {
		t.Fatalf("compiled block = %#v", compiled.Document.Body[0])
	}
	if block.Items[0].Track.Kind != layout.RowColumnTrackFixed || block.Items[0].Track.Size != 20 ||
		block.Items[1].Track.Kind != layout.RowColumnTrackFraction || block.Items[1].Track.Weight != 3 || block.Items[1].Track.Min != 12 {
		t.Fatalf("compiled tracks = %#v", block.Items)
	}
	mappings := map[string]NodeMapping{}
	for _, mapping := range compiled.Mapping.Nodes {
		mappings[mapping.ID] = mapping
	}
	if mappings["@stack"].BodyIndex != 0 || mappings["@stack"].SegmentIndex != -1 ||
		mappings["@title"].SegmentIndex != 0 || mappings["@body-copy"].SegmentIndex != 1 ||
		mappings["@title-copy"].NestedBlockIndex != 0 {
		t.Fatalf("mappings = %+v", compiled.Mapping.Nodes)
	}
}

func TestCompileRowColumnRejectsRemovedTrackVocabulary(t *testing.T) {
	parsed := paperlang.Parse("bad-track.paper", "document:\n  page:\n    body:\n      row:\n        paragraph:\n          track-size: 1fr\n          text: \"removed property\"\n")
	compiled := Compile(parsed.AST)
	if !parsed.OK() || compiled.OK() {
		t.Fatalf("parse/compile = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
}

func TestCompileRowColumnDiagnosesInvalidFractionSize(t *testing.T) {
	for _, size := range []string{"0fr", "1.5fr"} {
		parsed := paperlang.Parse("bad-fraction.paper", "document:\n  page:\n    body:\n      row:\n        paragraph:\n          width: "+size+"\n          text: \"bad fraction\"\n")
		compiled := Compile(parsed.AST)
		if !parsed.OK() || compiled.OK() {
			t.Fatalf("size %s parse/compile = %+v / %+v", size, parsed.Diagnostics, compiled.Diagnostics)
		}
	}
}

func TestCompileRowColumnPreservesContainerRelativeAndAutoTrackSizes(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      row:\n        paragraph:\n          width: 50%\n          min-width: 20%\n          text: \"Half\"\n        paragraph:\n          width: \"auto\"\n          max-width: 40%\n          text: \"Intrinsic\"\n"
	parsed := paperlang.Parse("responsive-row.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	row := compiled.Document.Body[0].(layout.RowColumnBlock)
	first, second := row.Items[0].Track, row.Items[1].Track
	if first.Kind != layout.RowColumnTrackFlex || first.BasisKind != layout.RowColumnFlexBasisPercent ||
		first.BasisPercent != 50_000_000 || first.MinPercent != 20_000_000 || first.Shrink != 1 {
		t.Fatalf("percentage track = %#v", first)
	}
	if second.Kind != layout.RowColumnTrackFlex || second.BasisKind != layout.RowColumnFlexBasisContent ||
		second.MaxPercent != 40_000_000 || second.Shrink != 1 {
		t.Fatalf("automatic track = %#v", second)
	}
	tree, err := LowerLayoutDocumentTreeContext(context.Background(), compiled.Document, layoutengine.CanonicalTreeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	projection := tree.Projection()
	foundPercent := false
	for _, track := range projection.Tracks {
		foundPercent = foundPercent || track.Max.Kind == "percent" && track.Max.Value == 512
	}
	if !foundPercent {
		t.Fatalf("canonical tracks lost 50%% basis: %+v", projection.Tracks)
	}
}

func TestTypedTreePercentCoversFullUint32Domain(t *testing.T) {
	got := typedTreePercent(^uint32(0))
	if got.Kind != layoutengine.TreeLengthPercent || got.Value != 43_980 {
		t.Fatalf("typedTreePercent(max uint32) = %#v, want percent 43980", got)
	}
}

func TestCompileRowColumnAcceptsResponsiveImageChildren(t *testing.T) {
	source := "document:\n  page:\n    body:\n      row @media:\n        image @hero:\n          width: 40%\n          source: \"data:image/png;base64," + paperImagePNG + "\"\n          height: \"auto\"\n          alt: \"Evidence\"\n        paragraph @copy:\n          width: 60%\n          text: \"Caption\"\n"
	parsed := paperlang.Parse("row-image.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	row := compiled.Document.Body[0].(layout.RowColumnBlock)
	image, ok := row.Items[0].Block.(layout.ImageBlock)
	if !ok || image.WidthPercent != 100_000_000 || row.Items[0].Track.BasisPercent != 40_000_000 || len(image.Data) == 0 {
		t.Fatalf("row image = %#v", row.Items[0])
	}
	mappings := map[string]NodeMapping{}
	for _, mapping := range compiled.Mapping.Nodes {
		mappings[mapping.ID] = mapping
	}
	if mappings["@hero"].BodyIndex != 0 || mappings["@hero"].SegmentIndex != 0 || mappings["@hero"].ResourceDigest == "" {
		t.Fatalf("image mapping = %#v", mappings["@hero"])
	}
}

func TestCompileRowColumnAcceptsTableChildren(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      row @summary:\n        table @facts:\n          width: 70%\n          table-column:\n            width: 50%\n          table-column:\n            width: 50%\n          table-row:\n            cell:\n              text: \"Name\"\n            cell:\n              text: \"Value\"\n        paragraph @aside:\n          width: 30%\n          text: \"Aside\"\n"
	parsed := paperlang.Parse("row-table.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	row := compiled.Document.Body[0].(layout.RowColumnBlock)
	table, ok := row.Items[0].Block.(layout.TableBlock)
	if !ok || len(table.Columns) != 2 || table.Columns[0].WidthPercent != 50_000_000 ||
		len(table.Body) != 1 || len(table.Body[0].Cells) != 2 || row.Items[0].Track.BasisPercent != 70_000_000 {
		t.Fatalf("row table = %#v", row.Items[0])
	}
}

func TestCompileRowColumnExposesWrapAlignmentAndFlexConstraints(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      row:\n        line-gap: 4pt\n        height: 80pt\n        wrap: \"wrap-reverse\"\n        justify-content: \"space-between\"\n        align-items: \"center\"\n        align-content: \"stretch\"\n        reverse: true\n        paragraph:\n          width: 40pt\n          flex-grow: 1.5\n          flex-shrink: 0.5\n          height: 50%\n          min-height: 20%\n          max-height: 80%\n          text: \"Flexible\"\n"
	parsed := paperlang.Parse("flex-row.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	row := compiled.Document.Body[0].(layout.RowColumnBlock)
	track, item := row.Items[0].Track, row.Items[0]
	if row.CrossGap != 4 || row.CrossSize != 80 || row.Wrap != "wrap-reverse" || row.MainAlign != "space-between" ||
		row.CrossAlign != "center" || row.AlignContent != "stretch" || !row.ReverseMain ||
		track.Kind != layout.RowColumnTrackFlex || track.BasisKind != layout.RowColumnFlexBasisFixed || track.Basis != 40 ||
		track.GrowFactor != 1_500_000 || track.ShrinkFactor != 500_000 || item.CrossMinPercent != 20_000_000 ||
		item.CrossMaxPercent != 80_000_000 {
		t.Fatalf("row/item = %#v / %#v", row, item)
	}
}

func TestCompileRowColumnAcceptsOneReadableNestedLevel(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      row @outer:\n        column @details:\n          width: 70%\n          gap: 2pt\n          paragraph @first:\n            height: 14pt\n            text: \"First\"\n          paragraph @second:\n            height: 14pt\n            text: \"Second\"\n        paragraph @aside:\n          width: 30%\n          text: \"Aside\"\n"
	parsed := paperlang.Parse("nested-layout.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	outer := compiled.Document.Body[0].(layout.RowColumnBlock)
	nested, ok := outer.Items[0].Block.(layout.RowColumnBlock)
	if !ok || nested.Direction != layout.ColumnDirection || nested.Gap != 2 || len(nested.Items) != 2 ||
		outer.Items[0].Track.BasisPercent != 70_000_000 || nested.Items[0].Track.Size != 14 {
		t.Fatalf("nested layout = %#v", outer.Items[0])
	}
	mappings := map[string]NodeMapping{}
	for _, mapping := range compiled.Mapping.Nodes {
		mappings[mapping.ID] = mapping
	}
	if mappings["@details"].SegmentIndex != 0 || mappings["@first"].SegmentIndex != 0 || mappings["@first"].NestedBlockIndex != 0 ||
		mappings["@second"].NestedBlockIndex != 1 {
		t.Fatalf("nested mappings = %+v", mappings)
	}
}

func TestCompileRowColumnUsesDirectionalAuthoringNames(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      row @outer:\n        line-gap: 4pt\n        height: 80pt\n        wrap: \"wrap-reverse\"\n        justify-content: \"space-between\"\n        align-items: \"center\"\n        align-content: \"stretch\"\n        reverse: true\n        column @details:\n          width: 70%\n          gap: 2pt\n          paragraph:\n            height: \"auto\"\n            min-height: 12pt\n            flex-grow: 1.5\n            width: 50%\n            align-self: \"end\"\n            text: \"First\"\n          paragraph:\n            height: 1fr\n            text: \"Second\"\n        table:\n          width: 30%\n          table-column:\n            width: 100%\n          table-row:\n            cell:\n              text: \"Aside\"\n"
	parsed := paperlang.Parse("directional-layout.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("diagnostics = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	outer := compiled.Document.Body[0].(layout.RowColumnBlock)
	nested := outer.Items[0].Block.(layout.RowColumnBlock)
	first := nested.Items[0]
	if outer.CrossGap != 4 || outer.CrossSize != 80 || outer.MainAlign != "space-between" || outer.CrossAlign != "center" || !outer.ReverseMain ||
		outer.Items[0].Track.BasisPercent != 70_000_000 || outer.Items[1].Track.BasisPercent != 30_000_000 ||
		first.Track.Kind != layout.RowColumnTrackFlex || first.Track.BasisKind != layout.RowColumnFlexBasisContent || first.Track.Min != 12 || first.Track.GrowFactor != 1_500_000 ||
		nested.Items[1].Track.Kind != layout.RowColumnTrackFraction || nested.Items[1].Track.Weight != 1 ||
		first.CrossMinPercent != 50_000_000 || first.CrossMaxPercent != 50_000_000 || first.CrossAlign != "end" {
		t.Fatalf("directional layout = %#v / %#v", outer, first)
	}
}

func TestCompileRowColumnKeepsFractionSharesWhenConstraintsPromoteSizing(t *testing.T) {
	const source = "document:\n  page:\n    body:\n      row:\n        paragraph:\n          width: 2fr\n          max-width: 30%\n          text: \"Capped\"\n        paragraph:\n          width: 1fr\n          min-width: 20%\n          text: \"Floored\"\n"
	parsed := paperlang.Parse("constrained-fractions.paper", source)
	compiled := Compile(parsed.AST)
	if !parsed.OK() || !compiled.OK() {
		t.Fatalf("parse/compile = %+v / %+v", parsed.Diagnostics, compiled.Diagnostics)
	}
	items := compiled.Document.Body[0].(layout.RowColumnBlock).Items
	if items[0].Track.Kind != layout.RowColumnTrackFlex || items[0].Track.Grow != 2 || items[0].Track.MaxPercent != 30_000_000 ||
		items[1].Track.Kind != layout.RowColumnTrackFlex || items[1].Track.Grow != 1 || items[1].Track.MinPercent != 20_000_000 {
		t.Fatalf("constrained fraction shares = %#v", items)
	}
}
