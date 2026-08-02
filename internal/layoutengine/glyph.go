// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package layoutengine

import (
	"encoding/hex"
	"errors"
	"fmt"
	"unicode/utf8"
)

// FontResourceID is a one-based index into a plan's canonical font table.
// Zero means no font resource.
type FontResourceID uint32

func (id FontResourceID) Valid() bool { return id != 0 }

// CoreFontFace is one of the PDF standard 14 font faces supported by the
// initial no-layout painter contract.
type CoreFontFace string

// FontVerticalMetrics describes the canonical vertical extents of a font in
// thousandths of an em. Descent is negative, matching PDF font descriptors.
// These metrics let planners and non-PDF painters agree on the same baseline
// even when a Standard-14 font needs a substitute outline for preview.
type FontVerticalMetrics struct {
	Ascent  int16
	Descent int16
}

const (
	CoreFontCourier              CoreFontFace = "courier"
	CoreFontCourierBold          CoreFontFace = "courier_bold"
	CoreFontCourierOblique       CoreFontFace = "courier_oblique"
	CoreFontCourierBoldOblique   CoreFontFace = "courier_bold_oblique"
	CoreFontHelvetica            CoreFontFace = "helvetica"
	CoreFontHelveticaBold        CoreFontFace = "helvetica_bold"
	CoreFontHelveticaOblique     CoreFontFace = "helvetica_oblique"
	CoreFontHelveticaBoldOblique CoreFontFace = "helvetica_bold_oblique"
	CoreFontTimesRoman           CoreFontFace = "times_roman"
	CoreFontTimesBold            CoreFontFace = "times_bold"
	CoreFontTimesItalic          CoreFontFace = "times_italic"
	CoreFontTimesBoldItalic      CoreFontFace = "times_bold_italic"
	CoreFontSymbol               CoreFontFace = "symbol"
	CoreFontZapfDingbats         CoreFontFace = "zapf_dingbats"
)

func (face CoreFontFace) valid() bool {
	switch face {
	case CoreFontCourier, CoreFontCourierBold, CoreFontCourierOblique,
		CoreFontCourierBoldOblique, CoreFontHelvetica, CoreFontHelveticaBold,
		CoreFontHelveticaOblique, CoreFontHelveticaBoldOblique,
		CoreFontTimesRoman, CoreFontTimesBold, CoreFontTimesItalic,
		CoreFontTimesBoldItalic, CoreFontSymbol, CoreFontZapfDingbats:
		return true
	default:
		return false
	}
}

// VerticalMetrics returns the canonical Standard-14 ascent and descent for
// face. Symbol faces do not publish ascender fields in their AFM data, so
// their font bounding-box extents are used instead.
func (face CoreFontFace) VerticalMetrics() (FontVerticalMetrics, bool) {
	switch face {
	case CoreFontCourier, CoreFontCourierBold, CoreFontCourierOblique, CoreFontCourierBoldOblique:
		return FontVerticalMetrics{Ascent: 629, Descent: -157}, true
	case CoreFontHelvetica, CoreFontHelveticaBold, CoreFontHelveticaOblique, CoreFontHelveticaBoldOblique:
		return FontVerticalMetrics{Ascent: 718, Descent: -207}, true
	case CoreFontTimesRoman:
		return FontVerticalMetrics{Ascent: 683, Descent: -217}, true
	case CoreFontTimesBold:
		return FontVerticalMetrics{Ascent: 676, Descent: -205}, true
	case CoreFontTimesItalic:
		return FontVerticalMetrics{Ascent: 683, Descent: -205}, true
	case CoreFontTimesBoldItalic:
		return FontVerticalMetrics{Ascent: 699, Descent: -205}, true
	case CoreFontSymbol:
		return FontVerticalMetrics{Ascent: 1010, Descent: -293}, true
	case CoreFontZapfDingbats:
		return FontVerticalMetrics{Ascent: 820, Descent: -143}, true
	default:
		return FontVerticalMetrics{}, false
	}
}

