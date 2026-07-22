// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"

	"github.com/cssbruno/paperrune/internal/papercompile"
	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

type PaperInsertTemplateRequest struct {
	Guard      PaperMutationGuard `json:"guard"`
	Template   string             `json:"template"`
	ID         string             `json:"id"`
	Component  string             `json:"component,omitempty"`
	ImportPath string             `json:"import_path,omitempty"`
	Preset     string             `json:"preset,omitempty"`
	Path       string             `json:"path,omitempty"`
}

// PaperInsertTemplate inserts one closed, typed starter shape beneath an
// exact layout container. The edit renders one minimal CST insertion.
func (w *Workspace) PaperInsertTemplate(request PaperInsertTemplateRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	parent := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if parent == nil || !authoringTemplateAllowed(parent.Kind, request.Template) {
		return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_PARENT", "template is not valid beneath the selected node", paperedit.ErrInvalidOperation)
	}
	if request.Template == "import" {
		if parent.Kind != paperlang.NodeDocument || !safeAuthoringImportPath(request.ImportPath) {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE", "import template requires a safe relative .paper path on the document", ErrInvalidQuery)
		}
		return w.applyPaperMutation("insert_template", request.Guard, opened, revision,
			[]string{request.Guard.Target}, []paperedit.Operation{paperedit.AppendProperty{Target: request.Guard.Target, Name: "import", Value: paperedit.StringValue(request.ImportPath)}}, "INVALID_TEMPLATE_RESULT")
	}
	if !validAuthorityNodeID(request.ID) {
		return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_ID", "template requires a bounded readable @id", ErrInvalidQuery)
	}
	var node paperedit.NodeSpec
	switch request.Template {
	case "schema":
		if parent.Kind != paperlang.NodeDocument {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_PARENT", "schema template must target the document", paperedit.ErrInvalidOperation)
		}
		base := strings.TrimPrefix(request.ID, "@")
		if len(base) > 110 {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_ID", "schema ID is too long for its starter field ID", ErrInvalidQuery)
		}
		node = paperedit.NodeSpec{Kind: paperlang.NodeSchema, ID: request.ID, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeField, ID: "@" + base + "-value", FieldType: paperlang.FieldString},
		}}
	case "schema-object":
		if parent.Kind != paperlang.NodeDocument {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_PARENT", "custom object template must target the document", paperedit.ErrInvalidOperation)
		}
		base := strings.TrimPrefix(request.ID, "@")
		if len(base) > 110 {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_ID", "custom object ID is too long for its starter field ID", ErrInvalidQuery)
		}
		node = paperedit.NodeSpec{Kind: paperlang.NodeObjectType, ID: request.ID, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeField, ID: "@" + base + "-value", FieldType: paperlang.FieldString},
		}}
	case "page", "document-preset":
		if parent.Kind != paperlang.NodeDocument {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_PARENT", "page template must target the document", paperedit.ErrInvalidOperation)
		}
		for _, member := range parent.Members {
			if member.Node != nil && member.Node.Kind == paperlang.NodePage {
				return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE", "page template is only available before the document has a page", paperedit.ErrInvalidOperation)
			}
		}
		base := strings.TrimPrefix(request.ID, "@")
		if len(base) > 220 {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_ID", "template ID is too long for derived readable child IDs", ErrInvalidQuery)
		}
		if request.Template == "document-preset" {
			node, err = authoringDocumentPreset(request.Preset, request.ID)
			if err != nil {
				return PaperMutationResult{}, err
			}
		} else {
			node = paperedit.NodeSpec{Kind: paperlang.NodePage, ID: request.ID, Children: []paperedit.NodeSpec{
				{Kind: paperlang.NodeBody, ID: "@" + base + "-body", Children: []paperedit.NodeSpec{
					{Kind: paperlang.NodeParagraph, ID: "@" + base + "-copy", Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("New content")}}},
				}},
			}}
		}
	case "paragraph":
		node = paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: request.ID, Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("New content")}}}
	case "text":
		value := paperedit.StringValue("New text")
		node = paperedit.NodeSpec{Kind: paperlang.NodeText, ID: request.ID, Value: &value}
	case "heading":
		node = paperedit.NodeSpec{Kind: paperlang.NodeHeading, ID: request.ID, Properties: []paperedit.PropertySpec{
			{Name: "level", Value: paperedit.NumberValue(2)},
			{Name: "text", Value: paperedit.StringValue("New heading")},
		}}
	case "list":
		value := paperedit.StringValue("New item")
		node = paperedit.NodeSpec{Kind: paperlang.NodeList, ID: request.ID, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeItem, Children: []paperedit.NodeSpec{{Kind: paperlang.NodeText, Value: &value}}},
		}}
	case "item":
		base := strings.TrimPrefix(request.ID, "@")
		value := paperedit.StringValue("New item")
		node = paperedit.NodeSpec{Kind: paperlang.NodeItem, ID: request.ID, Children: []paperedit.NodeSpec{{Kind: paperlang.NodeText, ID: "@" + base + "-text", Value: &value}}}
	case "table-row":
		base := strings.TrimPrefix(request.ID, "@")
		cell := func(suffix, value string) paperedit.NodeSpec {
			return paperedit.NodeSpec{Kind: paperlang.NodeTableCell, ID: "@" + base + "-" + suffix, Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue(value)}}}
		}
		node = paperedit.NodeSpec{Kind: paperlang.NodeTableRow, ID: request.ID, Children: []paperedit.NodeSpec{cell("cell-one", "New cell"), cell("cell-two", "New cell")}}
	case "cell":
		node = paperedit.NodeSpec{Kind: paperlang.NodeTableCell, ID: request.ID, Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("New cell")}}}
	case "row", "column":
		base := strings.TrimPrefix(request.ID, "@")
		if len(base) > 220 {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_ID", "template ID is too long for derived readable child IDs", ErrInvalidQuery)
		}
		kind := paperlang.NodeRow
		if request.Template == "column" {
			kind = paperlang.NodeColumn
		}
		node = paperedit.NodeSpec{Kind: kind, ID: request.ID, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeParagraph, ID: "@" + base + "-copy", Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("New content")}}},
		}}
	case "page-break":
		node = paperedit.NodeSpec{Kind: paperlang.NodePageBreak, ID: request.ID}
	case "component":
		if !validAuthorityNodeID(request.Component) {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_COMPONENT", "component template requires a readable component reference", ErrInvalidQuery)
		}
		if _, err := uniqueComponentDefinition(revision.parsed.AST.Root, request.Component); err != nil {
			return PaperMutationResult{}, err
		}
		node = paperedit.NodeSpec{Kind: paperlang.NodeUse, ID: request.ID, Properties: []paperedit.PropertySpec{{Name: "component", Value: paperedit.ExpressionValue(request.Component)}}}
	case "section":
		base := strings.TrimPrefix(request.ID, "@")
		if len(base) > 220 {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_ID", "template ID is too long for derived readable child IDs", ErrInvalidQuery)
		}
		node = paperedit.NodeSpec{Kind: paperlang.NodeColumn, ID: request.ID, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeHeading, ID: "@" + base + "-heading", Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("Section heading")}}},
			{Kind: paperlang.NodeParagraph, ID: "@" + base + "-body", Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("New content")}}},
		}}
	case "image", "table", "canvas", "note-box", "metadata-grid", "signature-row", "qr-verification", "clause",
		"title-block", "two-column", "image-caption", "quote", "checklist", "disclaimer", "divider",
		"cover-block", "recipient-block", "code-block", "status-banner", "numbered-steps", "timeline", "comparison-table", "approval-block", "image-grid", "invoice-totals",
		"kpi-strip", "table-of-contents", "risk-register", "change-log", "decision-record", "pros-cons", "faq-block":
		if parent.Kind == paperlang.NodeDocument || parent.Kind == paperlang.NodePage {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_PARENT", "flow templates require a region, body, row, or column", paperedit.ErrInvalidOperation)
		}
		node, err = authoringFlowTemplate(request.Template, request.ID)
		if err != nil {
			return PaperMutationResult{}, err
		}
	case "repeat", "loop":
		if parent.Kind == paperlang.NodeDocument || parent.Kind == paperlang.NodePage {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_PARENT", "repeat and loop templates require a flow container", paperedit.ErrInvalidOperation)
		}
		node, err = authoringRepeaterTemplate(revision.parsed.AST, request.Template, request.ID, request.Path)
		if err != nil {
			return PaperMutationResult{}, err
		}
	case "header", "footer":
		if parent.Kind != paperlang.NodePage {
			return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE_PARENT", "header and footer templates must target a page", paperedit.ErrInvalidOperation)
		}
		kind := paperlang.NodeHeader
		if request.Template == "footer" {
			kind = paperlang.NodeFooter
		}
		for _, member := range parent.Members {
			if member.Node != nil && member.Node.Kind == kind {
				return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE", request.Template+" already exists on this page", paperedit.ErrInvalidOperation)
			}
		}
		base := strings.TrimPrefix(request.ID, "@")
		node = paperedit.NodeSpec{Kind: kind, ID: request.ID, Children: []paperedit.NodeSpec{{Kind: paperlang.NodeParagraph, ID: "@" + base + "-copy", Properties: []paperedit.PropertySpec{{Name: "size", Value: paperedit.UnitValue(8, "pt")}, {Name: "text", Value: paperedit.StringValue(strings.ToUpper(request.Template) + " | Document title | Page")}}}}}
	default:
		return PaperMutationResult{}, workspaceError("INVALID_TEMPLATE", "template is outside the closed authoring palette", ErrInvalidQuery)
	}
	return w.applyPaperMutation("insert_template", request.Guard, opened, revision,
		[]string{request.Guard.Target}, []paperedit.Operation{paperedit.InsertNode{Parent: request.Guard.Target, Node: node}}, "INVALID_TEMPLATE_RESULT")
}

// authoringTemplateAllowed mirrors the grammar's parent/child contract at the
// template level. Keeping this closed prevents Studio from offering mutations
// that can only fail during compilation.
func authoringTemplateAllowed(parent paperlang.NodeKind, template string) bool {
	for _, candidate := range AuthoringTemplateChoices(parent) {
		if template == candidate {
			return true
		}
	}
	return false
}

