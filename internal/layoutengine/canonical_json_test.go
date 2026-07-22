// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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
