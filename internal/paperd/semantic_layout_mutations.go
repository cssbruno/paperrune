// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

// PaperBoxProperty is the closed authored vocabulary exposed to box handles.
// Each request changes one readable property and therefore produces one
// minimal CST patch even when the property did not previously exist.
type PaperBoxProperty string

const (
	PaperBoxMargin            PaperBoxProperty = "margin"
	PaperBoxMarginTop         PaperBoxProperty = "margin-top"
	PaperBoxMarginRight       PaperBoxProperty = "margin-right"
	PaperBoxMarginBottom      PaperBoxProperty = "margin-bottom"
	PaperBoxMarginLeft        PaperBoxProperty = "margin-left"
	PaperBoxPadding           PaperBoxProperty = "padding"
	PaperBoxPaddingTop        PaperBoxProperty = "padding-top"
	PaperBoxPaddingRight      PaperBoxProperty = "padding-right"
	PaperBoxPaddingBottom     PaperBoxProperty = "padding-bottom"
	PaperBoxPaddingLeft       PaperBoxProperty = "padding-left"
	PaperBoxBorderWidth       PaperBoxProperty = "border-width"
	PaperBoxBorderTopWidth    PaperBoxProperty = "border-top-width"
	PaperBoxBorderRightWidth  PaperBoxProperty = "border-right-width"
	PaperBoxBorderBottomWidth PaperBoxProperty = "border-bottom-width"
	PaperBoxBorderLeftWidth   PaperBoxProperty = "border-left-width"
	PaperBoxBorderColor       PaperBoxProperty = "border-color"
	PaperBoxRadius            PaperBoxProperty = "border-radius"
	PaperBoxBackground        PaperBoxProperty = "background"
)

type PaperPageMarginProperty string

const (
	PaperPageMarginAll    PaperPageMarginProperty = "margin"
	PaperPageMarginTop    PaperPageMarginProperty = "margin-top"
	PaperPageMarginRight  PaperPageMarginProperty = "margin-right"
	PaperPageMarginBottom PaperPageMarginProperty = "margin-bottom"
	PaperPageMarginLeft   PaperPageMarginProperty = "margin-left"
)

type PaperSetPageMarginRequest struct {
	Guard    PaperMutationGuard      `json:"guard"`
	Property PaperPageMarginProperty `json:"property"`
	Points   float64                 `json:"points"`
}

type PaperSetPageSizeRequest struct {
	Guard        PaperMutationGuard `json:"guard"`
	WidthPoints  float64            `json:"width_points"`
	HeightPoints float64            `json:"height_points"`
}

type PaperCanvasAnchorProperty string

const (
	PaperCanvasLeft    PaperCanvasAnchorProperty = "left"
	PaperCanvasRight   PaperCanvasAnchorProperty = "right"
	PaperCanvasCenterX PaperCanvasAnchorProperty = "center-x"
	PaperCanvasTop     PaperCanvasAnchorProperty = "top"
	PaperCanvasBottom  PaperCanvasAnchorProperty = "bottom"
	PaperCanvasCenterY PaperCanvasAnchorProperty = "center-y"
)

type PaperCanvasItemProperty string

const (
	PaperCanvasItemWidth  PaperCanvasItemProperty = "width"
	PaperCanvasItemHeight PaperCanvasItemProperty = "height"
	PaperCanvasItemAlt    PaperCanvasItemProperty = "alt"
)

type PaperSetCanvasItemRequest struct {
	Guard        PaperMutationGuard        `json:"guard"`
	Property     string                    `json:"property"`
	Reference    string                    `json:"reference"`
	TargetAnchor PaperCanvasAnchorProperty `json:"target_anchor"`
	Offset       float64                   `json:"offset_points,omitempty"`
	Points       float64                   `json:"points,omitempty"`
	Text         string                    `json:"text,omitempty"`
}

func canvasAnchorAxis(property PaperCanvasAnchorProperty) byte {
	switch property {
	case PaperCanvasLeft, PaperCanvasRight, PaperCanvasCenterX:
		return 'x'
	case PaperCanvasTop, PaperCanvasBottom, PaperCanvasCenterY:
		return 'y'
	default:
		return 0
	}
}