// AuthoringTemplateChoices returns the closed built-in palette accepted by a
// source parent. Callers receive a copy so the backend remains the single
// immutable owner of insertion compatibility.
func AuthoringTemplateChoices(parent paperlang.NodeKind) []string {
	var choices []string
	switch parent {
	case paperlang.NodeDocument:
		choices = []string{"import", "schema", "schema-object", "page", "document-preset"}
	case paperlang.NodePage:
		choices = []string{"header", "footer"}
	case paperlang.NodeBody, paperlang.NodeHeader, paperlang.NodeFooter:
		choices = []string{
			"paragraph", "heading", "list", "row", "column",
		}
		if parent == paperlang.NodeBody {
			choices = append(choices, "page-break")
		}
		choices = append(choices,
			"component", "section", "image", "table", "canvas",
			"note-box", "metadata-grid", "signature-row", "qr-verification", "clause", "title-block", "two-column", "image-caption",
			"quote", "checklist", "disclaimer", "divider", "cover-block", "recipient-block", "code-block", "status-banner", "numbered-steps",
			"timeline", "comparison-table", "approval-block", "image-grid", "invoice-totals", "kpi-strip", "table-of-contents",
			"risk-register", "change-log", "decision-record", "pros-cons", "faq-block", "repeat", "loop",
		)
	case paperlang.NodeRow, paperlang.NodeColumn:
		choices = []string{
			"paragraph", "heading", "row", "column", "component", "section", "image", "table", "note-box", "metadata-grid", "signature-row",
			"qr-verification", "clause", "title-block", "two-column", "image-caption", "quote", "checklist", "disclaimer", "divider",
			"recipient-block", "code-block", "status-banner", "numbered-steps", "timeline", "comparison-table", "approval-block", "invoice-totals",
			"kpi-strip", "table-of-contents", "risk-register", "change-log", "decision-record", "pros-cons", "faq-block",
		}
	case paperlang.NodeList:
		choices = []string{"item"}
	case paperlang.NodeItem:
		choices = []string{"text", "paragraph", "component"}
	case paperlang.NodeTable, paperlang.NodeTableHeader:
		choices = []string{"table-row"}
	case paperlang.NodeTableRow:
		choices = []string{"cell"}
	case paperlang.NodeTableCell:
		choices = []string{"text", "paragraph", "image", "list"}
	}
	return append([]string(nil), choices...)
}

func authoringRepeaterTemplate(ast paperlang.AST, template, id, sourcePath string) (paperedit.NodeSpec, error) {
	base := strings.TrimPrefix(id, "@")
	paragraph := paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: "@" + base + "-item", Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("Repeated item")}}}
	if template == "loop" {
		return paperedit.NodeSpec{Kind: paperlang.NodeLoop, ID: id, Properties: []paperedit.PropertySpec{
			{Name: "from", Value: paperedit.NumberValue(1)}, {Name: "through", Value: paperedit.NumberValue(3)}, {Name: "step", Value: paperedit.NumberValue(1)}, {Name: "max-iterations", Value: paperedit.NumberValue(3)}, {Name: "instance-prefix", Value: paperedit.StringValue(base)},
		}, Children: []paperedit.NodeSpec{paragraph}}, nil
	}
	sourcePath = strings.TrimSpace(sourcePath)
	metadata := papercompile.ExtractSchemas(ast)
	if !metadata.OK() {
		return paperedit.NodeSpec{}, workspaceError("INVALID_TEMPLATE", "repeat requires valid compiler schema metadata", ErrInvalidSource)
	}
	var selected *papercompile.FieldDescriptor
	for schemaIndex := range metadata.Schemas {
		schema := &metadata.Schemas[schemaIndex]
		path := sourcePath
		if len(metadata.Schemas) > 1 {
			prefix := strings.TrimPrefix(schema.Name, "@") + "."
			if !strings.HasPrefix(sourcePath, prefix) {
				continue
			}
			path = strings.TrimPrefix(sourcePath, prefix)
		}
		parts := strings.Split(path, ".")
		fields := schema.Fields
		for index, part := range parts {
			selected = nil
			for fieldIndex := range fields {
				if fields[fieldIndex].Name == strings.TrimPrefix(part, "@") {
					selected = &fields[fieldIndex]
					break
				}
			}
			if selected == nil || index < len(parts)-1 && selected.Kind != papercompile.SchemaObject {
				selected = nil
				break
			}
			fields = selected.Fields
		}
		break
	}
	if selected == nil || selected.Kind != papercompile.SchemaList || selected.MaxItems == 0 {
		return paperedit.NodeSpec{}, workspaceError("INVALID_TEMPLATE", "repeat source must address one bounded schema list", ErrInvalidQuery)
	}
	if selected.ItemKind == papercompile.SchemaObject && len(selected.Fields) != 0 {
		paragraph.Properties = append([]paperedit.PropertySpec{{Name: "bind", Value: paperedit.StringValue(selected.Fields[0].Name)}}, paragraph.Properties...)
	}
	return paperedit.NodeSpec{Kind: paperlang.NodeRepeat, ID: id, Properties: []paperedit.PropertySpec{
		{Name: "source", Value: paperedit.StringValue(sourcePath)}, {Name: "instance-prefix", Value: paperedit.StringValue(base)}, {Name: "max-items", Value: paperedit.NumberValue(float64(selected.MaxItems))},
	}, Children: []paperedit.NodeSpec{paragraph}}, nil
}

const authoringPlaceholderDataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAUAAAAC0CAIAAABqhmJGAAAACXBIWXMAAAABAAAAAQBPJcTWAAACs0lEQVR4nO3aLUooUQCGYQVXIUaL2rTaBIsLuHsQ7CJYxRXcPdwFWASbVZvcYhTXYbH7g845rz5PHjhfeTkMM6vbJ+crb7m5OH3zGWB5a6MHAJ8nYAgTMIQJGMIEDGEChjABQ5iAIUzAECZgCBMwhAkYwgQMYQKGMAFDmIAhTMAQJmAIEzCECRjCBAxhAoYwAUOYgCFMwBAmYAgTMIQJGMIEDGEChjABQ5iAIUzAECZgCBMwhAkYwgQMYQKGMAFDmIAhTMAQNiDgv7d3yx+6mOP9vdET+EXcwBAmYAgTMIQJGMIEDGECnsjB2eXoCbzLzcXp6AmvBAxhAoYwAUOYgCFMwBAmYAgTMIQJGMIEDGEChjABQ5iAJzLPH7ZUCBjCBAxhAoYwAUOYgCFMwBAmYAgTMIQJGMIEDGEChjABT+Tf1fXoCT/Kn6PD0RO+nYAhTMAQJmAIEzCECRjCBAxhAoYwAU/kN3y35GsJGMIEDGEChjABQ5iAIUzAECZgCBMwhAkYwgQ8kfuH/6MnTGd3Z2v0hKkJGMIEDGEChjABQ5iAIUzAECZgCBPwRHzz5KMEDGEChjABQ5iAIUzAECZgCBMwhAkYwgQMYQKGMAFDmIAhTMAQJmAIEzCECRjCBAxhAwI+3t9b/lD4kdzAECZgCBMwhAkYwgQMYQKGMAFDmIAhTMAQJmAIEzCECRjCBAxhAoYwAUOYgCFMwBAmYAgTMIQNCPjx6Xn5Q+ELbW6sj57wyg0MYQKGMAFD2ICA53l/gDo3MIQJGMIEDGEChjABQ5iAIUzAECZgCBMwhAkYwgQMYQKGMAFDmIAhTMAQJmAIEzCECRjCBAxhAoYwAUOYgCFMwBAmYAgTMIQJGMIEDGEChjABQ5iAIUzAECZgCBMwhAkYwgQMYQKGMAFDmIAhTMAQ9gLP9hj5jXcS6AAAAABJRU5ErkJggg=="

const authoringQRPlaceholderDataURI = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAANIAAADSCAIAAACw+wkVAAAACXBIWXMAAAABAAAAAQBPJcTWAAAD0ElEQVR4nO3dMY7dRACAYVhxB4qcgSZtRBtxHI7AETgOSovSpuEMKTjEriigQApZbL/x+8dvvq9+K3nXf8bz7PHk2+fn52/gvr6rD4AVyY6A7AjIjoDsCMiOgOwIyI6A7AjIjoDsCGzK7vsffzr7OG705++/1YdwJ49xLox2BGRHQHYEFs3uh59/+d/P/PHra58ZNcdaZ1b6b4tmR0t2BGRHQHYEZEdgQHb3+S42/935GVzlXBjtCMiOgOwIyI6A7Agsmt3rz1u3WPNZ6iiLZkfrxOw+Pz0d+Kk3Ly/Dj4TZzsW5o93bd+93ff7Txw8nHQlTnQsXWQKyIyA7ArIjIDsCsiMgOwKyI7BQdsfWxHr2eoaFsmMeshtmy04DX7p9LcwVyY6A7AjIjoDsCJybnfVz85jqXJyYnXXC85jtXLjIEpAdAdkRWCg7T1fnsVB2zEN2BGRHYEB29tmcx1XOhdGOgOwIyI6A7IZZc53wMbIjIDsCsiMgOwKyI7ApO2s35vEY58JoR0B2BGRHQHYEZEdAdgRkR0B2BGRHQHYEZEdgoezO26n92K7Fo2xZ1TzqjbJRT4QXyo55yI6A7Ajszm7UPOb1GcneuchjrEJbh9GOgOwIyI6A7AjIjoDsCMiOgOwGsNfTXrIjIDsCsiOwOzvzGG5ntCOwUHYrr1KZ7XdfKDvmITsCsiMgOwKyIyA7ArIjILsBtr/nNtv9s4rsCMiOgOwIyG46Z+wfdd6uUMdmq7IjsFx2x8YSqwzHWi47ZjBpdu5vPbZJs+OxyY6A7AjIjoDsCMiOgOwIyI6A7AjIbjorPP+VHQHZEZAdAdkRkB2B5bJb4Xvi/JbLjhnIjoDsCMiOgOwIyI6A7AjIjn/c891k2RGQHQHZEZAdAdkRkN0A9qfaS3YENmV3bFfbezLeXIvRjoDsLqy6Ct1+bZEdAdkRkB0B2Z3O/4TxJdkRWCi7M773uV94zELZMY8B2d3nX/z8T0rYzmhHQHYEZEdAdgRkR0B2F3bdu4ayI3Bidp+fng781JuXl+FHwmzOHe3evnu/6/OfPn446UiYykIX2evOhB7PQtkxD9kRkB0B2Z3usdcJHyM7ArIjIDsCsiMgOwKyW9Sxt3f/dvt3c9kRkB0B2RFYNLu9M5vts5n2fd6rrLI5Nzvr5/hPJ2ZnnTBfs+hFlpbsCMiOgOwIyI6A7AjIjsCA7OyzyV5GOwKyG+wqT0VbsiMgOwKyI7Bodt7Ub/8Ci2ZHS3YEZEdgU3buRTGW0Y6A7AjIjoDsCMiOgOwIyI6A7AjIjoDsCMiOwF/vhqJM2D7eAgAAAABJRU5ErkJggg=="

