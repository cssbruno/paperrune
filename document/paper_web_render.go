// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"context"
	"errors"
	"fmt"

	"github.com/cssbruno/paperrune/internal/layoutengine"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
)

// PaperPlanWebTextRun is one already-positioned text run for an HTML painter.
// Geometry is fixed-point page space; the browser may paint and select the
// glyphs but must not reflow or repaginate them.
type PaperPlanWebTextRun struct {
	Text       string `json:"text"`
	FontFamily string `json:"font_family"`
	FontWeight string `json:"font_weight"`
	FontStyle  string `json:"font_style"`
	Color      string `json:"color"`
	X          int64  `json:"x_fixed"`
	Baseline   int64  `json:"baseline_fixed"`
	Width      int64  `json:"width_fixed"`
	FontSize   int64  `json:"font_size_fixed"`
	Opacity    int64  `json:"opacity_fixed"`
}

// PaperPlanWebFont carries the deterministic font program used by the WASM
// raster painter so an HTML text painter can use the same outlines.
type PaperPlanWebFont struct {
	Family string `json:"family"`
	Data   []byte `json:"data"`
}

// PaperPlanWebGraphicsPage is the non-text display list for a direct canvas
// painter. Payload indexes remain plan-local and all geometry is fixed-point.
type PaperPlanWebGraphicsPage struct {
	Width      int64                         `json:"width_fixed"`
	Height     int64                         `json:"height_fixed"`
	FixedScale int64                         `json:"fixed_scale"`
	Commands   []layoutengine.DisplayCommand `json:"commands"`
	Paths      []layoutengine.PlannedPath    `json:"paths"`
	Transforms []layoutengine.Transform      `json:"transforms"`
	Clips      []layoutengine.PlannedClip    `json:"clips"`
	Fills      []layoutengine.PlannedFill    `json:"fills"`
	Strokes    []layoutengine.PlannedStroke  `json:"strokes"`
}

// WebDisplayGraphicsPage returns only non-text paint commands for one page.
// Text is intentionally excluded so browsers render it as selectable HTML.
func (p PaperPlan) WebDisplayGraphicsPage(page uint32) (PaperPlanWebGraphicsPage, error) {
	projection := p.plan.Projection()
	if page == 0 || int(page) > len(projection.Pages) {
		return PaperPlanWebGraphicsPage{}, errors.New("document: invalid paper plan web graphics page")
	}
	plannedPage := projection.Pages[page-1]
	end := uint64(plannedPage.Commands.Start) + uint64(plannedPage.Commands.Count)
	if end > uint64(len(projection.Commands)) {
		return PaperPlanWebGraphicsPage{}, errors.New("document: invalid paper plan web graphics command range")
	}
	commands := make([]layoutengine.DisplayCommand, 0, plannedPage.Commands.Count)
	for _, command := range projection.Commands[plannedPage.Commands.Start:end] {
		if command.Kind != layoutengine.CommandGlyphRun && command.Kind != layoutengine.CommandImage && command.Kind != layoutengine.CommandLink {
			commands = append(commands, command)
		}
	}
	return PaperPlanWebGraphicsPage{
		Width: int64(plannedPage.Size.Width), Height: int64(plannedPage.Size.Height), FixedScale: layoutengine.FixedScale,
		Commands: commands, Paths: projection.Paths, Transforms: projection.Transforms,
		Clips: projection.Clips, Fills: projection.Fills, Strokes: projection.Strokes,
	}, nil
}