// PaperSetCanvasItem edits one addressed item inside its governing canvas.
// The canvas is an explicit transitive effect because size and constraint
// changes can alter every dependent sibling.
func (w *Workspace) PaperSetCanvasItem(request PaperSetCanvasItemRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node, parent := sourceNodeAndParent(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || parent == nil || node.Kind != paperlang.NodeAnchor || parent.Kind != paperlang.NodeCanvas || parent.ID == "" {
		return PaperMutationResult{}, workspaceError("INVALID_CANVAS_ITEM_TARGET", "canvas-item controls require an exact anchor directly inside an addressed canvas", paperedit.ErrInvalidOperation)
	}
	if err := requireAdditionalTargetGuard(revision, request.Guard, parent.ID); err != nil {
		return PaperMutationResult{}, err
	}
	var value paperedit.Value
	property := PaperCanvasAnchorProperty(request.Property)
	axis := canvasAnchorAxis(property)
	switch PaperCanvasItemProperty(request.Property) {
	case PaperCanvasItemWidth, PaperCanvasItemHeight:
		if request.Reference != "" || request.TargetAnchor != "" || request.Offset != 0 || request.Text != "" || !finiteLayoutHandle(request.Points) || request.Points <= 0 || request.Points > 1_000_000 {
			return PaperMutationResult{}, workspaceError("INVALID_CANVAS_ITEM_VALUE", "canvas item dimensions must be finite positive point values without unrelated fields", paperedit.ErrInvalidOperation)
		}
		value = paperedit.UnitValue(request.Points, "pt")
	case PaperCanvasItemAlt:
		if request.Reference != "" || request.TargetAnchor != "" || request.Offset != 0 || request.Points != 0 || !utf8.ValidString(request.Text) || len(request.Text) > w.maxMutationPayloadBytes() {
			return PaperMutationResult{}, workspaceError("INVALID_CANVAS_ITEM_VALUE", "canvas item alt text must be bounded valid UTF-8 without unrelated fields", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(request.Text)
	default:
		if axis == 0 || canvasAnchorAxis(request.TargetAnchor) != axis || request.Points != 0 || request.Text != "" {
			return PaperMutationResult{}, workspaceError("INVALID_CANVAS_ITEM_PROPERTY", "canvas constraints require supported same-axis source and target anchors", paperedit.ErrInvalidOperation)
		}
		if request.Reference != "canvas" {
			reference := findNodeByID(revision.parsed.AST.Root, request.Reference)
			_, referenceParent := sourceNodeAndParent(revision.parsed.AST.Root, request.Reference)
			if reference == nil || reference.Kind != paperlang.NodeAnchor || referenceParent != parent || reference.ID == node.ID {
				return PaperMutationResult{}, workspaceError("INVALID_CANVAS_ITEM_REFERENCE", "canvas reference must be a different addressed sibling in the same canvas", paperedit.ErrInvalidOperation)
			}
		}
		if !finiteLayoutHandle(request.Offset) || request.Offset < -1_000_000 || request.Offset > 1_000_000 {
			return PaperMutationResult{}, workspaceError("INVALID_CANVAS_ITEM_OFFSET", "canvas offset must be a finite bounded point value", paperedit.ErrInvalidOperation)
		}
		expression := request.Reference + "." + string(request.TargetAnchor)
		if request.Offset > 0 {
			expression += " + " + strconv.FormatFloat(request.Offset, 'f', -1, 64) + "pt"
		} else if request.Offset < 0 {
			expression += " - " + strconv.FormatFloat(-request.Offset, 'f', -1, 64) + "pt"
		}
		value = paperedit.StringValue(expression)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: request.Property, Value: value}
	return w.applyPaperMutation("set_canvas_item", request.Guard, opened, revision, []string{request.Guard.Target, parent.ID}, []paperedit.Operation{operation}, "INVALID_CANVAS_ITEM_STATE")
}

// PaperSetPageMargin edits the authored page-master body region instead of a
// computed page box. The page node is the governing source node and the full
// candidate is recompiled before publication.
func (w *Workspace) PaperSetPageMargin(request PaperSetPageMarginRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodePage {
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_MASTER_TARGET", "page-master margin handles require an exact page source node", paperedit.ErrInvalidOperation)
	}
	switch request.Property {
	case PaperPageMarginAll, PaperPageMarginTop, PaperPageMarginRight, PaperPageMarginBottom, PaperPageMarginLeft:
	default:
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_MARGIN_PROPERTY", "page margin is outside the closed handle vocabulary", paperedit.ErrInvalidOperation)
	}
	if !finiteLayoutHandle(request.Points) || request.Points < 0 || request.Points > 1_000_000 {
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_MARGIN_VALUE", "page margin must be a finite non-negative point value", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: paperedit.UnitValue(request.Points, "pt")}
	return w.applyPaperMutation("set_page_margin", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_PAGE_MASTER")
}

// PaperSetPageSize writes explicit physical page dimensions. Presets are
// resolved by the caller before this boundary so the retained source remains
// unambiguous and custom sizes use the identical mutation path.
func (w *Workspace) PaperSetPageSize(request PaperSetPageSizeRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodePage {
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_SIZE_TARGET", "page-size handles require an exact page source node", paperedit.ErrInvalidOperation)
	}
	if !finiteLayoutHandle(request.WidthPoints) || !finiteLayoutHandle(request.HeightPoints) ||
		request.WidthPoints <= 0 || request.HeightPoints <= 0 || request.WidthPoints > 1_000_000 || request.HeightPoints > 1_000_000 {
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_SIZE_VALUE", "page dimensions must be finite positive point values", paperedit.ErrInvalidOperation)
	}
	operations := []paperedit.Operation{
		paperedit.SetProperty{Target: request.Guard.Target, Name: "width", Value: paperedit.UnitValue(request.WidthPoints, "pt")},
		paperedit.SetProperty{Target: request.Guard.Target, Name: "height", Value: paperedit.UnitValue(request.HeightPoints, "pt")},
	}
	return w.applyPaperMutation("set_page_size", request.Guard, opened, revision, []string{request.Guard.Target}, operations, "INVALID_PAGE_SIZE")
}

type PaperSetPageRegionRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	Property string             `json:"property"`
	Points   float64            `json:"points,omitempty"`
	Color    string             `json:"color,omitempty"`
	Bool     bool               `json:"bool,omitempty"`
}

// PaperSetPageRegion writes the authored header/footer node and treats the
// governing page master as a transitive effect because region measurement
// changes the body's available rectangle.
func (w *Workspace) PaperSetPageRegion(request PaperSetPageRegionRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node, parent := sourceNodeAndParent(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || parent == nil || (node.Kind != paperlang.NodeHeader && node.Kind != paperlang.NodeFooter) || parent.Kind != paperlang.NodePage || parent.ID == "" {
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_REGION_TARGET", "page-region handles require an exact header or footer directly inside an addressed page", paperedit.ErrInvalidOperation)
	}
	if err := requireAdditionalTargetGuard(revision, request.Guard, parent.ID); err != nil {
		return PaperMutationResult{}, err
	}
	var value paperedit.Value
	property := PaperBoxProperty(request.Property)
	switch {
	case property.length():
		if request.Color != "" || request.Bool || !finiteLayoutHandle(request.Points) || request.Points < 0 || request.Points > 1_000_000 {
			return PaperMutationResult{}, workspaceError("INVALID_PAGE_REGION_VALUE", "region length must be a finite non-negative point value", paperedit.ErrInvalidOperation)
		}
		value = paperedit.UnitValue(request.Points, "pt")
	case property.color():
		color, ok := canonicalLayoutHandleColor(request.Color)
		if !ok || request.Points != 0 || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_PAGE_REGION_VALUE", "region color must be canonical #RRGGBB", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(color)
	default:
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_REGION_PROPERTY", "page region property is outside the closed handle vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: request.Property, Value: value}
	return w.applyPaperMutation("set_page_region", request.Guard, opened, revision, []string{request.Guard.Target, parent.ID}, []paperedit.Operation{operation}, "INVALID_PAGE_REGION")
}

func (property PaperBoxProperty) length() bool {
	switch property {
	case PaperBoxMargin, PaperBoxMarginTop, PaperBoxMarginRight, PaperBoxMarginBottom, PaperBoxMarginLeft,
		PaperBoxPadding, PaperBoxPaddingTop, PaperBoxPaddingRight, PaperBoxPaddingBottom, PaperBoxPaddingLeft,
		PaperBoxBorderWidth, PaperBoxBorderTopWidth, PaperBoxBorderRightWidth, PaperBoxBorderBottomWidth,
		PaperBoxBorderLeftWidth, PaperBoxRadius:
		return true
	default:
		return false
	}
}

func (property PaperBoxProperty) color() bool {
	return property == PaperBoxBorderColor || property == PaperBoxBackground
}

type PaperSetBoxPropertyRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	Property PaperBoxProperty   `json:"property"`
	Points   float64            `json:"points,omitempty"`
	Color    string             `json:"color,omitempty"`
}

// PaperTextProperty is the closed authored typography vocabulary exposed by
// text-style handles. Font replacement is explicit and never automatic.
type PaperTextProperty string

const (
	PaperTextFont       PaperTextProperty = "font"
	PaperTextSize       PaperTextProperty = "size"
	PaperTextLineHeight PaperTextProperty = "line-height"
	PaperTextColor      PaperTextProperty = "color"
	PaperTextAlign      PaperTextProperty = "align"
	PaperTextBold       PaperTextProperty = "bold"
	PaperTextItalic     PaperTextProperty = "italic"
	PaperTextLevel      PaperTextProperty = "level"
)

type PaperSetTextPropertyRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	Property PaperTextProperty  `json:"property"`
	Text     string             `json:"text,omitempty"`
	Points   float64            `json:"points,omitempty"`
	Length   string             `json:"length,omitempty"`
	Color    string             `json:"color,omitempty"`
	Kind     string             `json:"kind,omitempty"`
	Bool     bool               `json:"bool,omitempty"`
	Count    uint32             `json:"count,omitempty"`
}

func canonicalPaperCoreFont(value string) (string, bool) {
	switch value {
	case "Courier", "Helvetica", "Times", "Symbol", "ZapfDingbats":
		return value, true
	default:
		return "", false
	}
}

// PaperSetTextProperty replaces one authored text-style property. The source
// may fail compilation because of the old font; the replacement is published
// only when the complete candidate compiles.
func (w *Workspace) PaperSetTextProperty(request PaperSetTextPropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodeParagraph && node.Kind != paperlang.NodeHeading && node.Kind != paperlang.NodeList && node.Kind != paperlang.NodeTableCell {
		return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_TARGET", "typography controls require a paragraph, heading, list, or table cell", paperedit.ErrInvalidOperation)
	}
	var value paperedit.Value
	switch request.Property {
	case PaperTextFont:
		font, ok := canonicalPaperCoreFont(request.Text)
		if !ok || request.Points != 0 || request.Length != "" || request.Color != "" || request.Kind != "" || request.Bool || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_VALUE", "font must name an existing supported core font", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(font)
	case PaperTextSize, PaperTextLineHeight:
		resolved, ok := physicalLayoutLengthValue(request.Length, request.Points, true)
		if !ok || request.Text != "" || request.Color != "" || request.Kind != "" || request.Bool || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_VALUE", "text size must be a finite positive physical length", paperedit.ErrInvalidOperation)
		}
		value = resolved
	case PaperTextColor:
		color, ok := canonicalLayoutHandleColor(request.Color)
		if !ok || request.Text != "" || request.Points != 0 || request.Length != "" || request.Kind != "" || request.Bool || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_VALUE", "text color must be canonical #RRGGBB", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(color)
	case PaperTextAlign:
		align := strings.ToLower(strings.TrimSpace(request.Kind))
		if !layoutChoice(align, "left", "center", "right", "justify") || request.Text != "" || request.Points != 0 || request.Length != "" || request.Color != "" || request.Bool || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_VALUE", "text alignment must be left, center, right, or justify", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(align)
	case PaperTextBold, PaperTextItalic:
		if request.Text != "" || request.Points != 0 || request.Length != "" || request.Color != "" || request.Kind != "" || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_VALUE", "bold and italic accept only a boolean value", paperedit.ErrInvalidOperation)
		}
		value = paperedit.BoolValue(request.Bool)
	case PaperTextLevel:
		if node.Kind != paperlang.NodeHeading || request.Count < 1 || request.Count > 6 || request.Text != "" || request.Points != 0 || request.Length != "" || request.Color != "" || request.Kind != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_VALUE", "heading level must be an integer from 1 through 6 without unrelated fields", paperedit.ErrInvalidOperation)
		}
		value = paperedit.NumberValue(float64(request.Count))
	default:
		return PaperMutationResult{}, workspaceError("INVALID_TEXT_STYLE_PROPERTY", "text property is outside the supported editing vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: value}
	return w.applyPaperMutation("set_text_property", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_TEXT_STYLE")
}

type PaperListProperty string

const (
	PaperListOrdered PaperListProperty = "ordered"
	PaperListMarker  PaperListProperty = "marker"
)

type PaperSetListPropertyRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	Property PaperListProperty  `json:"property"`
	Marker   string             `json:"marker,omitempty"`
	Bool     bool               `json:"bool,omitempty"`
}

// PaperSetListProperty keeps ordering and marker semantics consistent in one
// candidate so Studio never has to publish a contradictory intermediate list.
func (w *Workspace) PaperSetListProperty(request PaperSetListPropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodeList {
		return PaperMutationResult{}, workspaceError("INVALID_LIST_TARGET", "list controls require an exact list source node", paperedit.ErrInvalidOperation)
	}
	operations := make([]paperedit.Operation, 0, 2)
	set := func(name string, value paperedit.Value) {
		operations = append(operations, paperedit.SetProperty{Target: request.Guard.Target, Name: name, Value: value})
	}
	switch request.Property {
	case PaperListOrdered:
		if request.Marker != "" {
			return PaperMutationResult{}, workspaceError("INVALID_LIST_VALUE", "ordered accepts only a boolean value", paperedit.ErrInvalidOperation)
		}
		set("ordered", paperedit.BoolValue(request.Bool))
		marker := nodeAuthoredStringProperty(node, "marker")
		if request.Bool && marker != "decimal" {
			set("marker", paperedit.StringValue("decimal"))
		} else if !request.Bool && marker == "decimal" {
			set("marker", paperedit.StringValue("dash"))
		}
	case PaperListMarker:
		marker := strings.ToLower(strings.TrimSpace(request.Marker))
		if request.Bool || !layoutChoice(marker, "decimal", "dash", "asterisk") {
			return PaperMutationResult{}, workspaceError("INVALID_LIST_VALUE", "marker must be decimal, dash, or asterisk", paperedit.ErrInvalidOperation)
		}
		set("marker", paperedit.StringValue(marker))
		ordered := marker == "decimal"
		if nodeAuthoredBoolProperty(node, "ordered") != ordered {
			set("ordered", paperedit.BoolValue(ordered))
		}
	default:
		return PaperMutationResult{}, workspaceError("INVALID_LIST_PROPERTY", "list property is outside the supported editing vocabulary", paperedit.ErrInvalidOperation)
	}
	return w.applyPaperMutation("set_list_property", request.Guard, opened, revision, []string{request.Guard.Target}, operations, "INVALID_LIST_PROPERTY_STATE")
}

// PaperSetBoxProperty updates one box control on an authored boxed element.
// The typed compiler validates the complete candidate before publication.
func (w *Workspace) PaperSetBoxProperty(request PaperSetBoxPropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodeParagraph && node.Kind != paperlang.NodeHeading && node.Kind != paperlang.NodeList && node.Kind != paperlang.NodeImage && node.Kind != paperlang.NodeTableCell && node.Kind != paperlang.NodeAnchor {
		return PaperMutationResult{}, workspaceError("INVALID_BOX_TARGET", "box controls require an authored text block, image, table cell, or canvas item", paperedit.ErrInvalidOperation)
	}
	var value paperedit.Value
	switch {
	case request.Property.length():
		if request.Color != "" || !finiteLayoutHandle(request.Points) || request.Points < 0 || request.Points > 1_000_000 {
			return PaperMutationResult{}, workspaceError("INVALID_BOX_VALUE", "box length must be a finite non-negative point value", paperedit.ErrInvalidOperation)
		}
		value = paperedit.UnitValue(request.Points, "pt")
	case request.Property.color():
		color, ok := canonicalLayoutHandleColor(request.Color)
		if !ok || request.Points != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_BOX_VALUE", "box color must be canonical #RRGGBB", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(color)
	default:
		return PaperMutationResult{}, workspaceError("INVALID_BOX_PROPERTY", "box property is outside the closed handle vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: value}
	return w.applyPaperMutation("set_box_property", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_BOX_STYLE")
}

type PaperLayoutItemProperty string

const (
	PaperLayoutItemWidth      PaperLayoutItemProperty = "width"
	PaperLayoutItemMinWidth   PaperLayoutItemProperty = "min-width"
	PaperLayoutItemMaxWidth   PaperLayoutItemProperty = "max-width"
	PaperLayoutItemHeight     PaperLayoutItemProperty = "height"
	PaperLayoutItemMinHeight  PaperLayoutItemProperty = "min-height"
	PaperLayoutItemMaxHeight  PaperLayoutItemProperty = "max-height"
	PaperLayoutItemFlexGrow   PaperLayoutItemProperty = "flex-grow"
	PaperLayoutItemFlexShrink PaperLayoutItemProperty = "flex-shrink"
	PaperLayoutItemAlignSelf  PaperLayoutItemProperty = "align-self"
)

type PaperSetLayoutItemRequest struct {
	Guard    PaperMutationGuard      `json:"guard"`
	Property PaperLayoutItemProperty `json:"property"`
	Kind     string                  `json:"kind,omitempty"`
	Points   float64                 `json:"points,omitempty"`
	Length   string                  `json:"length,omitempty"`
	Factor   float64                 `json:"factor,omitempty"`
}

type PaperLayoutContainerProperty string

const (
	PaperLayoutGap            PaperLayoutContainerProperty = "gap"
	PaperLayoutLineGap        PaperLayoutContainerProperty = "line-gap"
	PaperLayoutWidth          PaperLayoutContainerProperty = "width"
	PaperLayoutHeight         PaperLayoutContainerProperty = "height"
	PaperLayoutWrap           PaperLayoutContainerProperty = "wrap"
	PaperLayoutJustifyContent PaperLayoutContainerProperty = "justify-content"
	PaperLayoutAlignItems     PaperLayoutContainerProperty = "align-items"
	PaperLayoutAlignContent   PaperLayoutContainerProperty = "align-content"
	PaperLayoutReverse        PaperLayoutContainerProperty = "reverse"
)

type PaperSetLayoutContainerRequest struct {
	Guard    PaperMutationGuard           `json:"guard"`
	Property PaperLayoutContainerProperty `json:"property"`
	Points   float64                      `json:"points,omitempty"`
	Length   string                       `json:"length,omitempty"`
	Kind     string                       `json:"kind,omitempty"`
	Bool     bool                         `json:"bool,omitempty"`
}

type PaperImageProperty string

const (
	PaperImageFit        PaperImageProperty = "fit"
	PaperImageFocusX     PaperImageProperty = "focus-x"
	PaperImageFocusY     PaperImageProperty = "focus-y"
	PaperImageWidth      PaperImageProperty = "width"
	PaperImageHeight     PaperImageProperty = "height"
	PaperImageMaxWidth   PaperImageProperty = "max-width"
	PaperImageMaxHeight  PaperImageProperty = "max-height"
	PaperImageAlign      PaperImageProperty = "align"
	PaperImageCaption    PaperImageProperty = "caption"
	PaperImageAlt        PaperImageProperty = "alt"
	PaperImageDecorative PaperImageProperty = "decorative"
	PaperImageSource     PaperImageProperty = "source"
)

type PaperSetImagePropertyRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	Property PaperImageProperty `json:"property"`
	Fit      string             `json:"fit,omitempty"`
	Number   float64            `json:"number,omitempty"`
	Points   float64            `json:"points,omitempty"`
	Length   string             `json:"length,omitempty"`
	Text     string             `json:"text,omitempty"`
	Bool     bool               `json:"bool,omitempty"`
}

// PaperSetImageProperty changes one authored image concern. Accessibility
// transitions may update alt and decorative together because publishing an
// invalid intermediate accessibility state is forbidden.
func (w *Workspace) PaperSetImageProperty(request PaperSetImagePropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodeImage {
		return PaperMutationResult{}, workspaceError("INVALID_IMAGE_TARGET", "image handles require an exact image source node", paperedit.ErrInvalidOperation)
	}
	operations := make([]paperedit.Operation, 0, 2)
	add := func(name string, value paperedit.Value) {
		operations = append(operations, paperedit.SetProperty{Target: request.Guard.Target, Name: name, Value: value})
	}
	switch request.Property {
	case PaperImageSource:
		if request.Fit != "" || request.Number != 0 || request.Points != 0 || request.Length != "" || request.Bool || !strings.HasPrefix(request.Text, "asset:") || len(request.Text) <= len("asset:") {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "image source replacement must be one exact asset:name reference", paperedit.ErrInvalidOperation)
		}
		add("source", paperedit.StringValue(request.Text))
	case PaperImageFit:
		fit := strings.ToLower(strings.TrimSpace(request.Fit))
		if (fit != "auto" && fit != "contain" && fit != "cover") || request.Number != 0 || request.Points != 0 || request.Length != "" || request.Text != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "image fit must be auto, contain, or cover without unrelated values", paperedit.ErrInvalidOperation)
		}
		add("fit", paperedit.StringValue(fit))
	case PaperImageFocusX, PaperImageFocusY:
		if request.Fit != "" || request.Points != 0 || request.Length != "" || request.Text != "" || request.Bool || !finiteLayoutHandle(request.Number) || request.Number < 0 || request.Number > 1 {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "image focus must be a finite number between 0 and 1", paperedit.ErrInvalidOperation)
		}
		add(string(request.Property), paperedit.NumberValue(request.Number))
	case PaperImageWidth, PaperImageHeight, PaperImageMaxWidth, PaperImageMaxHeight:
		value, ok := responsiveLayoutLengthValue(request.Length, request.Points, true, true)
		if request.Fit != "" || request.Number != 0 || request.Text != "" || request.Bool || !ok ||
			(request.Property == PaperImageHeight || request.Property == PaperImageMaxHeight) && strings.HasSuffix(strings.TrimSpace(request.Length), "%") {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "image dimension must be auto, a positive physical length, or a width percentage", paperedit.ErrInvalidOperation)
		}
		add(string(request.Property), value)
	case PaperImageAlign:
		align := strings.ToLower(strings.TrimSpace(request.Fit))
		if !layoutChoice(align, "left", "center", "right") || request.Number != 0 || request.Points != 0 || request.Length != "" || request.Text != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "image alignment must be left, center, or right", paperedit.ErrInvalidOperation)
		}
		add("align", paperedit.StringValue(align))
	case PaperImageCaption:
		if request.Fit != "" || request.Number != 0 || request.Points != 0 || request.Length != "" || request.Bool || !utf8.ValidString(request.Text) || len(request.Text) > w.maxMutationPayloadBytes() {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "image caption must be valid bounded UTF-8", paperedit.ErrInvalidOperation)
		}
		add("caption", paperedit.StringValue(request.Text))
	case PaperImageAlt:
		if request.Fit != "" || request.Number != 0 || request.Points != 0 || request.Length != "" || request.Bool || !utf8.ValidString(request.Text) || len(request.Text) > w.maxMutationPayloadBytes() {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "image alt text must be valid bounded UTF-8 without unrelated values", paperedit.ErrInvalidOperation)
		}
		add("alt", paperedit.StringValue(request.Text))
		if request.Text != "" && nodeAuthoredBoolProperty(node, "decorative") {
			add("decorative", paperedit.BoolValue(false))
		}
	case PaperImageDecorative:
		if request.Fit != "" || request.Number != 0 || request.Points != 0 || request.Length != "" || request.Text != "" {
			return PaperMutationResult{}, workspaceError("INVALID_IMAGE_VALUE", "decorative accepts only its boolean value", paperedit.ErrInvalidOperation)
		}
		add("decorative", paperedit.BoolValue(request.Bool))
		if request.Bool && nodeAuthoredStringProperty(node, "alt") != "" {
			add("alt", paperedit.StringValue(""))
		}
	default:
		return PaperMutationResult{}, workspaceError("INVALID_IMAGE_PROPERTY", "image property is outside the closed handle vocabulary", paperedit.ErrInvalidOperation)
	}
	return w.applyPaperMutation("set_image_property", request.Guard, opened, revision, []string{request.Guard.Target}, operations, "INVALID_IMAGE_PROPERTY_STATE")
}