const (
	authoringInk          = "#163A46"
	authoringAccent       = "#2C6E7F"
	authoringAccentStrong = "#1F5967"
	authoringAccentSoft   = "#EAF4F7"
	authoringText         = "#334155"
	authoringMuted        = "#64748B"
	authoringBorder       = "#D5E2E6"
	authoringSurface      = "#F7FAFB"
	authoringSuccess      = "#177245"
	authoringSuccessSoft  = "#E8F7EF"
	authoringWarning      = "#9A6700"
	authoringWarningSoft  = "#FFF4CF"
	authoringDanger       = "#A53A3A"
	authoringDangerSoft   = "#FCEBEC"
)

func authoringFlowTemplate(template, id string) (paperedit.NodeSpec, error) {
	base := strings.TrimPrefix(id, "@")
	childID := func(suffix string) string { return "@" + base + "-" + suffix }
	mergeProperties := func(baseProperties, overrides []paperedit.PropertySpec) []paperedit.PropertySpec {
		merged := append([]paperedit.PropertySpec(nil), baseProperties...)
		for _, override := range overrides {
			replaced := false
			for index := range merged {
				if merged[index].Name == override.Name {
					merged[index] = override
					replaced = true
					break
				}
			}
			if !replaced {
				merged = append(merged, override)
			}
		}
		return merged
	}
	cell := func(suffix, value string, header bool) paperedit.NodeSpec {
		properties := []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue(value)}, {Name: "size", Value: paperedit.UnitValue(8.5, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(11, "pt")}, {Name: "padding", Value: paperedit.UnitValue(6, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringText)}, {Name: "border-width", Value: paperedit.UnitValue(0.5, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringBorder)}}
		if header {
			properties = mergeProperties(properties, []paperedit.PropertySpec{
				{Name: "bold", Value: paperedit.BoolValue(true)},
				{Name: "color", Value: paperedit.StringValue("#FFFFFF")},
				{Name: "background", Value: paperedit.StringValue(authoringAccentStrong)},
			})
		}
		return paperedit.NodeSpec{Kind: paperlang.NodeTableCell, ID: childID(suffix), Properties: properties}
	}
	plainCell := func(suffix, value string, properties ...paperedit.PropertySpec) paperedit.NodeSpec {
		baseProperties := []paperedit.PropertySpec{
			{Name: "text", Value: paperedit.StringValue(value)},
			{Name: "size", Value: paperedit.UnitValue(9, "pt")},
			{Name: "line-height", Value: paperedit.UnitValue(12, "pt")},
		}
		return paperedit.NodeSpec{Kind: paperlang.NodeTableCell, ID: childID(suffix), Properties: mergeProperties(baseProperties, properties)}
	}
	dataCell := func(suffix, value string, properties ...paperedit.PropertySpec) paperedit.NodeSpec {
		baseProperties := []paperedit.PropertySpec{
			{Name: "text", Value: paperedit.StringValue(value)},
			{Name: "size", Value: paperedit.UnitValue(9, "pt")},
			{Name: "line-height", Value: paperedit.UnitValue(12, "pt")},
			{Name: "padding", Value: paperedit.UnitValue(6, "pt")},
			{Name: "color", Value: paperedit.StringValue(authoringText)},
			{Name: "border-width", Value: paperedit.UnitValue(0.5, "pt")},
			{Name: "border-color", Value: paperedit.StringValue(authoringBorder)},
		}
		return paperedit.NodeSpec{Kind: paperlang.NodeTableCell, ID: childID(suffix), Properties: mergeProperties(baseProperties, properties)}
	}
	column := func(suffix string, width float64) paperedit.NodeSpec {
		return paperedit.NodeSpec{Kind: paperlang.NodeTableColumn, ID: childID(suffix), Properties: []paperedit.PropertySpec{{Name: "width", Value: paperedit.UnitValue(width, "%")}}}
	}
	switch template {
	case "image":
		return paperedit.NodeSpec{Kind: paperlang.NodeImage, ID: id, Properties: []paperedit.PropertySpec{
			{Name: "source", Value: paperedit.StringValue(authoringPlaceholderDataURI)}, {Name: "width", Value: paperedit.UnitValue(160, "pt")}, {Name: "height", Value: paperedit.UnitValue(90, "pt")}, {Name: "fit", Value: paperedit.StringValue("cover")}, {Name: "alt", Value: paperedit.StringValue("Replace with a descriptive image")},
		}}, nil
	case "table", "metadata-grid", "signature-row", "timeline", "comparison-table", "approval-block", "invoice-totals", "checklist", "numbered-steps", "kpi-strip", "table-of-contents", "risk-register", "change-log", "pros-cons":
		rows := []paperedit.NodeSpec{}
		columns := []paperedit.NodeSpec{}
		switch template {
		case "table":
			columns = []paperedit.NodeSpec{column("item-column", 36), column("details-column", 64)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableHeader, ID: childID("head"), Children: []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("head-row"), Children: []paperedit.NodeSpec{cell("head-one", "DELIVERABLE", true), cell("head-two", "STATUS", true)}}}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{dataCell("one-a", "Clinic onboarding", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("one-b", "Ready for review")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{dataCell("two-a", "Data migration", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("two-b", "Blocked by vendor")}},
			}
		case "metadata-grid":
			columns = []paperedit.NodeSpec{column("label-column", 30), column("value-column", 70)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{dataCell("label-one", "REFERENCE", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(7.5, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("value-one", "OPS-042", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)})}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{dataCell("label-two", "ISSUED", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(7.5, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("value-two", "20 Jul 2026 | Marina Costa")}},
			}
		case "signature-row":
			columns = []paperedit.NodeSpec{column("signature-column", 62), column("date-column", 38)}
			rows = []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("row"), Children: []paperedit.NodeSpec{
				plainCell("signer", "Marina Costa | Program lead", paperedit.PropertySpec{Name: "padding-top", Value: paperedit.UnitValue(12, "pt")}, paperedit.PropertySpec{Name: "padding-right", Value: paperedit.UnitValue(16, "pt")}, paperedit.PropertySpec{Name: "border-top-width", Value: paperedit.UnitValue(0.75, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringMuted)}),
				plainCell("date", "20 Jul 2026", paperedit.PropertySpec{Name: "padding-top", Value: paperedit.UnitValue(12, "pt")}, paperedit.PropertySpec{Name: "padding-left", Value: paperedit.UnitValue(16, "pt")}, paperedit.PropertySpec{Name: "border-top-width", Value: paperedit.UnitValue(0.75, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringMuted)}),
			}}}
		case "timeline":
			columns = []paperedit.NodeSpec{column("date-column", 24), column("milestone-column", 76)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableHeader, ID: childID("head"), Children: []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("head-row"), Children: []paperedit.NodeSpec{cell("head-date", "DATE", true), cell("head-event", "MILESTONE", true)}}}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{dataCell("date-one", "05 Aug 2026", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("event-one", "Requirements approved", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)})}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{dataCell("date-two", "19 Aug 2026", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("event-two", "Pilot begins")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-three"), Children: []paperedit.NodeSpec{dataCell("date-three", "02 Sep 2026", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("event-three", "Go-live review", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)})}},
			}
		case "comparison-table":
			columns = []paperedit.NodeSpec{column("feature-column", 36), column("option-a-column", 32), column("option-b-column", 32)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableHeader, ID: childID("head"), Children: []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("head-row"), Children: []paperedit.NodeSpec{cell("head-feature", "FEATURE", true), cell("head-option-a", "MANUAL", true), cell("head-option-b", "AUTOMATED", true)}}}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{dataCell("feature-one", "Turnaround", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("option-a-one", "2 business days"), dataCell("option-b-one", "Under 4 hours")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{dataCell("feature-two", "Audit trail", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("option-a-two", "Spreadsheet"), dataCell("option-b-two", "Versioned log")}},
			}
		case "approval-block":
			columns = []paperedit.NodeSpec{column("label-column", 30), column("value-column", 70)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID("review-row"), Children: []paperedit.NodeSpec{dataCell("review-label", "REVIEWED", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("review-value", "Rafael Lima | Quality | 18 Jul 2026")}},
				{Kind: paperlang.NodeTableRow, ID: childID("approval-row"), Children: []paperedit.NodeSpec{dataCell("approval-label", "APPROVED", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringSuccess)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSuccessSoft)}), dataCell("approval-value", "Marina Costa | Director | 20 Jul 2026", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)})}},
			}
		case "invoice-totals":
			columns = []paperedit.NodeSpec{column("label-column", 72), column("amount-column", 28)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID("subtotal-row"), Children: []paperedit.NodeSpec{dataCell("subtotal-label", "Subtotal", paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("subtotal-value", "$2,480.00", paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("right")})}},
				{Kind: paperlang.NodeTableRow, ID: childID("tax-row"), Children: []paperedit.NodeSpec{dataCell("tax-label", "Tax", paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("tax-value", "$124.00", paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("right")})}},
				{Kind: paperlang.NodeTableRow, ID: childID("total-row"), Children: []paperedit.NodeSpec{dataCell("total-label", "TOTAL", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentStrong)}), dataCell("total-value", "$2,604.00", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("right")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentStrong)})}},
			}
		case "checklist":
			columns = []paperedit.NodeSpec{column("check-column", 5), column("task-column", 95)}
			checkCell := func(suffix string) paperedit.NodeSpec {
				return plainCell(suffix, " ",
					paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(1, "pt")},
					paperedit.PropertySpec{Name: "line-height", Value: paperedit.UnitValue(1, "pt")},
					paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(7, "pt")},
					paperedit.PropertySpec{Name: "border-width", Value: paperedit.UnitValue(1.25, "pt")},
					paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)},
					paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue("#FFFFFF")},
				)
			}
			taskCell := func(suffix, task string) paperedit.NodeSpec {
				return plainCell(suffix, task,
					paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(9, "pt")},
					paperedit.PropertySpec{Name: "line-height", Value: paperedit.UnitValue(12, "pt")},
					paperedit.PropertySpec{Name: "padding-top", Value: paperedit.UnitValue(6, "pt")},
					paperedit.PropertySpec{Name: "padding-right", Value: paperedit.UnitValue(8, "pt")},
					paperedit.PropertySpec{Name: "padding-bottom", Value: paperedit.UnitValue(6, "pt")},
					paperedit.PropertySpec{Name: "padding-left", Value: paperedit.UnitValue(12, "pt")},
					paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringText)},
					paperedit.PropertySpec{Name: "border-bottom-width", Value: paperedit.UnitValue(0.5, "pt")},
					paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringBorder)},
				)
			}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{checkCell("check-one"), taskCell("task-one", "Confirm scope and owner")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{checkCell("check-two"), taskCell("task-two", "Review supporting details")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-three"), Children: []paperedit.NodeSpec{checkCell("check-three"), taskCell("task-three", "Record completion and sign-off")}},
			}
		case "numbered-steps":
			columns = []paperedit.NodeSpec{column("number-column", 9), column("step-column", 91)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Properties: []paperedit.PropertySpec{{Name: "keep-with-next", Value: paperedit.BoolValue(true)}}, Children: []paperedit.NodeSpec{dataCell("number-one", "1", paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentStrong)}), dataCell("step-one", "Validate source records.", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)})}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Properties: []paperedit.PropertySpec{{Name: "keep-with-next", Value: paperedit.BoolValue(true)}}, Children: []paperedit.NodeSpec{dataCell("number-two", "2", paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccent)}), dataCell("step-two", "Review flagged exceptions.")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-three"), Children: []paperedit.NodeSpec{dataCell("number-three", "3", paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue("#4F8794")}), dataCell("step-three", "Publish the signed report.")}},
			}
		case "kpi-strip":
			columns = []paperedit.NodeSpec{column("metric-one-column", 34), column("metric-two-column", 33), column("metric-three-column", 33)}
			rows = []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("row"), Children: []paperedit.NodeSpec{
				dataCell("metric-one", "128\nCASES", paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(11, "pt")}, paperedit.PropertySpec{Name: "line-height", Value: paperedit.UnitValue(13, "pt")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(9, "pt")}),
				dataCell("metric-two", "96%\nON TIME", paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(11, "pt")}, paperedit.PropertySpec{Name: "line-height", Value: paperedit.UnitValue(13, "pt")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccent)}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(9, "pt")}),
				dataCell("metric-three", "2.4 d\nREVIEW", paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(11, "pt")}, paperedit.PropertySpec{Name: "line-height", Value: paperedit.UnitValue(13, "pt")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue("#4F8794")}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(9, "pt")}),
			}}}
		case "table-of-contents":
			columns = []paperedit.NodeSpec{column("section-column", 88), column("page-column", 12)}
			sectionCell := func(suffix, value string, bold bool) paperedit.NodeSpec {
				properties := []paperedit.PropertySpec{{Name: "padding", Value: paperedit.UnitValue(7, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringText)}, {Name: "border-bottom-width", Value: paperedit.UnitValue(0.5, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringBorder)}}
				if bold {
					properties = append(properties, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)})
				}
				return plainCell(suffix, value, properties...)
			}
			pageCell := func(suffix, value string) paperedit.NodeSpec {
				return plainCell(suffix, value, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(7, "pt")}, paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("right")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccent)}, paperedit.PropertySpec{Name: "border-bottom-width", Value: paperedit.UnitValue(0.5, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringBorder)})
			}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{sectionCell("section-one", "1. Introduction", true), pageCell("page-one", "1")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{sectionCell("section-two", "2. Findings", false), pageCell("page-two", "3")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-three"), Children: []paperedit.NodeSpec{sectionCell("section-three", "3. Recommendations", false), pageCell("page-three", "7")}},
			}
		case "risk-register":
			columns = []paperedit.NodeSpec{column("risk-column", 36), column("impact-column", 14), column("owner-column", 20), column("action-column", 30)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableHeader, ID: childID("head"), Children: []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("head-row"), Children: []paperedit.NodeSpec{cell("head-risk", "RISK", true), cell("head-impact", "IMPACT", true), cell("head-owner", "OWNER", true), cell("head-action", "MITIGATION", true)}}}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{dataCell("risk-one", "Vendor data delay", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}), dataCell("impact-one", "HIGH", paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringDanger)}), dataCell("owner-one", "Integration", paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("action-one", "Escalate by 24 Jul")}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{dataCell("risk-two", "Low reviewer capacity", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}), dataCell("impact-two", "MEDIUM", paperedit.PropertySpec{Name: "align", Value: paperedit.StringValue("center")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringWarning)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringWarningSoft)}), dataCell("owner-two", "Quality", paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("action-two", "Add backup reviewer")}},
			}
		case "change-log":
			columns = []paperedit.NodeSpec{column("version-column", 16), column("date-column", 20), column("author-column", 20), column("change-column", 44)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableHeader, ID: childID("head"), Children: []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("head-row"), Children: []paperedit.NodeSpec{cell("head-version", "VERSION", true), cell("head-date", "DATE", true), cell("head-author", "AUTHOR", true), cell("head-change", "CHANGE", true)}}}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{dataCell("version-one", "1.0", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("date-one", "2026-07-12"), dataCell("author-one", "M. Costa"), dataCell("change-one", "Baseline approved", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)})}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{dataCell("version-two", "1.1", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}), dataCell("date-two", "2026-07-18"), dataCell("author-two", "R. Lima"), dataCell("change-two", "Risk controls updated")}},
			}
		case "pros-cons":
			columns = []paperedit.NodeSpec{column("advantages-column", 50), column("tradeoffs-column", 50)}
			rows = []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableHeader, ID: childID("head"), Children: []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("head-row"), Children: []paperedit.NodeSpec{dataCell("head-pros", "ADVANTAGES", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSuccess)}), dataCell("head-cons", "TRADE-OFFS", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringDanger)})}}}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-one"), Children: []paperedit.NodeSpec{dataCell("pro-one", "+ Faster review", paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringSuccess)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSuccessSoft)}), dataCell("con-one", "- Setup effort", paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringDanger)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringDangerSoft)})}},
				{Kind: paperlang.NodeTableRow, ID: childID("row-two"), Children: []paperedit.NodeSpec{dataCell("pro-two", "+ Consistent audit trail", paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringSuccess)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSuccessSoft)}), dataCell("con-two", "- Training required", paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringDanger)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringDangerSoft)})}},
			}
		}
		split := "avoid"
		if template == "table" || template == "numbered-steps" {
			split = "rows"
		}
		children := append(columns, rows...)
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue(split)}}, Children: children}, nil
	case "canvas":
		return paperedit.NodeSpec{Kind: paperlang.NodeCanvas, ID: id, Properties: []paperedit.PropertySpec{{Name: "width", Value: paperedit.UnitValue(160, "pt")}, {Name: "height", Value: paperedit.UnitValue(100, "pt")}}, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeAnchor, ID: childID("panel"), Properties: []paperedit.PropertySpec{{Name: "width", Value: paperedit.UnitValue(120, "pt")}, {Name: "height", Value: paperedit.UnitValue(64, "pt")}, {Name: "left", Value: paperedit.StringValue("canvas.left + 12pt")}, {Name: "top", Value: paperedit.StringValue("canvas.top + 12pt")}, {Name: "background", Value: paperedit.StringValue("#DCEAF7")}, {Name: "alt", Value: paperedit.StringValue("Positioned design panel")}}},
		}}, nil
	case "note-box":
		return paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: id, Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("DECISION NOTE | Pilot release is limited to two clinics until error rates stay below 2%.")}, {Name: "bold", Value: paperedit.BoolValue(true)}, {Name: "size", Value: paperedit.UnitValue(9, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(13, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringInk)}, {Name: "padding", Value: paperedit.UnitValue(10, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}, {Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringAccent)}}}, nil
	case "qr-verification":
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: []paperedit.NodeSpec{
			column("qr-column", 18),
			column("verification-column", 82),
			{Kind: paperlang.NodeTableRow, ID: childID("row"), Children: []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableCell, ID: childID("qr"), Properties: []paperedit.PropertySpec{{Name: "padding", Value: paperedit.UnitValue(8, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}, {Name: "border-width", Value: paperedit.UnitValue(0.5, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringBorder)}}, Children: []paperedit.NodeSpec{{Kind: paperlang.NodeImage, ID: childID("image"), Properties: []paperedit.PropertySpec{{Name: "source", Value: paperedit.StringValue(authoringQRPlaceholderDataURI)}, {Name: "width", Value: paperedit.UnitValue(56, "pt")}, {Name: "height", Value: paperedit.UnitValue(56, "pt")}, {Name: "fit", Value: paperedit.StringValue("contain")}, {Name: "alt", Value: paperedit.StringValue("Replace with verification QR code")}}}}},
				{Kind: paperlang.NodeTableCell, ID: childID("copy"), Properties: []paperedit.PropertySpec{{Name: "padding", Value: paperedit.UnitValue(10, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringSurface)}, {Name: "border-width", Value: paperedit.UnitValue(0.5, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringBorder)}}, Children: []paperedit.NodeSpec{
					{Kind: paperlang.NodeParagraph, ID: childID("verification-title"), Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("VALIDATE RECORD")}, {Name: "bold", Value: paperedit.BoolValue(true)}, {Name: "size", Value: paperedit.UnitValue(9, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringInk)}}},
					{Kind: paperlang.NodeParagraph, ID: childID("verification-copy"), Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("Scan to confirm report OPS-042 and its approval status.")}, {Name: "size", Value: paperedit.UnitValue(8, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(11, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringMuted)}}},
				}},
			}},
		}}, nil
	case "clause":
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeTableRow, ID: childID("label-row"), Children: []paperedit.NodeSpec{plainCell("label", "CLAUSE 1", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(8, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(6, "pt")})}},
			{Kind: paperlang.NodeTableRow, ID: childID("title-row"), Children: []paperedit.NodeSpec{plainCell("title", "Data retention", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(11, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringInk)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(7, "pt")})}},
			{Kind: paperlang.NodeTableRow, ID: childID("copy-row"), Children: []paperedit.NodeSpec{plainCell("copy", "Signed reports are retained for five years; access is limited to authorized reviewers.", paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(8, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringText)}, paperedit.PropertySpec{Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)})}},
		}}, nil
	case "title-block":
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeTableRow, ID: childID("eyebrow-row"), Children: []paperedit.NodeSpec{plainCell("eyebrow", "OPERATIONS | Q2 2026", paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(7.5, "pt")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccent)}, paperedit.PropertySpec{Name: "padding-bottom", Value: paperedit.UnitValue(3, "pt")})}},
			{Kind: paperlang.NodeTableRow, ID: childID("title-row"), Children: []paperedit.NodeSpec{plainCell("title", "Quarterly operations brief", paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(20, "pt")}, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringInk)})}},
			{Kind: paperlang.NodeTableRow, ID: childID("subtitle-row"), Children: []paperedit.NodeSpec{plainCell("subtitle", "Service performance, delivery risks, and actions.", paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(9.5, "pt")}, paperedit.PropertySpec{Name: "line-height", Value: paperedit.UnitValue(13, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringMuted)}, paperedit.PropertySpec{Name: "padding-bottom", Value: paperedit.UnitValue(8, "pt")}, paperedit.PropertySpec{Name: "border-bottom-width", Value: paperedit.UnitValue(1.5, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)})}},
		}}, nil
	case "two-column":
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: []paperedit.NodeSpec{
			column("left-column", 50),
			column("right-column", 50),
			{Kind: paperlang.NodeTableHeader, ID: childID("head"), Children: []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("head-row"), Children: []paperedit.NodeSpec{cell("left-heading", "CURRENT STATE", true), cell("right-heading", "NEXT ACTION", true)}}}},
			{Kind: paperlang.NodeTableRow, ID: childID("body-row"), Children: []paperedit.NodeSpec{dataCell("left-copy", "18 records await review.", paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}), dataCell("right-copy", "Assign two backup reviewers by Friday.")}},
		}}, nil
	case "image-caption":
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: []paperedit.NodeSpec{
			column("image-column", 42),
			column("caption-column", 58),
			{Kind: paperlang.NodeTableRow, ID: childID("row"), Children: []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableCell, ID: childID("image-cell"), Properties: []paperedit.PropertySpec{{Name: "padding", Value: paperedit.UnitValue(8, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}, {Name: "border-width", Value: paperedit.UnitValue(0.5, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringBorder)}}, Children: []paperedit.NodeSpec{
					{Kind: paperlang.NodeImage, ID: childID("image"), Properties: []paperedit.PropertySpec{{Name: "source", Value: paperedit.StringValue(authoringPlaceholderDataURI)}, {Name: "width", Value: paperedit.UnitValue(180, "pt")}, {Name: "height", Value: paperedit.UnitValue(108, "pt")}, {Name: "fit", Value: paperedit.StringValue("cover")}, {Name: "alt", Value: paperedit.StringValue("Replace with a descriptive image")}}},
				}},
				{Kind: paperlang.NodeTableCell, ID: childID("caption-cell"), Properties: []paperedit.PropertySpec{{Name: "padding", Value: paperedit.UnitValue(12, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringSurface)}, {Name: "border-width", Value: paperedit.UnitValue(0.5, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringBorder)}}, Children: []paperedit.NodeSpec{
					{Kind: paperlang.NodeParagraph, ID: childID("figure-label"), Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("FIGURE 1")}, {Name: "size", Value: paperedit.UnitValue(8, "pt")}, {Name: "bold", Value: paperedit.BoolValue(true)}, {Name: "color", Value: paperedit.StringValue(authoringAccent)}}},
					{Kind: paperlang.NodeParagraph, ID: childID("caption"), Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("Pilot team reviewing the first clinic data import.")}, {Name: "size", Value: paperedit.UnitValue(9, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(12, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringText)}}},
					{Kind: paperlang.NodeParagraph, ID: childID("source"), Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("Program office | 18 Jul 2026")}, {Name: "size", Value: paperedit.UnitValue(7.5, "pt")}, {Name: "italic", Value: paperedit.BoolValue(true)}, {Name: "color", Value: paperedit.StringValue(authoringMuted)}}},
				}},
			}},
		}}, nil
	case "quote":
		return paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: id, Properties: []paperedit.PropertySpec{
			{Name: "text", Value: paperedit.StringValue("\"A clear audit trail turns review into a decision, not a search.\"\n- Quality review principle")}, {Name: "italic", Value: paperedit.BoolValue(true)}, {Name: "size", Value: paperedit.UnitValue(10, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(14, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringInk)}, {Name: "padding", Value: paperedit.UnitValue(11, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringSurface)}, {Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringAccent)},
		}}, nil
	case "disclaimer":
		return paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: id, Properties: []paperedit.PropertySpec{
			{Name: "text", Value: paperedit.StringValue("INTERNAL DRAFT | Forecast values are provisional until finance review is complete.")}, {Name: "size", Value: paperedit.UnitValue(7.5, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(11, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringMuted)}, {Name: "padding", Value: paperedit.UnitValue(8, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringWarningSoft)}, {Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringWarning)},
		}}, nil
	case "divider":
		return paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: id, Properties: []paperedit.PropertySpec{
			{Name: "text", Value: paperedit.StringValue(" ")}, {Name: "size", Value: paperedit.UnitValue(1, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(5, "pt")}, {Name: "border-bottom-width", Value: paperedit.UnitValue(1, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringAccent)},
		}}, nil
	case "cover-block":
		return paperedit.NodeSpec{Kind: paperlang.NodeColumn, ID: id, Properties: []paperedit.PropertySpec{{Name: "gap", Value: paperedit.UnitValue(8, "pt")}}, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeParagraph, ID: childID("eyebrow"), Properties: []paperedit.PropertySpec{{Name: "size", Value: paperedit.UnitValue(7.5, "pt")}, {Name: "bold", Value: paperedit.BoolValue(true)}, {Name: "align", Value: paperedit.StringValue("center")}, {Name: "color", Value: paperedit.StringValue(authoringAccent)}, {Name: "text", Value: paperedit.StringValue("PROGRAM BRIEF | JUL 2026")}}},
			{Kind: paperlang.NodeHeading, ID: childID("title"), Properties: []paperedit.PropertySpec{{Name: "level", Value: paperedit.NumberValue(1)}, {Name: "size", Value: paperedit.UnitValue(24, "pt")}, {Name: "align", Value: paperedit.StringValue("center")}, {Name: "color", Value: paperedit.StringValue(authoringInk)}, {Name: "text", Value: paperedit.StringValue("Community health access plan")}}},
			{Kind: paperlang.NodeParagraph, ID: childID("subtitle"), Properties: []paperedit.PropertySpec{{Name: "size", Value: paperedit.UnitValue(10, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(14, "pt")}, {Name: "align", Value: paperedit.StringValue("center")}, {Name: "color", Value: paperedit.StringValue(authoringText)}, {Name: "text", Value: paperedit.StringValue("Implementation brief | Northeast region")}}},
			{Kind: paperlang.NodeParagraph, ID: childID("metadata"), Properties: []paperedit.PropertySpec{{Name: "size", Value: paperedit.UnitValue(8, "pt")}, {Name: "align", Value: paperedit.StringValue("center")}, {Name: "color", Value: paperedit.StringValue(authoringMuted)}, {Name: "text", Value: paperedit.StringValue("Program office | 20 Jul 2026")}}},
		}}, nil
	case "recipient-block":
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: []paperedit.NodeSpec{
			{Kind: paperlang.NodeTableRow, ID: childID("label-row"), Children: []paperedit.NodeSpec{plainCell("label", "DELIVER TO", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(8, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccent)}, paperedit.PropertySpec{Name: "padding-bottom", Value: paperedit.UnitValue(5, "pt")})}},
			{Kind: paperlang.NodeTableRow, ID: childID("name-row"), Children: []paperedit.NodeSpec{plainCell("name", "Dr. Helena Moura", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(11, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringInk)}, paperedit.PropertySpec{Name: "padding-left", Value: paperedit.UnitValue(10, "pt")}, paperedit.PropertySpec{Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)})}},
			{Kind: paperlang.NodeTableRow, ID: childID("organization-row"), Children: []paperedit.NodeSpec{plainCell("organization", "Regional Health Coordination", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringText)}, paperedit.PropertySpec{Name: "padding-left", Value: paperedit.UnitValue(10, "pt")}, paperedit.PropertySpec{Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)})}},
			{Kind: paperlang.NodeTableRow, ID: childID("address-row"), Children: []paperedit.NodeSpec{plainCell("address", "Rua do Sol, 184\nFortaleza - CE, 60160-120\nBrazil", paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(8, "pt")}, paperedit.PropertySpec{Name: "line-height", Value: paperedit.UnitValue(11, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringMuted)}, paperedit.PropertySpec{Name: "padding-left", Value: paperedit.UnitValue(10, "pt")}, paperedit.PropertySpec{Name: "padding-bottom", Value: paperedit.UnitValue(6, "pt")}, paperedit.PropertySpec{Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)})}},
		}}, nil
	case "code-block":
		return paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: id, Properties: []paperedit.PropertySpec{
			{Name: "text", Value: paperedit.StringValue("func example() {\n    // Replace with code or preformatted text.\n}")}, {Name: "font", Value: paperedit.StringValue("Courier")}, {Name: "size", Value: paperedit.UnitValue(9, "pt")}, {Name: "line-height", Value: paperedit.UnitValue(13, "pt")}, {Name: "color", Value: paperedit.StringValue("#E6F0F2")}, {Name: "padding", Value: paperedit.UnitValue(12, "pt")}, {Name: "background", Value: paperedit.StringValue("#102D36")}, {Name: "border-left-width", Value: paperedit.UnitValue(4, "pt")}, {Name: "border-color", Value: paperedit.StringValue("#5FB2C0")},
		}}, nil
	case "status-banner":
		return paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: id, Properties: []paperedit.PropertySpec{
			{Name: "text", Value: paperedit.StringValue("APPROVED | Ready for release")}, {Name: "bold", Value: paperedit.BoolValue(true)}, {Name: "size", Value: paperedit.UnitValue(10, "pt")}, {Name: "color", Value: paperedit.StringValue(authoringSuccess)}, {Name: "padding", Value: paperedit.UnitValue(9, "pt")}, {Name: "background", Value: paperedit.StringValue(authoringSuccessSoft)}, {Name: "border-left-width", Value: paperedit.UnitValue(4, "pt")}, {Name: "border-color", Value: paperedit.StringValue(authoringSuccess)},
		}}, nil
	case "image-grid":
		image := func(suffix string) paperedit.NodeSpec {
			return paperedit.NodeSpec{Kind: paperlang.NodeImage, ID: childID(suffix), Properties: []paperedit.PropertySpec{{Name: "source", Value: paperedit.StringValue(authoringPlaceholderDataURI)}, {Name: "width", Value: paperedit.UnitValue(1, "fr")}, {Name: "height", Value: paperedit.UnitValue(108, "pt")}, {Name: "fit", Value: paperedit.StringValue("cover")}, {Name: "alt", Value: paperedit.StringValue("Replace with a descriptive image")}}}
		}
		return paperedit.NodeSpec{Kind: paperlang.NodeRow, ID: id, Properties: []paperedit.PropertySpec{{Name: "gap", Value: paperedit.UnitValue(12, "pt")}}, Children: []paperedit.NodeSpec{
			image("image-one"), image("image-two"),
		}}, nil
	case "decision-record":
		section := func(suffix, label, value string) []paperedit.NodeSpec {
			return []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID(suffix + "-label-row"), Children: []paperedit.NodeSpec{plainCell(suffix+"-label", label, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(8, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentSoft)}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(7, "pt")}, paperedit.PropertySpec{Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)})}},
				{Kind: paperlang.NodeTableRow, ID: childID(suffix + "-copy-row"), Children: []paperedit.NodeSpec{plainCell(suffix+"-copy", value, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(9, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringText)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)})}},
			}
		}
		rows := []paperedit.NodeSpec{{Kind: paperlang.NodeTableRow, ID: childID("title-row"), Children: []paperedit.NodeSpec{plainCell("title", "Decision record", paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "size", Value: paperedit.UnitValue(14, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringInk)}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(9, "pt")})}}}
		rows = append(rows, section("context", "CONTEXT", "Describe the problem, constraints, and relevant evidence.")...)
		rows = append(rows, section("decision", "DECISION", "State the chosen direction and why it was selected.")...)
		rows = append(rows, section("consequences", "CONSEQUENCES", "Record expected benefits, costs, and follow-up work.")...)
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: rows}, nil
	case "faq-block":
		question := func(suffix, question, answer string) []paperedit.NodeSpec {
			return []paperedit.NodeSpec{
				{Kind: paperlang.NodeTableRow, ID: childID(suffix + "-question-row"), Children: []paperedit.NodeSpec{plainCell(suffix+"-question", question, paperedit.PropertySpec{Name: "bold", Value: paperedit.BoolValue(true)}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue("#FFFFFF")}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringAccentStrong)}, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(8, "pt")})}},
				{Kind: paperlang.NodeTableRow, ID: childID(suffix + "-answer-row"), Children: []paperedit.NodeSpec{plainCell(suffix+"-answer", answer, paperedit.PropertySpec{Name: "padding", Value: paperedit.UnitValue(9, "pt")}, paperedit.PropertySpec{Name: "color", Value: paperedit.StringValue(authoringText)}, paperedit.PropertySpec{Name: "background", Value: paperedit.StringValue(authoringSurface)}, paperedit.PropertySpec{Name: "border-left-width", Value: paperedit.UnitValue(3, "pt")}, paperedit.PropertySpec{Name: "border-color", Value: paperedit.StringValue(authoringAccent)})}},
			}
		}
		rows := question("one", "Q1 | What is the first common question?", "Provide a direct, concise answer with the essential details.")
		rows = append(rows, question("two", "Q2 | What else should readers know?", "Add the second answer or remove this pair when unnecessary.")...)
		return paperedit.NodeSpec{Kind: paperlang.NodeTable, ID: id, Properties: []paperedit.PropertySpec{{Name: "split", Value: paperedit.StringValue("avoid")}}, Children: rows}, nil
	default:
		return paperedit.NodeSpec{}, workspaceError("INVALID_TEMPLATE", "unknown flow template", ErrInvalidQuery)
	}
}

