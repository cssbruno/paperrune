// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/layoutengine"
)

func TestPaperPlanWebViewsUseOnlyFinalPlanGeometry(t *testing.T) {
	plan, result, err := PlanPaper("web-render.paper", paperPipelineFixture)
	if err != nil || !result.OK() {
		t.Fatalf("PlanPaper() = %#v, %v", result, err)
	}

	graphics, err := plan.WebDisplayGraphicsPage(1)
	if err != nil || graphics.Width <= 0 || graphics.Height <= 0 || graphics.FixedScale != layoutengine.FixedScale {
		t.Fatalf("graphics page = %#v, %v", graphics, err)
	}
	for _, command := range graphics.Commands {
		if command.Kind == layoutengine.CommandGlyphRun || command.Kind == layoutengine.CommandImage || command.Kind == layoutengine.CommandLink {
			t.Fatalf("graphics payload retained non-graphics command: %+v", command)
		}
	}
	if _, err := plan.WebDisplayGraphicsPage(0); err == nil {
		t.Fatal("graphics page zero was accepted")
	}
	if _, err := plan.WebDisplayGraphicsPage(uint32(plan.PageCount() + 1)); err == nil {
		t.Fatal("graphics page beyond the plan was accepted")
	}

	runs, fonts, err := plan.WebDisplayTextPage(1)
	if err != nil || len(runs) == 0 || len(fonts) != 0 {
		t.Fatalf("text page = %d runs, %d fonts, %v", len(runs), len(fonts), err)
	}
	if runs[0].Text == "" || runs[0].FontSize <= 0 || runs[0].Width <= 0 || runs[0].Opacity != layoutengine.FixedScale {
		t.Fatalf("positioned text run = %+v", runs[0])
	}
	if _, _, err := plan.WebDisplayTextPage(0); err == nil {
		t.Fatal("text page zero was accepted")
	}

	request := DefaultPaperPlanWebRenderRequest(1)
	payload, err := plan.WebDisplayRenderPayload(t.Context(), request)
	if err != nil || len(payload) == 0 {
		t.Fatalf("web render payload = %d bytes, %v", len(payload), err)
	}
	var nilContext context.Context
	if _, err := plan.WebDisplayRenderPayload(nilContext, request); err == nil {
		t.Fatal("nil render context was accepted")
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := plan.WebDisplayRenderPayload(canceled, request); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled render = %v", err)
	}
	if _, err := (PaperPlan{}).WebDisplayRenderPayload(t.Context(), request); err == nil {
		t.Fatal("empty plan render was accepted")
	}
	request.Page = uint32(plan.PageCount() + 1)
	if _, err := plan.WebDisplayRenderPayload(t.Context(), request); err == nil {
		t.Fatal("render page beyond the plan was accepted")
	}
}

func TestPaperPlanReviewUsesImmutablePlanOutputs(t *testing.T) {
	base, result, err := PlanPaper("review.paper", paperPipelineFixture)
	if err != nil || !result.OK() {
		t.Fatalf("base PlanPaper() = %#v, %v", result, err)
	}
	candidateSource := strings.Replace(paperPipelineFixture, `"A\nB`, `"Z\nB`, 1)
	candidate, result, err := PlanPaper("review.paper", candidateSource)
	if err != nil || !result.OK() {
		t.Fatalf("candidate PlanPaper() = %#v, %v", result, err)
	}
	request := DefaultPaperReviewRequest()
	request.CoreFontProgram = paperWebCoreFontProgram(layoutengine.CoreFontCourier)
	request.SourceDiff = []byte("A -> Z")
	bundle, err := candidate.ReviewAgainst(t.Context(), base, request)
	if err != nil || bundle.BeforePlanHash != base.Hash() || bundle.AfterPlanHash != candidate.Hash() ||
		len(bundle.ManifestJSON) == 0 || len(bundle.Artifacts) == 0 {
		t.Fatalf("review = before %q after %q manifest %d artifacts %d, %v", bundle.BeforePlanHash,
			bundle.AfterPlanHash, len(bundle.ManifestJSON), len(bundle.Artifacts), err)
	}
	if !bytes.Contains(bundle.ManifestJSON, []byte(base.Hash())) || !bytes.Contains(bundle.ManifestJSON, []byte(candidate.Hash())) {
		t.Fatalf("review manifest is not plan-bound: %s", bundle.ManifestJSON)
	}
	invalid := request
	invalid.MaxPages = 0
	if _, err := candidate.ReviewAgainst(t.Context(), base, invalid); err == nil {
		t.Fatal("unbounded review request was accepted")
	}
}

func TestPaperWebCoreFontMappingsAreComplete(t *testing.T) {
	cases := []struct {
		face   layoutengine.CoreFontFace
		family string
		weight string
		style  string
	}{
		{layoutengine.CoreFontCourier, `"Courier New", Courier, monospace`, "400", "normal"},
		{layoutengine.CoreFontCourierBold, `"Courier New", Courier, monospace`, "700", "normal"},
		{layoutengine.CoreFontCourierOblique, `"Courier New", Courier, monospace`, "400", "italic"},
		{layoutengine.CoreFontCourierBoldOblique, `"Courier New", Courier, monospace`, "700", "italic"},
		{layoutengine.CoreFontTimesRoman, `"Times New Roman", Times, serif`, "400", "normal"},
		{layoutengine.CoreFontTimesBold, `"Times New Roman", Times, serif`, "700", "normal"},
		{layoutengine.CoreFontTimesItalic, `"Times New Roman", Times, serif`, "400", "italic"},
		{layoutengine.CoreFontTimesBoldItalic, `"Times New Roman", Times, serif`, "700", "italic"},
		{layoutengine.CoreFontHelvetica, `Helvetica, Arial, sans-serif`, "400", "normal"},
		{layoutengine.CoreFontHelveticaBold, `Helvetica, Arial, sans-serif`, "700", "normal"},
		{layoutengine.CoreFontHelveticaOblique, `Helvetica, Arial, sans-serif`, "400", "italic"},
		{layoutengine.CoreFontHelveticaBoldOblique, `Helvetica, Arial, sans-serif`, "700", "italic"},
		{layoutengine.CoreFontSymbol, `Symbol, serif`, "400", "normal"},
		{layoutengine.CoreFontZapfDingbats, `"Zapf Dingbats", serif`, "400", "normal"},
	}
	for _, test := range cases {
		resource := layoutengine.CoreFontResource{Face: test.face}
		family, weight, style := paperWebFontAppearance(resource)
		if family != test.family || weight != test.weight || style != test.style || paperWebFontFamily(resource) != family {
			t.Errorf("face %q = %q/%q/%q", test.face, family, weight, style)
		}
		if len(paperWebCoreFontProgram(test.face)) == 0 {
			t.Errorf("face %q has no browser font program", test.face)
		}
	}
	if got := paperWebTextColor(layoutengine.CoreRGBColor{}); got != "#000000" {
		t.Fatalf("default color = %q", got)
	}
	if got := paperWebTextColor(layoutengine.CoreRGBColor{R: 1, G: 2, B: 255, Set: true}); got != "#0102ff" {
		t.Fatalf("explicit color = %q", got)
	}
}