// VerticalExtents scales the face's canonical ascent and positive descent to
// a fixed-point font size. Keeping this conversion beside VerticalMetrics
// prevents plan builders and painters from rounding the same font box in
// different ways.
func (face CoreFontFace) VerticalExtents(fontSize Fixed) (ascent, descent Fixed, ok bool) {
	metrics, ok := face.VerticalMetrics()
	if !ok || fontSize <= 0 {
		return 0, 0, false
	}
	ascent, err := fontSize.MulInt(int64(metrics.Ascent))
	if err == nil {
		ascent, err = ascent.DivInt(1000)
	}
	if err != nil || ascent <= 0 {
		return 0, 0, false
	}
	descent, err = fontSize.MulInt(-int64(metrics.Descent))
	if err == nil {
		descent, err = descent.DivInt(1000)
	}
	if err != nil || descent < 0 {
		return 0, 0, false
	}
	return ascent, descent, true
}

// CoreFontMetricsDigest is a lowercase SHA-256 digest of the exact metrics
// used during planning. It makes a font name insufficient to silently change
// layout or paint geometry.
type CoreFontMetricsDigest string

func (digest CoreFontMetricsDigest) validate() error {
	if len(digest) != 64 {
		return errors.New("metrics digest is not a lowercase SHA-256 digest")
	}
	decoded, err := hex.DecodeString(string(digest))
	if err != nil || hex.EncodeToString(decoded) != string(digest) {
		return errors.New("metrics digest is not a lowercase SHA-256 digest")
	}
	allZero := true
	for _, value := range decoded {
		allZero = allZero && value == 0
	}
	if allZero {
		return errors.New("metrics digest is zero")
	}
	return nil
}

// CoreFontResource identifies immutable core-font metrics. Font size belongs
// to each glyph run, allowing the resource to be reused across sizes.
type CoreFontResource struct {
	ID            FontResourceID        `json:"id"`
	Face          CoreFontFace          `json:"face,omitempty"`
	MetricsDigest CoreFontMetricsDigest `json:"metrics_digest"`
	EmbeddedUTF8  *EmbeddedUTF8Font     `json:"embedded_utf8,omitempty"`
}

// EmbeddedUTF8Font identifies a detached TrueType program used by an exact
// plan. The program bytes are deliberately not serialized into LayoutPlan;
// immutable callers retain them in a content-addressed sidecar and painters
// must verify Digest and ByteLength before installing the resource.
type EmbeddedUTF8Font struct {
	Name       string                `json:"name"`
	Digest     CoreFontMetricsDigest `json:"digest"`
	ByteLength uint32                `json:"byte_length"`
}

func (resource CoreFontResource) IsEmbeddedUTF8() bool { return resource.EmbeddedUTF8 != nil }

func (resource CoreFontResource) GlyphCount(codes string) int {
	if resource.IsEmbeddedUTF8() {
		return utf8.RuneCountInString(codes)
	}
	return len(codes)
}

func cloneFontResources(resources []CoreFontResource) []CoreFontResource {
	if len(resources) == 0 {
		return nil
	}
	cloned := append([]CoreFontResource(nil), resources...)
	for index := range cloned {
		if resources[index].EmbeddedUTF8 != nil {
			descriptor := *resources[index].EmbeddedUTF8
			cloned[index].EmbeddedUTF8 = &descriptor
		}
	}
	return cloned
}

// CoreRGBColor is an optional exact sRGB text color carried from layout
// through every plan consumer. An unset color is canonical only when RGB are
// all zero and means the painter's default black.
type CoreRGBColor struct {
	R   uint8 `json:"r"`
	G   uint8 `json:"g"`
	B   uint8 `json:"b"`
	Set bool  `json:"set"`
}