func authoringDocumentPreset(preset, id string) (paperedit.NodeSpec, error) {
	if preset == "" {
		preset = "blank"
	}
	base := strings.TrimPrefix(id, "@")
	flow, err := authoringFlowTemplate("metadata-grid", "@"+base+"-metadata")
	if err != nil {
		return paperedit.NodeSpec{}, err
	}
	title, subtitle := "Untitled document", "Start writing with a clean, production-ready structure."
	switch preset {
	case "blank":
	case "letter":
		title, subtitle = "Business Letter", "Recipient name\nOrganization\nAddress\n\nDear recipient,"
	case "prescription":
		title, subtitle = "Prescription", "Patient and prescriber details"
	case "medical-report":
		title, subtitle = "Clinical Report", "Patient | encounter | responsible clinician"
	case "invoice":
		title, subtitle = "Invoice", "Bill to | invoice number | issue date"
	case "contract":
		title, subtitle = "Agreement", "Parties | effective date | reference"
	case "certificate":
		title, subtitle = "Certificate", "This certifies that"
	case "table-report":
		title, subtitle = "Tabular Report", "Reporting period | owner | generated date"
	default:
		return paperedit.NodeSpec{}, workspaceError("INVALID_TEMPLATE", "unknown document preset", ErrInvalidQuery)
	}
	children := []paperedit.NodeSpec{
		{Kind: paperlang.NodeHeading, ID: "@" + base + "-title", Properties: []paperedit.PropertySpec{{Name: "level", Value: paperedit.NumberValue(1)}, {Name: "size", Value: paperedit.UnitValue(28, "pt")}, {Name: "color", Value: paperedit.StringValue("#163A46")}, {Name: "text", Value: paperedit.StringValue(title)}}},
		{Kind: paperlang.NodeParagraph, ID: "@" + base + "-subtitle", Properties: []paperedit.PropertySpec{{Name: "color", Value: paperedit.StringValue("#475569")}, {Name: "text", Value: paperedit.StringValue(subtitle)}}},
		flow,
	}
	switch preset {
	case "prescription":
		medications, _ := authoringFlowTemplate("table", "@"+base+"-medications")
		children = append(children, paperedit.NodeSpec{Kind: paperlang.NodeHeading, ID: "@" + base + "-medications-title", Properties: []paperedit.PropertySpec{{Name: "level", Value: paperedit.NumberValue(2)}, {Name: "text", Value: paperedit.StringValue("Medication plan")}}}, medications)
	case "invoice", "table-report":
		table, _ := authoringFlowTemplate("table", "@"+base+"-items")
		children = append(children, table)
	case "contract":
		clause, _ := authoringFlowTemplate("clause", "@"+base+"-clause")
		children = append(children, clause)
	default:
		children = append(children, paperedit.NodeSpec{Kind: paperlang.NodeParagraph, ID: "@" + base + "-content", Properties: []paperedit.PropertySpec{{Name: "text", Value: paperedit.StringValue("Add document content here.")}}})
	}
	return paperedit.NodeSpec{Kind: paperlang.NodePage, ID: id, Properties: []paperedit.PropertySpec{{Name: "width", Value: paperedit.UnitValue(595.275590551, "pt")}, {Name: "height", Value: paperedit.UnitValue(841.88976378, "pt")}, {Name: "margin", Value: paperedit.UnitValue(36, "pt")}}, Children: []paperedit.NodeSpec{
		{Kind: paperlang.NodeHeader, ID: "@" + base + "-header", Children: []paperedit.NodeSpec{{Kind: paperlang.NodeParagraph, ID: "@" + base + "-header-copy", Properties: []paperedit.PropertySpec{{Name: "size", Value: paperedit.UnitValue(8, "pt")}, {Name: "bold", Value: paperedit.BoolValue(true)}, {Name: "color", Value: paperedit.StringValue("#2C6E7F")}, {Name: "text", Value: paperedit.StringValue("YOUR ORGANIZATION  |  DOCUMENT")}}}}},
		{Kind: paperlang.NodeFooter, ID: "@" + base + "-footer", Children: []paperedit.NodeSpec{{Kind: paperlang.NodeParagraph, ID: "@" + base + "-footer-copy", Properties: []paperedit.PropertySpec{{Name: "size", Value: paperedit.UnitValue(8, "pt")}, {Name: "color", Value: paperedit.StringValue("#64748B")}, {Name: "text", Value: paperedit.StringValue("Confidential | Replace with document footer")}}}}},
		{Kind: paperlang.NodeBody, ID: "@" + base + "-body", Children: children},
	}}, nil
}

