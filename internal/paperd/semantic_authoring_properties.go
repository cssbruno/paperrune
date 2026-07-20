// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"strings"
	"unicode/utf8"

	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperexpr"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

type PaperDocumentProperty string

const (
	PaperDocumentTitle    PaperDocumentProperty = "title"
	PaperDocumentLanguage PaperDocumentProperty = "language"
	PaperDocumentTheme    PaperDocumentProperty = "theme"
)

type PaperSetDocumentPropertyRequest struct {
	Guard    PaperMutationGuard    `json:"guard"`
	Property PaperDocumentProperty `json:"property"`
	Text     string                `json:"text"`
}

func (w *Workspace) PaperSetDocumentProperty(request PaperSetDocumentPropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodeDocument {
		return PaperMutationResult{}, workspaceError("INVALID_DOCUMENT_TARGET", "document controls require an exact document source node", paperedit.ErrInvalidOperation)
	}
	if !utf8.ValidString(request.Text) || len(request.Text) > w.maxMutationPayloadBytes() {
		return PaperMutationResult{}, workspaceError("INVALID_DOCUMENT_VALUE", "document text must be bounded valid UTF-8", paperedit.ErrInvalidOperation)
	}
	value := request.Text
	switch request.Property {
	case PaperDocumentTitle:
	case PaperDocumentLanguage:
		value = strings.TrimSpace(value)
		if value == "" || strings.ContainsAny(value, " \t\r\n\x00") {
			return PaperMutationResult{}, workspaceError("INVALID_DOCUMENT_VALUE", "document language must be one readable language tag", paperedit.ErrInvalidOperation)
		}
	case PaperDocumentTheme:
		value = strings.TrimSpace(value)
		if !validAuthorityNodeID(value) {
			return PaperMutationResult{}, workspaceError("INVALID_DOCUMENT_VALUE", "document theme must be one exact @theme ID", paperedit.ErrInvalidOperation)
		}
	default:
		return PaperMutationResult{}, workspaceError("INVALID_DOCUMENT_PROPERTY", "document property is outside the supported editing vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: paperedit.StringValue(value)}
	return w.applyPaperMutation("set_document_property", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_DOCUMENT_PROPERTY_STATE")
}

type PaperPageNumberingProperty string

const (
	PaperPageNumbers      PaperPageNumberingProperty = "page-numbers"
	PaperPageNumberFormat PaperPageNumberingProperty = "page-number-format"
	PaperPageTotalAlias   PaperPageNumberingProperty = "page-total-alias"
	PaperPageNumberAlign  PaperPageNumberingProperty = "page-number-align"
	PaperPageNumberPlace  PaperPageNumberingProperty = "page-number-position"
	PaperPageNumberFirst  PaperPageNumberingProperty = "page-number-hide-first"
	PaperPageNumberStart  PaperPageNumberingProperty = "page-number-start"
)

type PaperSetPageNumberingRequest struct {
	Guard    PaperMutationGuard         `json:"guard"`
	Property PaperPageNumberingProperty `json:"property"`
	Text     string                     `json:"text,omitempty"`
	Kind     string                     `json:"kind,omitempty"`
	Bool     bool                       `json:"bool,omitempty"`
	Count    uint32                     `json:"count,omitempty"`
}

func (w *Workspace) PaperSetPageNumbering(request PaperSetPageNumberingRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodePage {
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_NUMBERING_TARGET", "page-number controls require an exact page source node", paperedit.ErrInvalidOperation)
	}
	allowedKind := func(value string, choices ...string) bool {
		for _, choice := range choices {
			if value == choice {
				return true
			}
		}
		return false
	}
	var value paperedit.Value
	switch request.Property {
	case PaperPageNumbers, PaperPageNumberFirst:
		if request.Text != "" || request.Kind != "" || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_PAGE_NUMBERING_VALUE", "page-numbers accepts only a boolean value", paperedit.ErrInvalidOperation)
		}
		value = paperedit.BoolValue(request.Bool)
	case PaperPageNumberFormat, PaperPageTotalAlias:
		if request.Bool || request.Kind != "" || request.Count != 0 || !utf8.ValidString(request.Text) || strings.TrimSpace(request.Text) == "" || len(request.Text) > w.maxMutationPayloadBytes() {
			return PaperMutationResult{}, workspaceError("INVALID_PAGE_NUMBERING_VALUE", "page-number text must be non-empty bounded valid UTF-8", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(request.Text)
	case PaperPageNumberAlign:
		if request.Text != "" || request.Bool || request.Count != 0 || !allowedKind(request.Kind, "left", "center", "right", "inner", "outer") {
			return PaperMutationResult{}, workspaceError("INVALID_PAGE_NUMBERING_VALUE", "page-number-align accepts left, center, right, inner, or outer", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(request.Kind)
	case PaperPageNumberPlace:
		if request.Text != "" || request.Bool || request.Count != 0 || !allowedKind(request.Kind, "header", "footer") {
			return PaperMutationResult{}, workspaceError("INVALID_PAGE_NUMBERING_VALUE", "page-number-position accepts header or footer", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(request.Kind)
	case PaperPageNumberStart:
		if request.Text != "" || request.Kind != "" || request.Bool || request.Count == 0 || request.Count > 1<<20 {
			return PaperMutationResult{}, workspaceError("INVALID_PAGE_NUMBERING_VALUE", "page-number-start accepts a whole number from 1 through 1048576", paperedit.ErrInvalidOperation)
		}
		value = paperedit.NumberValue(float64(request.Count))
	default:
		return PaperMutationResult{}, workspaceError("INVALID_PAGE_NUMBERING_PROPERTY", "page-number property is outside the supported editing vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: value}
	return w.applyPaperMutation("set_page_numbering", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_PAGE_NUMBERING_STATE")
}

type PaperCanvasProperty string

const (
	PaperCanvasWidth             PaperCanvasProperty = "width"
	PaperCanvasHeight            PaperCanvasProperty = "height"
	PaperCanvasDefaultHorizontal PaperCanvasProperty = "default-horizontal"
	PaperCanvasDefaultVertical   PaperCanvasProperty = "default-vertical"
)

type PaperSetCanvasPropertyRequest struct {
	Guard    PaperMutationGuard  `json:"guard"`
	Property PaperCanvasProperty `json:"property"`
	Points   float64             `json:"points,omitempty"`
	Kind     string              `json:"kind,omitempty"`
}

func (w *Workspace) PaperSetCanvasProperty(request PaperSetCanvasPropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || node.Kind != paperlang.NodeCanvas {
		return PaperMutationResult{}, workspaceError("INVALID_CANVAS_TARGET", "canvas controls require an exact canvas source node", paperedit.ErrInvalidOperation)
	}
	var value paperedit.Value
	switch request.Property {
	case PaperCanvasWidth, PaperCanvasHeight:
		if request.Kind != "" || !finiteLayoutHandle(request.Points) || request.Points <= 0 || request.Points > 1_000_000 {
			return PaperMutationResult{}, workspaceError("INVALID_CANVAS_VALUE", "canvas dimensions must be finite positive point values", paperedit.ErrInvalidOperation)
		}
		value = paperedit.UnitValue(request.Points, "pt")
	case PaperCanvasDefaultHorizontal:
		kind := strings.ToLower(strings.TrimSpace(request.Kind))
		if request.Points != 0 || !layoutChoice(kind, "left", "right", "center-x") {
			return PaperMutationResult{}, workspaceError("INVALID_CANVAS_VALUE", "horizontal default must be left, right, or center-x", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(kind)
	case PaperCanvasDefaultVertical:
		kind := strings.ToLower(strings.TrimSpace(request.Kind))
		if request.Points != 0 || !layoutChoice(kind, "top", "bottom", "center-y") {
			return PaperMutationResult{}, workspaceError("INVALID_CANVAS_VALUE", "vertical default must be top, bottom, or center-y", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(kind)
	default:
		return PaperMutationResult{}, workspaceError("INVALID_CANVAS_PROPERTY", "canvas property is outside the supported editing vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: value}
	return w.applyPaperMutation("set_canvas_property", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_CANVAS_PROPERTY_STATE")
}

type PaperAppearanceProperty string

const (
	PaperAppearanceStyle      PaperAppearanceProperty = "style"
	PaperAppearanceFontToken  PaperAppearanceProperty = "font-token"
	PaperAppearanceSizeToken  PaperAppearanceProperty = "size-token"
	PaperAppearanceLineToken  PaperAppearanceProperty = "line-height-token"
	PaperAppearanceColorToken PaperAppearanceProperty = "color-token"
)

type PaperSetAppearanceRequest struct {
	Guard    PaperMutationGuard      `json:"guard"`
	Property PaperAppearanceProperty `json:"property"`
	Text     string                  `json:"text"`
}

func (w *Workspace) PaperSetAppearance(request PaperSetAppearanceRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || !appearanceNode(node.Kind) {
		return PaperMutationResult{}, workspaceError("INVALID_APPEARANCE_TARGET", "appearance controls require an authored text block, list, image, or table cell", paperedit.ErrInvalidOperation)
	}
	text := strings.TrimSpace(request.Text)
	if !utf8.ValidString(text) || text == "" || len(text) > w.maxMutationPayloadBytes() || strings.ContainsAny(text, " \t\r\n\x00") {
		return PaperMutationResult{}, workspaceError("INVALID_APPEARANCE_VALUE", "style and token references must be one bounded readable name", paperedit.ErrInvalidOperation)
	}
	if request.Property == PaperAppearanceStyle {
		if !validAuthorityNodeID(text) {
			return PaperMutationResult{}, workspaceError("INVALID_APPEARANCE_VALUE", "style must reference one exact @style ID", paperedit.ErrInvalidOperation)
		}
	} else if node.Kind == paperlang.NodeImage {
		return PaperMutationResult{}, workspaceError("INVALID_APPEARANCE_PROPERTY", "images accept a named style but not text theme tokens", paperedit.ErrInvalidOperation)
	} else if !layoutChoice(string(request.Property), "font-token", "size-token", "line-height-token", "color-token") {
		return PaperMutationResult{}, workspaceError("INVALID_APPEARANCE_PROPERTY", "appearance property is outside the supported editing vocabulary", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: paperedit.StringValue(text)}
	return w.applyPaperMutation("set_appearance", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_APPEARANCE_STATE")
}

func appearanceNode(kind paperlang.NodeKind) bool {
	return kind == paperlang.NodeParagraph || kind == paperlang.NodeHeading || kind == paperlang.NodeList || kind == paperlang.NodeImage || kind == paperlang.NodeTableCell
}

type PaperSetConditionRequest struct {
	Guard      PaperMutationGuard `json:"guard"`
	Expression string             `json:"expression"`
}

func (w *Workspace) PaperSetCondition(request PaperSetConditionRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || !conditionNode(node.Kind) {
		return PaperMutationResult{}, workspaceError("INVALID_CONDITION_TARGET", "conditions require a paragraph, heading, list, row, column, image, or table", paperedit.ErrInvalidOperation)
	}
	expression := strings.TrimSpace(request.Expression)
	if expression == "" || !utf8.ValidString(expression) || len(expression) > w.maxMutationPayloadBytes() || strings.IndexByte(expression, 0) >= 0 {
		return PaperMutationResult{}, workspaceError("INVALID_CONDITION_VALUE", "when must be a non-empty bounded UTF-8 expression", paperedit.ErrInvalidOperation)
	}
	if _, err := paperexpr.Parse(expression, paperexpr.DefaultLanguageLimits()); err != nil {
		return PaperMutationResult{}, workspaceError("INVALID_CONDITION_VALUE", "when must use valid bounded expression syntax", paperedit.ErrInvalidOperation)
	}
	operation := paperedit.SetProperty{Target: request.Guard.Target, Name: "when", Value: paperedit.StringValue(expression)}
	return w.applyPaperMutation("set_condition", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_CONDITION_STATE")
}

func conditionNode(kind paperlang.NodeKind) bool {
	switch kind {
	case paperlang.NodeParagraph, paperlang.NodeHeading, paperlang.NodeList, paperlang.NodeRow, paperlang.NodeColumn, paperlang.NodeImage, paperlang.NodeTable:
		return true
	default:
		return false
	}
}

type PaperResetPropertyRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	Category string             `json:"category"`
	Property string             `json:"property"`
}

// PaperResetProperty removes one explicit override so normal inheritance or
// the language default becomes visible again. The category and node-kind
// matrix is closed for the same reason as the typed setters: a caller cannot
// turn this into an arbitrary source deletion primitive.
func (w *Workspace) PaperResetProperty(request PaperResetPropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || !resetPropertyAllowed(node.Kind, request.Category, request.Property) {
		return PaperMutationResult{}, workspaceError("INVALID_RESET_PROPERTY", "property cannot be reset on this authored node", paperedit.ErrInvalidOperation)
	}
	if request.Category == "binding" {
		operations := make([]paperedit.Operation, 0, 7)
		for _, name := range []string{"bind", "bind-required", "format", "format-locale", "format-currency", "format-min-fraction", "format-max-fraction"} {
			for _, member := range node.Members {
				if member.Property != nil && member.Property.Name == name {
					operations = append(operations, paperedit.DeleteProperty{Target: request.Guard.Target, Name: name})
					break
				}
			}
		}
		return w.applyPaperMutation("reset_property", request.Guard, opened, revision, []string{request.Guard.Target}, operations, "INVALID_RESET_PROPERTY_STATE")
	}
	return w.applyPaperMutation("reset_property", request.Guard, opened, revision, []string{request.Guard.Target}, []paperedit.Operation{
		paperedit.DeleteProperty{Target: request.Guard.Target, Name: request.Property},
	}, "INVALID_RESET_PROPERTY_STATE")
}

func resetPropertyAllowed(kind paperlang.NodeKind, category, property string) bool {
	allowed := func(values ...string) bool { return layoutChoice(property, values...) }
	switch category {
	case "document":
		return kind == paperlang.NodeDocument && allowed("title", "language", "theme")
	case "appearance":
		if !appearanceNode(kind) {
			return false
		}
		if kind == paperlang.NodeImage {
			return property == "style"
		}
		return allowed("style", "font-token", "size-token", "line-height-token", "color-token")
	case "condition":
		return conditionNode(kind) && property == "when"
	case "text":
		if kind != paperlang.NodeParagraph && kind != paperlang.NodeHeading && kind != paperlang.NodeList && kind != paperlang.NodeTableCell {
			return false
		}
		return allowed("font", "size", "line-height", "color", "align", "bold", "italic") || kind == paperlang.NodeHeading && property == "level"
	case "list":
		return kind == paperlang.NodeList && allowed("ordered", "marker")
	case "box", "region":
		boxKind := kind == paperlang.NodeParagraph || kind == paperlang.NodeHeading || kind == paperlang.NodeList || kind == paperlang.NodeImage || kind == paperlang.NodeTableCell || kind == paperlang.NodeAnchor
		return (category == "region" && (kind == paperlang.NodeHeader || kind == paperlang.NodeFooter) || category == "box" && boxKind) && allowed(
			"margin", "margin-top", "margin-right", "margin-bottom", "margin-left", "padding", "padding-top", "padding-right", "padding-bottom", "padding-left",
			"border-width", "border-top-width", "border-right-width", "border-bottom-width", "border-left-width", "border-radius", "border-color", "background")
	case "layout-item":
		itemKind := kind == paperlang.NodeParagraph || kind == paperlang.NodeHeading || kind == paperlang.NodeImage || kind == paperlang.NodeTable || kind == paperlang.NodeRow || kind == paperlang.NodeColumn
		return itemKind && allowed("width", "min-width", "max-width", "height", "min-height", "max-height", "flex-grow", "flex-shrink", "align-self")
	case "layout-container":
		return (kind == paperlang.NodeRow || kind == paperlang.NodeColumn) && allowed("gap", "line-gap", "width", "height", "wrap", "justify-content", "align-items", "align-content", "reverse")
	case "image":
		return kind == paperlang.NodeImage && allowed("fit", "focus-x", "focus-y", "width", "height", "max-width", "max-height", "align", "caption", "alt", "decorative")
	case "table":
		switch kind {
		case paperlang.NodeTable:
			return allowed("caption", "split", "repeat-header")
		case paperlang.NodeTableColumn:
			return allowed("width", "min-width", "max-width")
		case paperlang.NodeTableRow:
			return allowed("keep-together", "keep-with-next", "orphans", "widows")
		case paperlang.NodeTableCell:
			return allowed("header-cell", "vertical-align", "colspan", "rowspan")
		}
	case "page":
		return kind == paperlang.NodePage && allowed("margin", "margin-top", "margin-right", "margin-bottom", "margin-left", "page-numbers", "page-number-format", "page-total-alias", "page-number-align", "page-number-position", "page-number-hide-first", "page-number-start")
	case "canvas":
		return kind == paperlang.NodeAnchor && allowed("left", "right", "center-x", "top", "bottom", "center-y", "width", "height", "alt")
	case "canvas-container":
		return kind == paperlang.NodeCanvas && allowed("width", "height", "default-horizontal", "default-vertical")
	case "binding":
		return property == "bind" && (kind == paperlang.NodeParagraph || kind == paperlang.NodeHeading || kind == paperlang.NodeUse || kind == paperlang.NodeTableCell)
	}
	return false
}
