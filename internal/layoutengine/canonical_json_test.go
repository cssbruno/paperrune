// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"testing"
)

func TestCanonicalJSONHashMatchesMarshal(t *testing.T) {
	integer := 7
	plan, err := NewLayoutPlan(coreGlyphPlanInput())
	if err != nil {
		t.Fatal(err)
	}
	tree, err := NewCanonicalTree(context.Background(), canonicalTreeFixture(), CanonicalTreeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	values := []any{
		struct {
			Text string `json:"text"`
			IDs  []int  `json:"ids,omitempty"`
		}{Text: "<&>    ", IDs: []int{1, 2, 3}},
		LayoutPlanProjection{SchemaVersion: LayoutPlanSchemaVersion, PlannerVersion: PlannerVersion, PainterContractVersion: PainterContractVersion},
		CanonicalTreeProjection{SchemaVersion: CanonicalTreeSchemaVersion, Strings: []string{"alpha", "beta"}},
		struct {
			Text    string         `json:"text"`
			Empty   string         `json:"empty,omitempty"`
			Bytes   []byte         `json:"bytes"`
			Nil     []int          `json:"nil"`
			Values  []int          `json:"values"`
			Array   [2]byte        `json:"array"`
			Flag    bool           `json:"flag"`
			Float   float64        `json:"float"`
			Pointer *int           `json:"pointer,omitempty"`
			Struct  SourcePosition `json:"struct,omitempty"`
		}{Text: string([]byte{'<', '>', '&', '\n', 0xff}), Bytes: []byte{0, 1, 2}, Values: []int{-1, 2}, Array: [2]byte{3, 4}, Float: 1e-9, Pointer: &integer},
		plan.ReadOnlyProjection(),
		tree.readOnlyProjection(),
	}
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := canonicalJSONHash(value)
		if err != nil {
			t.Fatal(err)
		}
		if want := sha256.Sum256(encoded); got != want {
			t.Fatalf("hash mismatch for %T: got %x want %x", value, got, want)
		}
	}
}

func TestCanonicalProjectionHashesMatchMarshal(t *testing.T) {
	manifest := DeterministicInputManifest{}
	var populatedPlan LayoutPlanProjection
	populateCanonicalProjectionTestValue(reflect.ValueOf(&populatedPlan).Elem(), 0)
	planProjections := []LayoutPlanProjection{
		{},
		populatedPlan,
		{
			SchemaVersion: LayoutPlanSchemaVersion, PlannerVersion: PlannerVersion, PainterContractVersion: PainterContractVersion,
			DeterministicInputs: &manifest,
			Pages:               []PlannedPage{{}}, Fragments: []Fragment{{}}, Lines: []PlannedLine{{}},
			PageRegions: []PlannedPageRegion{{}}, GridTracks: []PlannedGridTrack{{}}, Fonts: []CoreFontResource{{}},
			GlyphRuns: []CoreGlyphRun{{}}, ImageResources: []ImageResource{{}}, Images: []PlannedImage{{}},
			Destinations: []PlannedDestination{{}}, Links: []PlannedLink{{}}, Paths: []PlannedPath{{}},
			Transforms: []Transform{{}}, Clips: []PlannedClip{{}}, Fills: []PlannedFill{{}}, Strokes: []PlannedStroke{{}},
			Commands: []DisplayCommand{{}}, Breaks: []BreakDecision{{}}, Diagnostics: []Diagnostic{{}},
			SemanticNodes: []SemanticNode{{}}, SemanticFragments: []SemanticFragmentAssociation{{}},
			ReadingOrder: []ReadingOccurrence{{}}, Provenance: []ProvenanceEntry{{}},
			FragmentProvenance: []ProvenanceID{0}, LineProvenance: []ProvenanceID{0},
		},
	}
	for index, projection := range planProjections {
		encoded, err := json.Marshal(projection)
		if err != nil {
			t.Fatal(err)
		}
		got, err := layoutPlanProjectionHash(projection)
		if err != nil {
			t.Fatal(err)
		}
		if want := sha256.Sum256(encoded); got != want {
			t.Fatalf("plan projection[%d] hash mismatch: got %x want %x\nJSON: %s", index, got, want, encoded)
		}
	}

	var populatedTree CanonicalTreeProjection
	populateCanonicalProjectionTestValue(reflect.ValueOf(&populatedTree).Elem(), 0)
	treeProjections := []CanonicalTreeProjection{
		{},
		{Nodes: []TreeNode{}},
		populatedTree,
		{
			SchemaVersion: CanonicalTreeSchemaVersion,
			Nodes:         []TreeNode{{}}, Children: []TreeNodeIndex{0}, Strings: []string{"<&> \u2028 \u2029"},
			Styles: []TreeStyle{{}}, Tracks: []TreeTrack{{}}, Resources: []TreeResource{{}}, Semantics: []TreeSemantic{{}},
		},
	}
	for index, projection := range treeProjections {
		encoded, err := json.Marshal(projection)
		if err != nil {
			t.Fatal(err)
		}
		got, err := canonicalTreeProjectionHash(projection)
		if err != nil {
			t.Fatal(err)
		}
		if want := sha256.Sum256(encoded); got != want {
			t.Fatalf("tree projection[%d] hash mismatch: got %x want %x\nJSON: %s", index, got, want, encoded)
		}
	}
}

func populateCanonicalProjectionTestValue(value reflect.Value, depth int) {
	if depth > 16 {
		return
	}
	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(-int64(depth + 1))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value.SetUint(uint64(depth + 1))
	case reflect.Float32, reflect.Float64:
		value.SetFloat(1e-9)
	case reflect.String:
		value.SetString(string([]byte{'<', '>', '&', '\n', 0xff}) + " \u2028 \u2029")
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
		populateCanonicalProjectionTestValue(value.Elem(), depth+1)
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 2, 2))
		for index := 0; index < value.Len(); index++ {
			populateCanonicalProjectionTestValue(value.Index(index), depth+1)
		}
	case reflect.Array:
		for index := 0; index < value.Len(); index++ {
			populateCanonicalProjectionTestValue(value.Index(index), depth+1)
		}
	case reflect.Struct:
		for index := 0; index < value.NumField(); index++ {
			if value.Field(index).CanSet() {
				populateCanonicalProjectionTestValue(value.Field(index), depth+1)
			}
		}
	}
}