func safeAuthoringImportPath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsRune(value, '\x00') || strings.Contains(value, "://") || strings.HasPrefix(value, "~") || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "\\") || (len(value) > 1 && value[1] == ':') {
		return false
	}
	return path.Clean(strings.ReplaceAll(value, "\\", "/")) != "."
}

type PaperCreateScenarioRequest struct {
	Guard  PaperMutationGuard `json:"guard"`
	Name   string             `json:"name"`
	Schema string             `json:"schema"`
	Preset string             `json:"preset"`
}

type PaperScenarioMatrixCase struct {
	Name   string `json:"name"`
	Preset string `json:"preset"`
}

type PaperCreateScenarioMatrixRequest struct {
	Guard  PaperMutationGuard        `json:"guard"`
	Schema string                    `json:"schema"`
	Cases  []PaperScenarioMatrixCase `json:"cases"`
}

// PaperCreateScenarioMatrix inserts several bounded schema-shaped fixtures in
// one source patch. Matrix creation is intentionally explicit: callers name
// every case and choose its compiler-owned preset.
func (w *Workspace) PaperCreateScenarioMatrix(request PaperCreateScenarioMatrixRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	parent := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if parent == nil || parent.Kind != paperlang.NodeDocument {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_PARENT", "scenario matrix parent must be the addressed document", paperedit.ErrInvalidOperation)
	}
	if !validAuthorityNodeID(request.Schema) || len(request.Cases) == 0 || len(request.Cases) > 16 {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_MATRIX", "matrix requires one schema and between one and sixteen cases", ErrInvalidQuery)
	}
	metadata := papercompile.ExtractSchemas(revision.parsed.AST)
	if !metadata.OK() {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_SCHEMA", "schema metadata contains compiler diagnostics", ErrInvalidSource)
	}
	var schema *papercompile.SchemaDescriptor
	for index := range metadata.Schemas {
		if metadata.Schemas[index].Name == request.Schema {
			schema = &metadata.Schemas[index]
			break
		}
	}
	if schema == nil {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_SCHEMA", "selected schema does not exist in the exact source revision", ErrInvalidQuery)
	}
	seen := make(map[string]struct{}, len(request.Cases))
	nodes := make([]paperedit.NodeSpec, 0, len(request.Cases))
	for _, matrixCase := range request.Cases {
		if !validAuthorityNodeID(matrixCase.Name) || requestCasePreset(matrixCase.Preset) == "" {
			return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_MATRIX", "matrix cases require readable IDs and empty, typical, or stress presets", ErrInvalidQuery)
		}
		if _, exists := seen[matrixCase.Name]; exists || findNodeByID(revision.parsed.AST.Root, matrixCase.Name) != nil {
			return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_MATRIX", "matrix case IDs must be unique and absent from the exact source revision", paperedit.ErrInvalidOperation)
		}
		seen[matrixCase.Name] = struct{}{}
		nodes = append(nodes, paperedit.NodeSpec{Kind: paperlang.NodeScenario, ID: matrixCase.Name, Children: scenarioFieldSpecs(schema.Fields, matrixCase.Preset, 0)})
	}
	return w.applyPaperMutation("create_scenario_matrix", request.Guard, opened, revision,
		[]string{request.Guard.Target}, []paperedit.Operation{paperedit.InsertNodes{Parent: request.Guard.Target, Nodes: nodes}}, "INVALID_SCENARIO_RESULT")
}