// PaperPlanWebTextPage returns positioned text and exact painter font programs
// for one retained page. It performs no browser measurement or layout.
func (p PaperPlan) WebDisplayTextPage(page uint32) ([]PaperPlanWebTextRun, []PaperPlanWebFont, error) {
	projection := p.plan.Projection()
	if page == 0 || int(page) > len(projection.Pages) {
		return nil, nil, errors.New("document: invalid paper plan web text page")
	}
	plannedPage := projection.Pages[page-1]
	lineEnd := uint64(plannedPage.Lines.Start) + uint64(plannedPage.Lines.Count)
	runs := make([]PaperPlanWebTextRun, 0, plannedPage.Lines.Count)
	usedFonts := make(map[layoutengine.FontResourceID]struct{})
	for _, run := range projection.GlyphRuns {
		if run.Line < plannedPage.Lines.Start || uint64(run.Line) >= lineEnd {
			continue
		}
		width := layoutengine.Fixed(0)
		for _, advance := range run.Advances {
			var err error
			width, err = width.Add(advance)
			if err != nil {
				return nil, nil, fmt.Errorf("document: web text run width: %w", err)
			}
		}
		resource := projection.Fonts[run.Font-1]
		opacity := run.Opacity
		if opacity == 0 {
			opacity = layoutengine.Fixed(layoutengine.FixedScale)
		}
		family, weight, style := paperWebFontAppearance(resource)
		runs = append(runs, PaperPlanWebTextRun{
			Text: run.Codes, FontFamily: family, FontWeight: weight, FontStyle: style, Color: paperWebTextColor(run.Color),
			X: int64(run.Origin.X), Baseline: int64(run.Origin.Y), Width: int64(width),
			FontSize: int64(run.FontSize), Opacity: int64(opacity),
		})
		if resource.EmbeddedUTF8 != nil {
			usedFonts[run.Font] = struct{}{}
		}
	}
	fonts := make([]PaperPlanWebFont, 0, len(usedFonts))
	for _, resource := range projection.Fonts {
		if _, used := usedFonts[resource.ID]; !used {
			continue
		}
		program := paperWebCoreFontProgram(resource.Face)
		if resource.EmbeddedUTF8 != nil {
			program = p.fontSources[resource.EmbeddedUTF8.Digest]
		}
		if len(program) == 0 {
			return nil, nil, fmt.Errorf("document: web text font %s is unavailable", paperWebFontFamily(resource))
		}
		fonts = append(fonts, PaperPlanWebFont{Family: paperWebFontFamily(resource), Data: append([]byte(nil), program...)})
	}
	return runs, fonts, nil
}

func paperWebFontAppearance(resource layoutengine.CoreFontResource) (family, weight, style string) {
	if resource.EmbeddedUTF8 != nil {
		return fmt.Sprintf("PaperRune-%x", resource.MetricsDigest[:8]), "400", "normal"
	}
	weight, style = "400", "normal"
	switch resource.Face {
	case layoutengine.CoreFontCourierBold, layoutengine.CoreFontCourierBoldOblique,
		layoutengine.CoreFontHelveticaBold, layoutengine.CoreFontHelveticaBoldOblique,
		layoutengine.CoreFontTimesBold, layoutengine.CoreFontTimesBoldItalic:
		weight = "700"
	}
	switch resource.Face {
	case layoutengine.CoreFontCourierOblique, layoutengine.CoreFontCourierBoldOblique,
		layoutengine.CoreFontHelveticaOblique, layoutengine.CoreFontHelveticaBoldOblique,
		layoutengine.CoreFontTimesItalic, layoutengine.CoreFontTimesBoldItalic:
		style = "italic"
	}
	switch resource.Face {
	case layoutengine.CoreFontCourier, layoutengine.CoreFontCourierBold,
		layoutengine.CoreFontCourierOblique, layoutengine.CoreFontCourierBoldOblique:
		family = `"Courier New", Courier, monospace`
	case layoutengine.CoreFontTimesRoman, layoutengine.CoreFontTimesBold,
		layoutengine.CoreFontTimesItalic, layoutengine.CoreFontTimesBoldItalic:
		family = `"Times New Roman", Times, serif`
	case layoutengine.CoreFontSymbol:
		family = `Symbol, serif`
	case layoutengine.CoreFontZapfDingbats:
		family = `"Zapf Dingbats", serif`
	default:
		family = `Helvetica, Arial, sans-serif`
	}
	return family, weight, style
}

