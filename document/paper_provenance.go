// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"encoding/json"
	"fmt"

	"github.com/cssbruno/paperrune/internal/layout"
	"github.com/cssbruno/paperrune/internal/papercompile"
	"github.com/cssbruno/paperrune/internal/paperlang"
	"github.com/cssbruno/paperrune/internal/papertheme"
)

// PaperPlanSourceSpan identifies the authored source range that caused one
// retained binding or style decision. It is detached from the compiler's
// internal span type so the plan API does not expose parser contracts.
type PaperPlanSourceSpan struct {
	File        string `json:"file,omitempty"`
	StartOffset uint64 `json:"start_offset,omitempty"`
	EndOffset   uint64 `json:"end_offset,omitempty"`
	StartLine   uint32 `json:"start_line,omitempty"`
	StartColumn uint32 `json:"start_column,omitempty"`
	EndLine     uint32 `json:"end_line,omitempty"`
	EndColumn   uint32 `json:"end_column,omitempty"`
}

// PaperPlanColor is a detached optional RGB color in provenance output.
type PaperPlanColor struct {
	R   int
	G   int
	B   int
	Set bool
}

// PaperPlanTextStyle is the public provenance projection of an internal text style.
type PaperPlanTextStyle struct {
	FontFamily    string
	FontSize      float64
	Bold          bool
	Italic        bool
	Underline     bool
	StrikeThrough bool
	Color         PaperPlanColor
	Align         string
	LineHeight    float64
	WhiteSpace    string
	TabSize       uint8
}

// PaperPlanSpacing is detached block-edge spacing in provenance output.
type PaperPlanSpacing struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// PaperPlanBorderSide is one detached block border edge.
type PaperPlanBorderSide struct {
	Width float64
	Style string
	Color PaperPlanColor
}

// PaperPlanBorderStyle is the detached per-side block border projection.
type PaperPlanBorderStyle struct {
	Top    PaperPlanBorderSide
	Right  PaperPlanBorderSide
	Bottom PaperPlanBorderSide
	Left   PaperPlanBorderSide
}

// PaperPlanBoxShadowStyle is the detached supported shadow projection.
type PaperPlanBoxShadowStyle struct {
	OffsetX float64
	OffsetY float64
	Spread  float64
	Color   PaperPlanColor
}

// PaperPlanBoxStyle is the public provenance projection of an internal box style.
type PaperPlanBoxStyle struct {
	Margin          PaperPlanSpacing
	Padding         PaperPlanSpacing
	Border          PaperPlanBorderStyle
	BackgroundColor PaperPlanColor
	Width           float64
	Height          float64
	MinWidth        float64
	MinHeight       float64
	MaxWidth        float64
	MaxHeight       float64
	Overflow        string
	BorderRadius    float64
	Shadow          PaperPlanBoxShadowStyle
	KeepTogether    bool
	KeepWithNext    bool
	Orphans         uint32
	Widows          uint32
}

// PaperPlanBindingProvenance is the exact data path used by a compiled text
// node. The path is source metadata, not a value read from the scenario.
type PaperPlanBindingProvenance struct {
	Node       string              `json:"node,omitempty"`
	Kind       string              `json:"kind"`
	Path       string              `json:"path"`
	Source     PaperPlanSourceSpan `json:"source"`
	Nullable   bool                `json:"nullable,omitempty"`
	Collection bool                `json:"collection,omitempty"`
}

// PaperPlanTokenStep is one authored token or alias declaration in a
// resolved style-token chain.
type PaperPlanTokenStep struct {
	Theme  string              `json:"theme"`
	Scope  []string            `json:"scope,omitempty"`
	Token  string              `json:"token"`
	Source PaperPlanSourceSpan `json:"source"`
}

// PaperPlanStyleTokenProvenance describes one computed property and the
// exact theme/token chain that supplied its value.
type PaperPlanStyleTokenProvenance struct {
	Node       string               `json:"node,omitempty"`
	Kind       string               `json:"kind"`
	Property   string               `json:"property"`
	Theme      string               `json:"theme"`
	Token      string               `json:"token"`
	Value      string               `json:"value"`
	Consumer   PaperPlanSourceSpan  `json:"consumer"`
	TokenChain []PaperPlanTokenStep `json:"token_chain"`
}

// PaperPlanComputedStyleProvenance is the resolved, renderer-independent
// style attached to one readable source block. It is exact compiler output,
// not browser-computed CSS.
type PaperPlanComputedStyleProvenance struct {
	Node      string              `json:"node,omitempty"`
	Kind      string              `json:"kind"`
	Source    PaperPlanSourceSpan `json:"source"`
	TextStyle *PaperPlanTextStyle `json:"text_style,omitempty"`
	BoxStyle  *PaperPlanBoxStyle  `json:"box_style,omitempty"`
}