func requestCasePreset(value string) string {
	switch value {
	case "empty", "typical", "stress":
		return value
	default:
		return ""
	}
}

type PaperAddSchemaFieldRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	ItemType string             `json:"item_type,omitempty"`
	MaxItems uint32             `json:"max_items,omitempty"`
}

// PaperAddSchemaField adds one compiler-shaped field below a schema or an
// object/object-list field. Object starters receive one valid nested string
// field so the edit remains compileable in a single transaction.
func (w *Workspace) PaperAddSchemaField(request PaperAddSchemaFieldRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	parent := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if parent == nil || !validAuthorityNodeID(request.ID) {
		return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "schema field requires an exact schema/field parent and readable @id", ErrInvalidQuery)
	}
	if parent.Kind != paperlang.NodeSchema && parent.Kind != paperlang.NodeObjectType && parent.Kind != paperlang.NodeField {
		return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "schema field parent must be a schema, custom object, or inline object field", paperedit.ErrInvalidOperation)
	}
	if parent.Kind == paperlang.NodeField && !schemaFieldCanContainChildren(parent) {
		return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "nested fields require an object or object-list parent", paperedit.ErrInvalidOperation)
	}
	if sourceNodesByID(revision.parsed.AST.Root, request.ID) != nil {
		return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "field ID already exists in the exact source revision", paperedit.ErrInvalidOperation)
	}
	fieldKind, ok := papercompile.SchemaKind(request.Type), false
	typeRef := ""
	switch fieldKind {
	case papercompile.SchemaString, papercompile.SchemaNumber, papercompile.SchemaBool, papercompile.SchemaObject, papercompile.SchemaList:
		ok = true
	default:
		if schemaObjectTypeExists(revision.parsed.AST.Root, request.Type) {
			ok = true
			typeRef = request.Type
		}
	}
	if !ok {
		return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "field type must be string, number, bool, object, list, or a declared custom object", ErrInvalidQuery)
	}
	properties := []paperedit.PropertySpec(nil)
	itemTypeRef := ""
	if fieldKind == papercompile.SchemaList {
		itemKind := papercompile.SchemaKind(request.ItemType)
		switch itemKind {
		case papercompile.SchemaString, papercompile.SchemaNumber, papercompile.SchemaBool, papercompile.SchemaObject:
		default:
			if schemaObjectTypeExists(revision.parsed.AST.Root, request.ItemType) {
				itemTypeRef = request.ItemType
			} else {
				return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "list item type must be string, number, bool, object, or a declared custom object", ErrInvalidQuery)
			}
		}
		maxItems := request.MaxItems
		if maxItems == 0 {
			maxItems = 16
		}
		if maxItems > 1_000_000 {
			return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "list max-items exceeds the schema limit", ErrLimit)
		}
		properties = append(properties, paperedit.PropertySpec{Name: "max-items", Value: paperedit.NumberValue(float64(maxItems))})
	}
	field := paperedit.NodeSpec{Kind: paperlang.NodeField, ID: request.ID, FieldType: paperlang.SchemaFieldType(fieldKind), TypeRef: typeRef, ItemType: paperlang.SchemaFieldType(request.ItemType), ItemTypeRef: itemTypeRef, Properties: properties}
	if typeRef != "" {
		field.FieldType = ""
	}
	if itemTypeRef != "" {
		field.ItemType = ""
	}
	if fieldKind == papercompile.SchemaObject {
		base := strings.TrimPrefix(request.ID, "@")
		if len(base) > 110 {
			return PaperMutationResult{}, workspaceError("INVALID_SCHEMA_FIELD", "object field ID is too long for its starter child", ErrInvalidQuery)
		}
		field.Children = []paperedit.NodeSpec{{Kind: paperlang.NodeField, ID: "@" + base + "-value", FieldType: paperlang.FieldString}}
	} else if fieldKind == papercompile.SchemaList && request.ItemType == string(papercompile.SchemaObject) {
		field.Children = []paperedit.NodeSpec{{Kind: paperlang.NodeField, ID: "@value", FieldType: paperlang.FieldString}}
	}
	return w.applyPaperMutation("add_schema_field", request.Guard, opened, revision,
		[]string{request.Guard.Target}, []paperedit.Operation{paperedit.InsertNode{Parent: request.Guard.Target, Node: field}}, "INVALID_SCHEMA_FIELD")
}