// CoreGlyphRun is already shaped and positioned. For a core-font resource,
// Codes contains printable PDF core-font bytes and Advances has one entry per
// byte. For an EmbeddedUTF8 resource, Codes is canonical BMP UTF-8 and
// Advances has one entry per Unicode scalar. In both cases advances sum
// exactly to the owning line width; the painter never shapes or measures it.
type CoreGlyphRun struct {
	Line     uint32         `json:"line"`
	Font     FontResourceID `json:"font"`
	FontSize Fixed          `json:"font_size"`
	Color    CoreRGBColor   `json:"color"`
	Opacity  Fixed          `json:"opacity,omitempty"`
	Origin   Point          `json:"origin"`
	Codes    string         `json:"codes"`
	// LeadingSpace records authored whitespace consumed by line wrapping. It is
	// extraction metadata only and must never change painted glyph geometry.
	LeadingSpace bool `json:"leading_space,omitempty"`
	// TrailingSpace records authored whitespace after the visible codes. The
	// painter carries it to the next run's extraction text without painting it.
	TrailingSpace bool       `json:"trailing_space,omitempty"`
	Advances      []Fixed    `json:"advances"`
	Source        SourceSpan `json:"source"`
}

func cloneCoreGlyphRuns(runs []CoreGlyphRun) []CoreGlyphRun {
	if len(runs) == 0 {
		return nil
	}
	cloned := append([]CoreGlyphRun(nil), runs...)
	for index := range cloned {
		cloned[index].Advances = cloneSlice(runs[index].Advances)
	}
	return cloned
}

func validateCoreFonts(fonts []CoreFontResource) error {
	type fontIdentity struct {
		face     CoreFontFace
		embedded CoreFontMetricsDigest
	}
	var seen map[fontIdentity]struct{}
	var seenEmbeddedNames map[string]struct{}
	if len(fonts) > 1 {
		seen = make(map[fontIdentity]struct{}, len(fonts))
		seenEmbeddedNames = make(map[string]struct{}, len(fonts))
	}
	for index, font := range fonts {
		if font.ID != FontResourceID(index+1) {
			return planIndexedError("fonts", index, "", "font IDs are not consecutive and one-based")
		}
		if err := font.MetricsDigest.validate(); err != nil {
			return planIndexedError("fonts", index, ".metrics_digest", err.Error())
		}
		identity := fontIdentity{face: font.Face}
		if font.EmbeddedUTF8 == nil {
			if !font.Face.valid() {
				return planIndexedError("fonts", index, ".face", "is not a canonical core font face")
			}
		} else {
			if font.Face != "" {
				return planIndexedError("fonts", index, ".face", "must be empty for an embedded UTF-8 font")
			}
			if !validEmbeddedFontName(font.EmbeddedUTF8.Name) {
				return planIndexedError("fonts", index, ".embedded_utf8.name", "must be a bounded ASCII resource name")
			}
			if err := font.EmbeddedUTF8.Digest.validate(); err != nil {
				return planIndexedError("fonts", index, ".embedded_utf8.digest", err.Error())
			}
			if font.EmbeddedUTF8.ByteLength == 0 {
				return planIndexedError("fonts", index, ".embedded_utf8.byte_length", "must be positive")
			}
			if _, duplicate := seenEmbeddedNames[font.EmbeddedUTF8.Name]; duplicate {
				return planIndexedError("fonts", index, ".embedded_utf8.name", "duplicates an embedded font resource name")
			}
			if seenEmbeddedNames != nil {
				seenEmbeddedNames[font.EmbeddedUTF8.Name] = struct{}{}
			}
			identity = fontIdentity{embedded: font.EmbeddedUTF8.Digest}
		}
		if _, duplicate := seen[identity]; duplicate {
			return planIndexedError("fonts", index, "", "duplicates a font resource")
		}
		if seen != nil {
			seen[identity] = struct{}{}
		}
	}
	return nil
}

func validEmbeddedFontName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, character := range name {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || character == '_' || character == '-' || character == '.' {
			continue
		}
		return false
	}
	return true
}

// AttachCoreGlyphRuns lowers a geometry-only plan into a command-backed core
// font plan without invoking layout. Runs must be ordered by their global line
// index. Empty planned lines are represented by the absence of a run.
func AttachCoreGlyphRuns(plan LayoutPlan, fonts []CoreFontResource, runs []CoreGlyphRun) (LayoutPlan, error) {
	return attachCoreGlyphRuns(plan, fonts, runs, true, false)
}