// PaperPlanExpansionProvenance distinguishes authored definitions from their
// concrete component/repeat invocation and instance path. It contains source
// coordinates only; scenario values are never retained here.
type PaperPlanExpansionProvenance struct {
	Node       string              `json:"node,omitempty"`
	Kind       string              `json:"kind"`
	Definition PaperPlanSourceSpan `json:"definition"`
	Invocation PaperPlanSourceSpan `json:"invocation"`
	Instance   string              `json:"instance,omitempty"`
}

// PaperPlanProvenance is a bounded, deterministic source projection attached
// to Explain responses. It contains paths and declarations, never scenario
// values or raw resource bytes.
type PaperPlanProvenance struct {
	Bindings       []PaperPlanBindingProvenance       `json:"bindings,omitempty"`
	StyleTokens    []PaperPlanStyleTokenProvenance    `json:"style_tokens,omitempty"`
	ComputedStyles []PaperPlanComputedStyleProvenance `json:"computed_styles,omitempty"`
	Expansions     []PaperPlanExpansionProvenance     `json:"expansions,omitempty"`
}

// Provenance returns detached binding and style-token evidence for this exact
// immutable plan. A zero plan returns an error rather than an ambiguous empty
// projection.
func (p PaperPlan) Provenance() (PaperPlanProvenance, error) {
	if p.hash == "" {
		return PaperPlanProvenance{}, fmt.Errorf("document: empty paper plan")
	}
	result := PaperPlanProvenance{
		Bindings:       make([]PaperPlanBindingProvenance, 0),
		StyleTokens:    make([]PaperPlanStyleTokenProvenance, 0),
		ComputedStyles: make([]PaperPlanComputedStyleProvenance, 0),
		Expansions:     make([]PaperPlanExpansionProvenance, 0),
	}
	nodes := make([]papercompile.NodeMapping, 0, len(p.mapping.Nodes)+len(p.mapping.AnonymousNodes))
	nodes = append(nodes, p.mapping.Nodes...)
	nodes = append(nodes, p.mapping.AnonymousNodes...)
	for _, node := range nodes {
		if node.InstancePath != "" || node.DefinitionSpan.File != "" || node.InvocationSpan.File != "" {
			result.Expansions = append(result.Expansions, PaperPlanExpansionProvenance{
				Node: node.ID, Kind: string(node.Kind), Definition: paperPlanSourceSpan(node.DefinitionSpan),
				Invocation: paperPlanSourceSpan(node.InvocationSpan), Instance: node.InstancePath,
			})
		}
		if node.BindingPath == "" {
			continue
		}
		result.Bindings = append(result.Bindings, PaperPlanBindingProvenance{
			Node: node.ID, Kind: string(node.Kind), Path: node.BindingPath,
			Source: paperPlanSourceSpan(node.BindingSpan), Nullable: node.BindingNullable,
			Collection: node.BindingCollection,
		})
	}
	for _, property := range p.mapping.ThemeProperties {
		chain := make([]PaperPlanTokenStep, len(property.Provenance.Chain))
		for index, step := range property.Provenance.Chain {
			chain[index] = PaperPlanTokenStep{Theme: step.Theme, Scope: append([]string(nil), step.Scope...), Token: step.Token, Source: paperThemeSourceSpan(step.Source)}
		}
		result.StyleTokens = append(result.StyleTokens, PaperPlanStyleTokenProvenance{
			Node: property.NodeID, Kind: string(property.Value.Kind), Property: property.Property,
			Theme: property.Theme, Token: property.Token, Value: paperThemeValueText(property.Value),
			Consumer: paperLangSourceSpan(property.ConsumerSpan), TokenChain: chain,
		})
	}
	for _, style := range p.mapping.ComputedStyles {
		entry := PaperPlanComputedStyleProvenance{Node: style.NodeID, Kind: string(style.NodeKind), Source: paperLangSourceSpan(style.Source)}
		if style.TextStyle != nil {
			copy := paperPlanTextStyle(*style.TextStyle)
			entry.TextStyle = &copy
		}
		if style.BoxStyle != nil {
			copy := paperPlanBoxStyle(*style.BoxStyle)
			entry.BoxStyle = &copy
		}
		result.ComputedStyles = append(result.ComputedStyles, entry)
	}
	return result, nil
}

func paperPlanTextStyle(style layout.TextStyle) PaperPlanTextStyle {
	return PaperPlanTextStyle{
		FontFamily: style.FontFamily, FontSize: style.FontSize, Bold: style.Bold, Italic: style.Italic,
		Underline: style.Underline, StrikeThrough: style.StrikeThrough, Color: paperPlanColor(style.Color),
		Align: style.Align, LineHeight: style.LineHeight, WhiteSpace: style.WhiteSpace, TabSize: style.TabSize,
	}
}