func nodeAuthoredStringProperty(node *paperlang.Node, name string) string {
	for _, member := range node.Members {
		if member.Property != nil && member.Property.Name == name && member.Property.Value.StringValue != nil {
			return *member.Property.Value.StringValue
		}
	}
	return ""
}

func nodeAuthoredBoolProperty(node *paperlang.Node, name string) bool {
	for _, member := range node.Members {
		if member.Property != nil && member.Property.Name == name && member.Property.Value.BoolValue != nil {
			return *member.Property.Value.BoolValue
		}
	}
	return false
}

// PaperSetLayoutContainer changes one authored row/column setting. Container
// dimensions are directional: rows have an explicit height and columns have
// an explicit width; their flow-axis size is determined by the containing flow.
func (w *Workspace) PaperSetLayoutContainer(request PaperSetLayoutContainerRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodeRow && node.Kind != paperlang.NodeColumn {
		return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_TARGET", "layout controls require an exact row or column", paperedit.ErrInvalidOperation)
	}

	var value paperedit.Value
	switch request.Property {
	case PaperLayoutGap, PaperLayoutLineGap:
		resolved, ok := physicalLayoutLengthValue(request.Length, request.Points, false)
		if request.Kind != "" || request.Bool || !ok {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_VALUE", "gap must be a finite non-negative physical length", paperedit.ErrInvalidOperation)
		}
		value = resolved
	case PaperLayoutWidth, PaperLayoutHeight:
		directional := node.Kind == paperlang.NodeRow && request.Property == PaperLayoutHeight || node.Kind == paperlang.NodeColumn && request.Property == PaperLayoutWidth
		resolved, ok := physicalLayoutLengthValue(request.Length, request.Points, true)
		if !directional || request.Kind != "" || request.Bool || !ok {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_VALUE", "rows accept a physical height and columns accept a physical width", paperedit.ErrInvalidOperation)
		}
		value = resolved
	case PaperLayoutWrap:
		kind := strings.ToLower(strings.TrimSpace(request.Kind))
		if kind != "nowrap" && kind != "wrap" && kind != "wrap-reverse" || request.Points != 0 || request.Length != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_VALUE", "wrap must be nowrap, wrap, or wrap-reverse", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(kind)
	case PaperLayoutJustifyContent:
		kind := strings.ToLower(strings.TrimSpace(request.Kind))
		if !layoutChoice(kind, "start", "center", "end", "space-between", "space-around", "space-evenly") || request.Points != 0 || request.Length != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_VALUE", "justify-content must be start, center, end, space-between, space-around, or space-evenly", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(kind)
	case PaperLayoutAlignItems:
		kind := strings.ToLower(strings.TrimSpace(request.Kind))
		if !layoutChoice(kind, "start", "center", "end", "stretch") || request.Points != 0 || request.Length != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_VALUE", "align-items must be start, center, end, or stretch", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(kind)
	case PaperLayoutAlignContent:
		kind := strings.ToLower(strings.TrimSpace(request.Kind))
		if !layoutChoice(kind, "start", "center", "end", "stretch", "space-between", "space-around", "space-evenly") || request.Points != 0 || request.Length != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_VALUE", "align-content must be a supported line alignment", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(kind)
	case PaperLayoutReverse:
		if request.Points != 0 || request.Length != "" || request.Kind != "" {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_VALUE", "reverse accepts only a boolean value", paperedit.ErrInvalidOperation)
		}
		value = paperedit.BoolValue(request.Bool)
	default:
		return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_CONTAINER_PROPERTY", "layout property is outside the supported container vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: value}
	return w.applyPaperMutation("set_layout_container", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_LAYOUT_CONTAINER")
}

func layoutChoice(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

// PaperSetLayoutItem changes one readable layout-item property on a direct
// paragraph, heading, image, or table child of a row or column. The parent is an explicit direct
// authorization effect and requires its own exact target precondition because
// changing one item can reposition every sibling.
func (w *Workspace) PaperSetLayoutItem(request PaperSetLayoutItemRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node, parent := sourceNodeAndParent(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || parent == nil || (node.Kind != paperlang.NodeParagraph && node.Kind != paperlang.NodeHeading && node.Kind != paperlang.NodeImage && node.Kind != paperlang.NodeTable && node.Kind != paperlang.NodeRow && node.Kind != paperlang.NodeColumn) ||
		(parent.Kind != paperlang.NodeRow && parent.Kind != paperlang.NodeColumn) || parent.ID == "" {
		return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_ITEM_TARGET", "layout controls require a readable child directly inside a readable row or column", paperedit.ErrInvalidOperation)
	}
	if err := requireAdditionalTargetGuard(revision, request.Guard, parent.ID); err != nil {
		return PaperMutationResult{}, err
	}
	var value paperedit.Value
	propertyName := string(request.Property)
	mainSize := PaperLayoutItemWidth
	if parent.Kind == paperlang.NodeColumn {
		mainSize = PaperLayoutItemHeight
	}
	switch request.Property {
	case PaperLayoutItemWidth, PaperLayoutItemMinWidth, PaperLayoutItemMaxWidth, PaperLayoutItemHeight, PaperLayoutItemMinHeight, PaperLayoutItemMaxHeight:
		isSize := request.Property == PaperLayoutItemWidth || request.Property == PaperLayoutItemHeight
		resolved, ok := responsiveLayoutLengthValue(request.Length, request.Points, true, isSize)
		if request.Property == mainSize && strings.HasSuffix(strings.ToLower(strings.TrimSpace(request.Length)), "fr") {
			resolved, ok = fractionalTrackLengthValue(request.Length, request.Points)
		} else if strings.HasSuffix(strings.ToLower(strings.TrimSpace(request.Length)), "fr") {
			ok = false
		}
		if request.Kind != "" || request.Factor != 0 || !ok {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_ITEM_VALUE", "width or height must be auto, a bounded percentage, a finite physical length, or a positive whole fraction on the parent flow axis", paperedit.ErrInvalidOperation)
		}
		value = resolved
	case PaperLayoutItemFlexGrow, PaperLayoutItemFlexShrink:
		if request.Kind != "" || request.Points != 0 || request.Length != "" || !validLayoutFlexFactor(request.Factor) {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_ITEM_VALUE", "flex factor must be between 0 and 4294.967295 with at most six decimal places", paperedit.ErrInvalidOperation)
		}
		value = paperedit.NumberValue(request.Factor)
	case PaperLayoutItemAlignSelf:
		align := strings.ToLower(strings.TrimSpace(request.Kind))
		switch align {
		case "flex-start":
			align = "start"
		case "flex-end":
			align = "end"
		}
		if align != "start" && align != "center" && align != "end" && align != "stretch" || request.Points != 0 || request.Length != "" || request.Factor != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_ITEM_VALUE", "alignment must be start, center, end, or stretch", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(align)
	default:
		return PaperMutationResult{}, workspaceError("INVALID_LAYOUT_ITEM_PROPERTY", "layout property is outside the supported editing vocabulary", paperedit.ErrInvalidOperation)
	}
	operations := []paperedit.Operation{paperedit.SetProperty{Target: request.Guard.Target, Name: propertyName, Value: value}}
	return w.applyPaperMutation("set_layout_item", request.Guard, opened, revision, []string{request.Guard.Target, parent.ID}, operations, "INVALID_LAYOUT_ITEM")
}

func finiteLayoutHandle(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func validLayoutFlexFactor(value float64) bool {
	if !finiteLayoutHandle(value) || value < 0 || value > 4294.967295 {
		return false
	}
	scaled := value * 1_000_000
	return math.Abs(scaled-math.Round(scaled)) <= 0.000001
}

func physicalLayoutLengthValue(length string, points float64, positive bool) (paperedit.Value, bool) {
	length = strings.ToLower(strings.TrimSpace(length))
	if length != "" && !strings.HasSuffix(length, "pt") {
		return paperedit.Value{}, false
	}
	return responsiveLayoutLengthValue(length, points, false, positive)
}

func responsiveLayoutLengthValue(length string, points float64, allowAuto, positive bool) (paperedit.Value, bool) {
	length = strings.ToLower(strings.TrimSpace(length))
	if length == "" {
		if !finiteLayoutHandle(points) || points < 0 || positive && points == 0 || points > 1_000_000 {
			return paperedit.Value{}, false
		}
		return paperedit.UnitValue(points, "pt"), true
	}
	if points != 0 {
		return paperedit.Value{}, false
	}
	if length == "auto" {
		if !allowAuto {
			return paperedit.Value{}, false
		}
		return paperedit.StringValue("auto"), true
	}
	var unit string
	switch {
	case strings.HasSuffix(length, "%"):
		unit = "%"
	case strings.HasSuffix(length, "pt"):
		unit = "pt"
	default:
		return paperedit.Value{}, false
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(length, unit)), 64)
	maximum := 1_000_000.0
	if unit == "%" {
		maximum = 100
	}
	if err != nil || !finiteLayoutHandle(number) || number < 0 || positive && number == 0 || number > maximum {
		return paperedit.Value{}, false
	}
	return paperedit.UnitValue(number, unit), true
}

func fractionalTrackLengthValue(length string, points float64) (paperedit.Value, bool) {
	length = strings.ToLower(strings.TrimSpace(length))
	if points != 0 || !strings.HasSuffix(length, "fr") {
		return paperedit.Value{}, false
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(length, "fr")), 64)
	if err != nil || !finiteLayoutHandle(number) || number <= 0 || number > math.MaxUint32 || math.Trunc(number) != number {
		return paperedit.Value{}, false
	}
	return paperedit.UnitValue(number, "fr"), true
}

func canonicalLayoutHandleColor(value string) (string, bool) {
	if len(value) != 7 || value[0] != '#' {
		return "", false
	}
	if _, err := strconv.ParseUint(value[1:], 16, 24); err != nil {
		return "", false
	}
	return strings.ToLower(value), true
}

func sourceNodeAndParent(root *paperlang.Node, target string) (*paperlang.Node, *paperlang.Node) {
	var found, parent *paperlang.Node
	var walk func(*paperlang.Node, *paperlang.Node)
	walk = func(node, owner *paperlang.Node) {
		if node == nil || found != nil {
			return
		}
		if node.ID == target {
			found, parent = node, owner
			return
		}
		for _, member := range node.Members {
			walk(member.Node, node)
		}
	}
	walk(root, nil)
	return found, parent
}

func requireAdditionalTargetGuard(revision *revisionRecord, guard PaperMutationGuard, target string) error {
	var matched *paperedit.TargetPrecondition
	for _, precondition := range guard.TargetPreconditions {
		if precondition.Target != target {
			continue
		}
		if matched != nil {
			return workspaceError("TRANSITIVE_PRECONDITION_INVALID", fmt.Sprintf("layout mutation declares transitive target %s more than once", target), paperedit.ErrInvalidOperation)
		}
		copy := precondition
		matched = &copy
	}
	if matched == nil || matched.ExpectedFingerprint == "" || matched.ExpectedInstance == "" {
		return workspaceError("TRANSITIVE_PRECONDITION_REQUIRED", fmt.Sprintf("layout mutation requires an exact precondition for transitive target %s", target), paperedit.ErrInvalidOperation)
	}
	actualFingerprint, err := paperedit.FingerprintNode(revision.file, revision.source, target)
	if err != nil {
		return workspaceError("TRANSITIVE_PRECONDITION_INVALID", fmt.Sprintf("layout mutation transitive target %s cannot be fingerprinted", target), paperedit.ErrInvalidOperation)
	}
	actualInstance, err := paperedit.SourceInstance(revision.file, revision.source, target)
	if err != nil {
		return workspaceError("TRANSITIVE_PRECONDITION_INVALID", fmt.Sprintf("layout mutation transitive target %s is not an exact source instance", target), paperedit.ErrInvalidOperation)
	}
	if matched.ExpectedFingerprint != actualFingerprint || matched.ExpectedInstance != actualInstance {
		return workspaceError("TRANSITIVE_PRECONDITION_CONFLICT", fmt.Sprintf("layout mutation transitive target %s changed after review", target), ErrRevisionConflict)
	}
	return nil
}
