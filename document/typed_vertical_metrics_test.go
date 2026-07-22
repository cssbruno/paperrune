// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"testing"
)

func TestTypedMetricBaselineCentersFontBoxInLineBox(t *testing.T) {
	baseline, ok := typedMetricBaseline(12, 6.29, 1.57)
	if !ok || !almostEqual(baseline, 8.36) {
		t.Fatalf("Courier baseline = %.4f, %t", baseline, ok)
	}
	baseline, ok = typedMetricBaseline(15, 7.18, 2.07)
	if !ok || !almostEqual(baseline, 10.055) {
		t.Fatalf("Helvetica baseline = %.4f, %t", baseline, ok)
	}
}

func TestTypedFontVerticalExtentsUseCoreAndEmbeddedMetrics(t *testing.T) {
	core, err := loadCoreFontDef("helvetica")
	if err != nil {
		t.Fatal(err)
	}
	ascent, descent, ok := typedFontVerticalExtents(core, 10)
	if !ok || !almostEqual(ascent, 7.18) || !almostEqual(descent, 2.07) {
		t.Fatalf("core extents = %.3f/%.3f, %t", ascent, descent, ok)
	}
	embedded := fontDefinition{Desc: FontDescriptor{Ascent: 750, Descent: -250}}
	ascent, descent, ok = typedFontVerticalExtents(embedded, 12)
	if !ok || !almostEqual(ascent, 9) || !almostEqual(descent, 3) {
		t.Fatalf("embedded extents = %.3f/%.3f, %t", ascent, descent, ok)
	}
}