func schemaObjectTypeExists(root *paperlang.Node, name string) bool {
	if root == nil || name == "" {
		return false
	}
	for _, member := range root.Members {
		if member.Node != nil && member.Node.Kind == paperlang.NodeObjectType && strings.TrimPrefix(member.Node.ID, "@") == name {
			return true
		}
	}
	return false
}

func schemaFieldCanContainChildren(node *paperlang.Node) bool {
	if node.FieldType == paperlang.FieldObject {
		return true
	}
	if node.FieldType != paperlang.FieldList {
		return false
	}
	return node.ItemType == paperlang.FieldObject
}

type PaperSetScenarioFixtureValueRequest struct {
	Guard  PaperMutationGuard `json:"guard"`
	Path   string             `json:"path"`
	Kind   string             `json:"kind,omitempty"`
	Text   string             `json:"text,omitempty"`
	Bool   *bool              `json:"bool,omitempty"`
	Number *float64           `json:"number,omitempty"`
}

// PaperSetScenarioValue edits one scalar fixture in place using a path local
// to the exact scenario root. The existing scalar kind is the default type;
// callers cannot silently change a fixture's schema contract.
func (w *Workspace) PaperSetScenarioFixtureValue(request PaperSetScenarioFixtureValueRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	scenario := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if scenario == nil || scenario.Kind != paperlang.NodeScenario || !validBindingPath(request.Path) {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_VALUE", "fixture value requires an authored scenario and relative dotted path", paperedit.ErrInvalidOperation)
	}
	node, err := scenarioRelativeNode(scenario, request.Path)
	if err != nil || node.Kind != paperlang.NodeValue || node.Value == nil {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_VALUE", "fixture path must resolve to one scalar value", paperedit.ErrInvalidOperation)
	}
	value, err := scenarioPaperValue(node.Value.Kind, request)
	if err != nil {
		return PaperMutationResult{}, err
	}
	return w.applyPaperMutation("set_scenario_value", request.Guard, opened, revision,
		[]string{request.Guard.Target}, []paperedit.Operation{paperedit.SetNodeValue{Root: request.Guard.Target, Path: request.Path, Value: value}}, "INVALID_SCENARIO_VALUE")
}

func scenarioPaperValue(kind paperlang.ScalarKind, request PaperSetScenarioFixtureValueRequest) (paperedit.Value, error) {
	want := request.Kind
	if want == "" {
		want = string(kind)
	}
	if want != string(kind) {
		return paperedit.Value{}, workspaceError("INVALID_SCENARIO_VALUE", "fixture value type must match its declared scalar kind", paperedit.ErrInvalidOperation)
	}
	switch kind {
	case paperlang.ScalarString:
		return paperedit.StringValue(request.Text), nil
	case paperlang.ScalarBool:
		if request.Bool != nil {
			return paperedit.BoolValue(*request.Bool), nil
		}
		parsed, err := strconv.ParseBool(strings.TrimSpace(request.Text))
		if err != nil {
			return paperedit.Value{}, workspaceError("INVALID_SCENARIO_VALUE", "boolean fixture values must be true or false", err)
		}
		return paperedit.BoolValue(parsed), nil
	case paperlang.ScalarNumber:
		var number float64
		if request.Number != nil {
			number = *request.Number
		} else {
			parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(request.Text), 64)
			if parseErr != nil {
				return paperedit.Value{}, workspaceError("INVALID_SCENARIO_VALUE", "number fixture values must be finite decimals", parseErr)
			}
			number = parsed
		}
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return paperedit.Value{}, workspaceError("INVALID_SCENARIO_VALUE", "number fixture values must be finite", paperedit.ErrInvalidOperation)
		}
		return paperedit.NumberValue(number), nil
	default:
		return paperedit.Value{}, workspaceError("INVALID_SCENARIO_VALUE", "null and unit fixture values are not editable by this matrix control", paperedit.ErrInvalidOperation)
	}
}