func paperPlanBoxStyle(style layout.BoxStyle) PaperPlanBoxStyle {
	return PaperPlanBoxStyle{
		Margin: paperPlanSpacing(style.Margin), Padding: paperPlanSpacing(style.Padding),
		Border: paperPlanBorderStyle(style.Border), BackgroundColor: paperPlanColor(style.BackgroundColor),
		Width: style.Width, Height: style.Height, MinWidth: style.MinWidth, MinHeight: style.MinHeight,
		MaxWidth: style.MaxWidth, MaxHeight: style.MaxHeight, Overflow: style.Overflow,
		BorderRadius: style.BorderRadius, Shadow: paperPlanBoxShadowStyle(style.Shadow),
		KeepTogether: style.KeepTogether, KeepWithNext: style.KeepWithNext, Orphans: style.Orphans, Widows: style.Widows,
	}
}

func paperPlanColor(color layout.DocumentColor) PaperPlanColor {
	return PaperPlanColor{R: color.R, G: color.G, B: color.B, Set: color.Set}
}

func paperPlanSpacing(spacing layout.Spacing) PaperPlanSpacing {
	return PaperPlanSpacing{Top: spacing.Top, Right: spacing.Right, Bottom: spacing.Bottom, Left: spacing.Left}
}

func paperPlanBorderStyle(border layout.BorderStyle) PaperPlanBorderStyle {
	return PaperPlanBorderStyle{
		Top: paperPlanBorderSide(border.Top), Right: paperPlanBorderSide(border.Right),
		Bottom: paperPlanBorderSide(border.Bottom), Left: paperPlanBorderSide(border.Left),
	}
}

func paperPlanBorderSide(side layout.BorderSide) PaperPlanBorderSide {
	return PaperPlanBorderSide{Width: side.Width, Style: side.Style, Color: paperPlanColor(side.Color)}
}

func paperPlanBoxShadowStyle(shadow layout.BoxShadowStyle) PaperPlanBoxShadowStyle {
	return PaperPlanBoxShadowStyle{
		OffsetX: shadow.OffsetX, OffsetY: shadow.OffsetY, Spread: shadow.Spread, Color: paperPlanColor(shadow.Color),
	}
}

func clonePaperCompileMapping(input papercompile.CompileMapping) papercompile.CompileMapping {
	result := papercompile.CompileMapping{
		SourceRevision:  input.SourceRevision,
		Nodes:           append([]papercompile.NodeMapping(nil), input.Nodes...),
		AnonymousNodes:  append([]papercompile.NodeMapping(nil), input.AnonymousNodes...),
		ThemeProperties: make([]papercompile.ThemePropertyMapping, len(input.ThemeProperties)),
		ComputedStyles:  make([]papercompile.ComputedStyleMapping, len(input.ComputedStyles)),
	}
	for index, property := range input.ThemeProperties {
		result.ThemeProperties[index] = property
		result.ThemeProperties[index].Provenance.Chain = make([]papertheme.TokenStep, len(property.Provenance.Chain))
		for stepIndex, step := range property.Provenance.Chain {
			result.ThemeProperties[index].Provenance.Chain[stepIndex] = papertheme.TokenStep{
				Theme: step.Theme, Scope: append([]string(nil), step.Scope...), Token: step.Token, Source: step.Source,
			}
		}
	}
	for index, style := range input.ComputedStyles {
		result.ComputedStyles[index] = style
		if style.TextStyle != nil {
			copy := *style.TextStyle
			result.ComputedStyles[index].TextStyle = &copy
		}
		if style.BoxStyle != nil {
			copy := *style.BoxStyle
			result.ComputedStyles[index].BoxStyle = &copy
		}
	}
	return result
}

func paperPlanSourceSpan(span paperlang.Span) PaperPlanSourceSpan {
	return paperLangSourceSpan(span)
}

func paperLangSourceSpan(span paperlang.Span) PaperPlanSourceSpan {
	return PaperPlanSourceSpan{File: span.File, StartOffset: span.Start.Offset, EndOffset: span.End.Offset,
		StartLine: span.Start.Line, StartColumn: span.Start.Column, EndLine: span.End.Line, EndColumn: span.End.Column}
}

func paperThemeSourceSpan(source papertheme.Source) PaperPlanSourceSpan {
	return PaperPlanSourceSpan{File: source.File, StartOffset: source.StartOffset, EndOffset: source.EndOffset,
		StartLine: source.Line, StartColumn: source.Column}
}

func paperThemeValueText(value papertheme.Value) string {
	switch value.Kind {
	case papertheme.Color:
		return value.Color
	case papertheme.String:
		return value.String
	case papertheme.Length:
		return value.Length.Number + value.Length.Unit
	case papertheme.Number:
		return value.Number
	case papertheme.Bool:
		if value.Bool {
			return "true"
		}
		return "false"
	default:
		encoded, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(encoded)
	}
}
