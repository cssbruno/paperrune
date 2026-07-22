// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"strconv"
	"testing"

	"github.com/cssbruno/paperrune/internal/layoutengine"
)

func TestAppendPDFTJAdjustmentMatchesFloatContract(t *testing.T) {
	fontSizes := []layoutengine.Fixed{1, 7, 511, 1024, 10 * 1024, 12 * 1024, 12345, 200 * 1024}
	for width := 0; width <= 1400; width += 17 {
		for _, fontSize := range fontSizes {
			nativeAdvance := layoutengine.Fixed(int64(width) * int64(fontSize) / 1000)
			for delta := layoutengine.Fixed(-16); delta <= 16; delta++ {
				advance := nativeAdvance + delta
				native := float64(width) * fontSize.Points() / 1000
				adjustment := (native - advance.Points()) * 1000 / fontSize.Points()
				want := strconv.AppendFloat(nil, adjustment, 'f', 10, 64)
				got := appendPDFTJAdjustment(nil, width, fontSize, advance)
				if string(got) != string(want) {
					t.Fatalf("width=%d fontSize=%d advance=%d: got %q want %q", width, fontSize, advance, got, want)
				}
			}
		}
	}
}