func paperWebFontFamily(resource layoutengine.CoreFontResource) string {
	family, _, _ := paperWebFontAppearance(resource)
	return family
}

func paperWebTextColor(color layoutengine.CoreRGBColor) string {
	if !color.Set {
		return "#000000"
	}
	return fmt.Sprintf("#%02x%02x%02x", color.R, color.G, color.B)
}

// PaperPlanWebRenderRequest selects one immutable page and a bounded raster
// density for a browser WASM renderer. No layout choices are accepted here.
type PaperPlanWebRenderRequest struct {
	Page uint32
	DPI  uint32
}

func DefaultPaperPlanWebRenderRequest(page uint32) PaperPlanWebRenderRequest {
	return PaperPlanWebRenderRequest{Page: page, DPI: layoutengine.DefaultDisplayRasterProfile().DPI}
}

// WebDisplayRenderPayload returns a detached, self-verifying payload consumed
// by the Go WASM renderer. It contains the canonical immutable plan and only
// content-addressed renderer resources; source text is never included.
func (p PaperPlan) WebDisplayRenderPayload(ctx context.Context, request PaperPlanWebRenderRequest) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("document: nil web render context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.hash == "" || p.pages <= 0 || request.Page == 0 || uint64(request.Page) > uint64(p.pages) {
		return nil, errors.New("document: invalid paper plan web render page")
	}
	profile := layoutengine.DefaultDisplayRasterProfile()
	profile.DPI = request.DPI
	limits := layoutengine.DefaultDisplayRasterLimits()
	pageProfile := ""
	if inputs, ok := p.plan.DeterministicInputs(); ok {
		pageProfile = inputs.PageProfile.ID
	}
	if pageProfile == "" {
		return nil, errors.New("document: web render plan has no deterministic page profile")
	}
	projection := p.plan.Projection()
	fonts := make(map[layoutengine.CoreFontMetricsDigest][]byte, len(projection.Fonts))
	for _, resource := range projection.Fonts {
		if resource.EmbeddedUTF8 == nil {
			fonts[resource.MetricsDigest] = paperWebCoreFontProgram(resource.Face)
			continue
		}
		program := p.fontSources[resource.EmbeddedUTF8.Digest]
		if len(program) == 0 {
			return nil, fmt.Errorf("document: web render embedded font %s is unavailable", resource.EmbeddedUTF8.Name)
		}
		fonts[resource.MetricsDigest] = append([]byte(nil), program...)
	}
	images := make(layoutengine.DisplaySVGImageSources, len(p.imageSources))
	for digest, source := range p.imageSources {
		images[digest] = append([]byte(nil), source...)
	}
	return layoutengine.EncodeWebDisplayRenderPayload(p.plan, layoutengine.DisplayRasterSources{FontPrograms: fonts, Images: images},
		layoutengine.DisplayRasterRequest{Page: request.Page, Profile: profile, Limits: limits, Revisions: p.revisions, PageProfile: pageProfile})
}

// Standard-14 fonts do not carry programs in a PDF plan. The browser preview
// uses deterministic Go outlines with matching weight/slant and, critically,
// a monospace family for Courier while retaining the exact planned advances.
func paperWebCoreFontProgram(face layoutengine.CoreFontFace) []byte {
	switch face {
	case layoutengine.CoreFontCourier:
		return gomono.TTF
	case layoutengine.CoreFontCourierBold:
		return gomonobold.TTF
	case layoutengine.CoreFontCourierOblique:
		return gomonoitalic.TTF
	case layoutengine.CoreFontCourierBoldOblique:
		return gomonobolditalic.TTF
	case layoutengine.CoreFontHelveticaBold, layoutengine.CoreFontTimesBold:
		return gobold.TTF
	case layoutengine.CoreFontHelveticaOblique, layoutengine.CoreFontTimesItalic:
		return goitalic.TTF
	case layoutengine.CoreFontHelveticaBoldOblique, layoutengine.CoreFontTimesBoldItalic:
		return gobolditalic.TTF
	default:
		return goregular.TTF
	}
}
