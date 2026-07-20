// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"strings"
	"unicode/utf8"

	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

type PaperTableProperty string

const (
	PaperTableSplit           PaperTableProperty = "split"
	PaperTableRepeatHeader    PaperTableProperty = "repeat-header"
	PaperTableCaption         PaperTableProperty = "caption"
	PaperTableColumnWidth     PaperTableProperty = "width"
	PaperTableColumnMin       PaperTableProperty = "min-width"
	PaperTableColumnMax       PaperTableProperty = "max-width"
	PaperTableCellIsHeader    PaperTableProperty = "header-cell"
	PaperTableCellAlign       PaperTableProperty = "vertical-align"
	PaperTableRowKeepTogether PaperTableProperty = "keep-together"
	PaperTableRowKeepWithNext PaperTableProperty = "keep-with-next"
	PaperTableRowOrphans      PaperTableProperty = "orphans"
	PaperTableRowWidows       PaperTableProperty = "widows"
	PaperTableCellColSpan     PaperTableProperty = "colspan"
	PaperTableCellRowSpan     PaperTableProperty = "rowspan"
)

type PaperSetTablePropertyRequest struct {
	Guard    PaperMutationGuard `json:"guard"`
	Property PaperTableProperty `json:"property"`
	Split    string             `json:"split,omitempty"`
	Points   float64            `json:"points,omitempty"`
	Length   string             `json:"length,omitempty"`
	Text     string             `json:"text,omitempty"`
	Kind     string             `json:"kind,omitempty"`
	Bool     bool               `json:"bool,omitempty"`
	Count    uint32             `json:"count,omitempty"`
}

func (w *Workspace) PaperSetTableProperty(request PaperSetTablePropertyRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	node, table := sourceNodeAndTable(revision.parsed.AST.Root, request.Guard.Target)
	if node == nil || table == nil || table.ID == "" {
		return PaperMutationResult{}, workspaceError("INVALID_TABLE_TARGET", "table mutation requires an exact table, table-column, table-row, or cell source node", paperedit.ErrInvalidOperation)
	}
	if table.ID != request.Guard.Target {
		if err := requireAdditionalTargetGuard(revision, request.Guard, table.ID); err != nil {
			return PaperMutationResult{}, err
		}
	}
	var value paperedit.Value
	switch request.Property {
	case PaperTableSplit:
		if node.Kind != paperlang.NodeTable || request.Points != 0 || request.Length != "" || request.Text != "" || request.Kind != "" || request.Bool || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "split targets a table without unrelated values", paperedit.ErrInvalidOperation)
		}
		split := strings.ToLower(strings.TrimSpace(request.Split))
		if split != "rows" && split != "avoid" {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "split must be rows or avoid", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(split)
	case PaperTableRepeatHeader:
		if node.Kind != paperlang.NodeTable || request.Split != "" || request.Points != 0 || request.Length != "" || request.Text != "" || request.Kind != "" || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "repeat-header targets a table boolean", paperedit.ErrInvalidOperation)
		}
		value = paperedit.BoolValue(request.Bool)
	case PaperTableCaption:
		if node.Kind != paperlang.NodeTable || request.Split != "" || request.Points != 0 || request.Length != "" || request.Kind != "" || request.Bool || request.Count != 0 || !utf8.ValidString(request.Text) || len(request.Text) > w.maxMutationPayloadBytes() {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "table caption must be valid bounded UTF-8", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(request.Text)
	case PaperTableColumnWidth, PaperTableColumnMin, PaperTableColumnMax:
		resolved, ok := responsiveLayoutLengthValue(request.Length, request.Points, true, true)
		if node.Kind != paperlang.NodeTableColumn || request.Split != "" || request.Text != "" || request.Kind != "" || request.Bool || request.Count != 0 || !ok {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "table column requires auto, a bounded percentage, or a finite positive physical length", paperedit.ErrInvalidOperation)
		}
		value = resolved
	case PaperTableCellIsHeader:
		if node.Kind != paperlang.NodeTableCell || request.Split != "" || request.Points != 0 || request.Length != "" || request.Text != "" || request.Kind != "" || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "header-cell targets a cell boolean", paperedit.ErrInvalidOperation)
		}
		value = paperedit.BoolValue(request.Bool)
	case PaperTableCellAlign:
		align := strings.ToLower(strings.TrimSpace(request.Kind))
		if node.Kind != paperlang.NodeTableCell || request.Split != "" || request.Points != 0 || request.Length != "" || request.Text != "" || request.Bool || request.Count != 0 || !layoutChoice(align, "top", "middle", "bottom") {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "cell vertical alignment must be top, middle, or bottom", paperedit.ErrInvalidOperation)
		}
		value = paperedit.StringValue(align)
	case PaperTableRowKeepTogether, PaperTableRowKeepWithNext:
		if node.Kind != paperlang.NodeTableRow || request.Split != "" || request.Points != 0 || request.Length != "" || request.Text != "" || request.Kind != "" || request.Count != 0 {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "table-row keep policy accepts only a boolean", paperedit.ErrInvalidOperation)
		}
		value = paperedit.BoolValue(request.Bool)
	case PaperTableRowOrphans, PaperTableRowWidows:
		if node.Kind != paperlang.NodeTableRow || request.Count < 1 || request.Count > 1<<20 || request.Split != "" || request.Points != 0 || request.Length != "" || request.Text != "" || request.Kind != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "table-row pagination count must be from 1 through 1048576", paperedit.ErrInvalidOperation)
		}
		value = paperedit.NumberValue(float64(request.Count))
	case PaperTableCellColSpan, PaperTableCellRowSpan:
		if node.Kind != paperlang.NodeTableCell || request.Count < 1 || request.Count > 1024 || request.Split != "" || request.Points != 0 || request.Length != "" || request.Text != "" || request.Kind != "" || request.Bool {
			return PaperMutationResult{}, workspaceError("INVALID_TABLE_VALUE", "cell span must be from 1 through 1024", paperedit.ErrInvalidOperation)
		}
		value = paperedit.NumberValue(float64(request.Count))
	default:
		return PaperMutationResult{}, workspaceError("INVALID_TABLE_PROPERTY", "table property is outside the closed mutation vocabulary", paperedit.ErrInvalidOperation)
	}
	targets := []string{request.Guard.Target}
	if table.ID != request.Guard.Target {
		targets = append(targets, table.ID)
	}
	return w.applyPaperMutation("set_table_property", request.Guard, opened, revision, targets, []paperedit.Operation{paperedit.SetProperty{Target: request.Guard.Target, Name: string(request.Property), Value: value}}, "INVALID_TABLE_PROPERTY_STATE")
}

func sourceNodeAndTable(root *paperlang.Node, target string) (*paperlang.Node, *paperlang.Node) {
	var found, table *paperlang.Node
	var walk func(*paperlang.Node, *paperlang.Node)
	walk = func(node, owner *paperlang.Node) {
		if node == nil || found != nil {
			return
		}
		next := owner
		if node.Kind == paperlang.NodeTable {
			next = node
		}
		if node.ID == target {
			found, table = node, next
			return
		}
		for _, member := range node.Members {
			walk(member.Node, next)
		}
	}
	walk(root, nil)
	return found, table
}