func scenarioRelativeNode(root *paperlang.Node, path string) (*paperlang.Node, error) {
	current := root
	for _, segment := range strings.Split(path, ".") {
		segment = strings.TrimSuffix(segment, "[]")
		if segment == "" {
			return nil, fmt.Errorf("empty fixture path segment")
		}
		if segment[0] != '@' {
			segment = "@" + segment
		}
		var found *paperlang.Node
		for _, member := range current.Members {
			if member.Node != nil && member.Node.ID == segment {
				if found != nil {
					return nil, fmt.Errorf("fixture path is ambiguous")
				}
				found = member.Node
			}
		}
		if found == nil {
			return nil, fmt.Errorf("fixture path segment %s is absent", segment)
		}
		current = found
	}
	return current, nil
}

type PaperManageScenarioRequest struct {
	Guard   PaperMutationGuard `json:"guard"`
	Action  string             `json:"action"`
	NewName string             `json:"new_name,omitempty"`
}

type PaperManageNodeRequest struct {
	Guard   PaperMutationGuard `json:"guard"`
	Action  string             `json:"action"`
	NewName string             `json:"new_name,omitempty"`
}

// PaperManageNode renames or removes an optional authored content node. Root
// document structure, definitions, schemas, and fixtures use dedicated tools
// and cannot be changed through this lifecycle endpoint.
func (w *Workspace) PaperManageNode(request PaperManageNodeRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node, parent := sourceNodeAndParent(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || parent == nil || !authoringNodeLifecycleAllowed(node.Kind) {
		return PaperMutationResult{}, workspaceError("INVALID_NODE_TARGET", "node lifecycle target must be an optional authored content node", paperedit.ErrInvalidOperation)
	}
	var operation paperedit.Operation
	switch request.Action {
	case "rename":
		if !validAuthorityNodeID(request.NewName) || request.NewName == request.Guard.Target {
			return PaperMutationResult{}, workspaceError("INVALID_NODE", "node rename requires a distinct readable @id", ErrInvalidQuery)
		}
		if findNodeByID(revision.parsed.AST.Root, request.NewName) != nil {
			return PaperMutationResult{}, workspaceError("INVALID_NODE", "node ID already exists in the exact source revision", paperedit.ErrInvalidOperation)
		}
		operation = paperedit.RenameID{Target: request.Guard.Target, NewID: request.NewName}
	case "delete":
		if request.NewName != "" {
			return PaperMutationResult{}, workspaceError("INVALID_NODE", "node delete does not accept a replacement ID", ErrInvalidQuery)
		}
		operation = paperedit.DeleteNode{Target: request.Guard.Target}
	default:
		return PaperMutationResult{}, workspaceError("INVALID_NODE", "node action must be rename or delete", ErrInvalidQuery)
	}
	return w.applyPaperMutation("manage_node", request.Guard, opened, revision,
		[]string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_NODE_RESULT")
}

func authoringNodeLifecycleAllowed(kind paperlang.NodeKind) bool {
	switch kind {
	case paperlang.NodeCanvas, paperlang.NodeAnchor, paperlang.NodeHeading, paperlang.NodeText, paperlang.NodeParagraph,
		paperlang.NodeList, paperlang.NodeItem, paperlang.NodePageBreak, paperlang.NodeRow, paperlang.NodeColumn,
		paperlang.NodeImage, paperlang.NodeTable, paperlang.NodeTableRow, paperlang.NodeTableCell, paperlang.NodeTableHeader,
		paperlang.NodeTableColumn, paperlang.NodeUse, paperlang.NodeRepeat, paperlang.NodeLoop:
		return true
	default:
		return false
	}
}

// PaperManageScenario provides the bounded lifecycle operations needed by a
// scenario matrix after creation. Rename and delete remain source edits, so
// they preserve comments and participate in the same exact-revision edit.
func (w *Workspace) PaperManageScenario(request PaperManageScenarioRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node, parent := sourceNodeAndParent(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || parent == nil || node.Kind != paperlang.NodeScenario || parent.Kind != paperlang.NodeDocument {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_TARGET", "scenario lifecycle target must be an authored scenario directly beneath the document", paperedit.ErrInvalidOperation)
	}
	var operation paperedit.Operation
	switch request.Action {
	case "rename":
		if !validAuthorityNodeID(request.NewName) || request.NewName == request.Guard.Target {
			return PaperMutationResult{}, workspaceError("INVALID_SCENARIO", "scenario rename requires a distinct readable @id", ErrInvalidQuery)
		}
		if findNodeByID(revision.parsed.AST.Root, request.NewName) != nil {
			return PaperMutationResult{}, workspaceError("INVALID_SCENARIO", "scenario ID already exists in the exact source revision", paperedit.ErrInvalidOperation)
		}
		operation = paperedit.RenameID{Target: request.Guard.Target, NewID: request.NewName}
	case "delete":
		if request.NewName != "" {
			return PaperMutationResult{}, workspaceError("INVALID_SCENARIO", "scenario delete does not accept a replacement ID", ErrInvalidQuery)
		}
		operation = paperedit.DeleteNode{Target: request.Guard.Target}
	default:
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO", "scenario action must be rename or delete", ErrInvalidQuery)
	}
	return w.applyPaperMutation("manage_scenario", request.Guard, opened, revision,
		[]string{request.Guard.Target}, []paperedit.Operation{operation}, "INVALID_SCENARIO_RESULT")
}

// PaperCreateScenario creates one schema-shaped fixture from compiler-owned
// descriptors. It does not infer schema syntax or accept arbitrary CST.
func (w *Workspace) PaperCreateScenario(request PaperCreateScenarioRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	parent := findNodeByID(revision.parsed.AST.Root, request.Guard.Target)
	if parent == nil || parent.Kind != paperlang.NodeDocument {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_PARENT", "scenario parent must be the addressed document", paperedit.ErrInvalidOperation)
	}
	if !validAuthorityNodeID(request.Name) || !validAuthorityNodeID(request.Schema) {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO", "scenario and schema require bounded readable @ids", ErrInvalidQuery)
	}
	if request.Preset != "empty" && request.Preset != "typical" && request.Preset != "stress" {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO", "preset must be empty, typical, or stress", ErrInvalidQuery)
	}
	metadata := papercompile.ExtractSchemas(revision.parsed.AST)
	if !metadata.OK() {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_SCHEMA", "schema metadata contains compiler diagnostics", ErrInvalidSource)
	}
	var schema *papercompile.SchemaDescriptor
	for index := range metadata.Schemas {
		if metadata.Schemas[index].Name == request.Schema {
			schema = &metadata.Schemas[index]
			break
		}
	}
	if schema == nil {
		return PaperMutationResult{}, workspaceError("INVALID_SCENARIO_SCHEMA", "selected schema does not exist in the exact source revision", ErrInvalidQuery)
	}
	node := paperedit.NodeSpec{Kind: paperlang.NodeScenario, ID: request.Name, Children: scenarioFieldSpecs(schema.Fields, request.Preset, 0)}
	return w.applyPaperMutation("create_scenario", request.Guard, opened, revision,
		[]string{request.Guard.Target}, []paperedit.Operation{paperedit.InsertNode{Parent: request.Guard.Target, Node: node}}, "INVALID_SCENARIO_RESULT")
}

func scenarioFieldSpecs(fields []papercompile.FieldDescriptor, preset string, depth int) []paperedit.NodeSpec {
	result := make([]paperedit.NodeSpec, 0, len(fields))
	for _, field := range fields {
		id := "@" + field.Name
		switch field.Kind {
		case papercompile.SchemaObject:
			result = append(result, paperedit.NodeSpec{Kind: paperlang.NodeObject, ID: id, Children: scenarioFieldSpecs(field.Fields, preset, depth+1)})
		case papercompile.SchemaList:
			items := 0
			switch preset {
			case "typical":
				items = 1
			case "stress":
				items = 3
			}
			if field.MaxItems > 0 && items > int(field.MaxItems) {
				items = int(field.MaxItems)
			}
			children := make([]paperedit.NodeSpec, 0, items)
			for index := 0; index < items; index++ {
				itemID := fmt.Sprintf("@item-%d", index+1)
				if field.ItemKind == papercompile.SchemaObject {
					children = append(children, paperedit.NodeSpec{Kind: paperlang.NodeObject, ID: itemID, Children: scenarioFieldSpecs(field.Fields, preset, depth+1)})
				} else {
					value := scenarioScalar(field.ItemKind, preset, depth+1)
					children = append(children, paperedit.NodeSpec{Kind: paperlang.NodeValue, ID: itemID, Value: &value})
				}
			}
			result = append(result, paperedit.NodeSpec{Kind: paperlang.NodeKeyedList, ID: id, Children: children})
		default:
			value := scenarioScalar(field.Kind, preset, depth)
			result = append(result, paperedit.NodeSpec{Kind: paperlang.NodeValue, ID: id, Value: &value})
		}
	}
	return result
}

func scenarioScalar(kind papercompile.SchemaKind, preset string, depth int) paperedit.Value {
	switch kind {
	case papercompile.SchemaNumber:
		if preset == "stress" {
			return paperedit.NumberValue(999999.99)
		}
		if preset == "typical" {
			return paperedit.NumberValue(123.45)
		}
		return paperedit.NumberValue(0)
	case papercompile.SchemaBool:
		return paperedit.BoolValue(preset == "typical")
	default:
		if preset == "stress" {
			return paperedit.StringValue(strings.Repeat("Wide value ", 8))
		}
		if preset == "typical" {
			return paperedit.StringValue("Sample value")
		}
		return paperedit.StringValue("")
	}
}
