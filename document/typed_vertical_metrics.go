// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"math"
)

// typedFontVerticalExtents returns ascent and positive descent in document
// units for the exact font selected by the planner.
func typedFontVerticalExtents(font fontDefinition, fontSize float64) (float64, float64, bool) {
	if !isFiniteFloat(fontSize) || fontSize <= 0 {
		return 0, 0, false
	}
	ascent, descent := font.Desc.Ascent, font.Desc.Descent
	if ascent <= 0 || descent >= 0 {
		face, _, ok := typedCoreFontFace(font)
		if !ok {
			return 0, 0, false
		}
		metrics, ok := face.VerticalMetrics()
		if !ok {
			return 0, 0, false
		}
		ascent, descent = int(metrics.Ascent), int(metrics.Descent)
	}
	up := float64(ascent) * fontSize / 1000
	down := -float64(descent) * fontSize / 1000
	if !isFiniteFloat(up) || !isFiniteFloat(down) || up <= 0 || down < 0 {
		return 0, 0, false
	}
	return up, down, true
}

// typedMetricBaseline centers the font's ascent/descent box inside the
// declared line box. Half-leading may be negative when line-height is tighter
// than the font box, as in CSS and traditional typesetting.
func typedMetricBaseline(lineHeight, ascent, descent float64) (float64, bool) {
	if !isFiniteFloat(lineHeight) || !isFiniteFloat(ascent) || !isFiniteFloat(descent) ||
		lineHeight <= 0 || ascent <= 0 || descent < 0 {
		return 0, false
	}
	baseline := (lineHeight-ascent-descent)/2 + ascent
	return baseline, !math.IsNaN(baseline) && !math.IsInf(baseline, 0) && baseline >= 0 && baseline <= lineHeight
}

func typedFontMetricBaseline(lineHeight float64, font fontDefinition, fontSize float64) (float64, bool) {
	ascent, descent, ok := typedFontVerticalExtents(font, fontSize)
	if !ok {
		return 0, false
	}
	return typedMetricBaseline(lineHeight, ascent, descent)
}