// AttachOwnedCoreGlyphRuns is AttachCoreGlyphRuns with explicit ownership
// transfer. The caller must not mutate fonts, runs, or nested run advances
// after the call.
func AttachOwnedCoreGlyphRuns(plan LayoutPlan, fonts []CoreFontResource, runs []CoreGlyphRun) (LayoutPlan, error) {
	return attachCoreGlyphRuns(plan, fonts, runs, true, true)
}

// AttachOwnedTrustedCoreGlyphRuns composes planner-owned font and glyph
// payloads without revalidating the unchanged geometry graph. The caller must
// have produced valid canonical resources, runs, and advances and must not
// mutate them after this ownership transfer.
func AttachOwnedTrustedCoreGlyphRuns(plan LayoutPlan, fonts []CoreFontResource, runs []CoreGlyphRun) (LayoutPlan, error) {
	return attachCoreGlyphRuns(plan, fonts, runs, false, true)
}

func attachCoreGlyphRuns(plan LayoutPlan, fonts []CoreFontResource, runs []CoreGlyphRun, validateResult, takeOwnership bool) (LayoutPlan, error) {
	if len(plan.pages) == 0 {
		return LayoutPlan{}, errors.New("layoutengine: core glyph runs require a non-empty plan")
	}
	if len(plan.fonts) != 0 || len(plan.glyphRuns) != 0 || len(plan.commands) != 0 {
		return LayoutPlan{}, errors.New("layoutengine: core glyph runs require a geometry-only plan")
	}
	if uint64(len(runs)) > uint64(^uint32(0)) {
		return LayoutPlan{}, errors.New("layoutengine: core glyph run count exceeds plan index capacity")
	}
	for index := 1; index < len(runs); index++ {
		if runs[index].Line < runs[index-1].Line {
			return LayoutPlan{}, errors.New("layoutengine: core glyph runs are not in line order")
		}
	}

	fragmentPages := make(map[FragmentID]uint32, len(plan.fragments))
	for _, fragment := range plan.fragments {
		fragmentPages[fragment.ID] = fragment.Page
	}
	commands := make([]DisplayCommand, 0, len(runs))
	pageCounts := make([]uint32, len(plan.pages))
	for index, run := range runs {
		if uint64(run.Line) >= uint64(len(plan.lines)) {
			return LayoutPlan{}, planError(fmt.Sprintf("glyph_runs[%d].line", index), "references a missing planned line")
		}
		line := plan.lines[run.Line]
		page := fragmentPages[line.Fragment]
		if page == 0 || uint64(page) > uint64(len(pageCounts)) {
			return LayoutPlan{}, planError(fmt.Sprintf("glyph_runs[%d]", index), "line has no owning page")
		}
		pageCounts[page-1]++
		commands = append(commands, DisplayCommand{
			Kind: CommandGlyphRun, Fragment: line.Fragment, Bounds: line.Bounds, Payload: uint32(index),
		})
	}
	pages := cloneSlice(plan.pages)
	var commandStart uint32
	for index := range pages {
		pages[index].Commands = IndexRange{Start: commandStart, Count: pageCounts[index]}
		commandStart += pageCounts[index]
	}
	result := plan
	result.pages = pages
	if takeOwnership {
		result.fonts = fonts
		result.glyphRuns = runs
	} else {
		result.fonts = cloneFontResources(fonts)
		result.glyphRuns = cloneCoreGlyphRuns(runs)
	}
	result.commands = commands
	if plan.hasDeterministicInputs {
		result.deterministicInputs = DeterministicInputManifest{}
		result.hasDeterministicInputs = false
	}
	if validateResult {
		if err := result.Validate(); err != nil {
			return LayoutPlan{}, err
		}
	}
	if !plan.hasDeterministicInputs {
		return result, nil
	}
	return rebindDeterministicResources(result, &plan.deterministicInputs)
}
