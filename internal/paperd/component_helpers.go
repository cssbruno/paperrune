// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"encoding/json"
	"strings"

	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

type PaperFillSlotRequest struct {
	Guard   PaperMutationGuard   `json:"guard"`
	Slot    string               `json:"slot"`
	Content []paperedit.NodeSpec `json:"content"`
}

// PaperFillSlot validates one component slot fill and publishes it as a
// transient semantic edit. It creates no durable history.
func (w *Workspace) PaperFillSlot(request PaperFillSlotRequest) (PaperMutationResult, error) {
	opened, revision, err := w.mutationRevision(request.Guard)
	if err != nil {
		return PaperMutationResult{}, err
	}
	operation, targets, err := w.prepareSlotFill(revision, request.Guard.Target, request.Slot, request.Content)
	if err != nil {
		return PaperMutationResult{}, err
	}
	return w.applyPaperMutation("fill_slot", request.Guard, opened, revision, targets, []paperedit.Operation{operation}, "INVALID_SLOT_FILL")
}

func (w *Workspace) prepareSlotFill(revision *revisionRecord, target, slotID string, content []paperedit.NodeSpec) (paperedit.Operation, []string, error) {
	if slotID == "" || len(slotID) > w.limits.MaxQueryBytes || slotID[0] != '@' {
		return nil, nil, workspaceError("INVALID_SLOT_FILL", "slot must be a bounded readable @id", paperedit.ErrInvalidOperation)
	}
	if len(content) == 0 || len(content) > w.limits.MaxOperations {
		return nil, nil, workspaceError("SLOT_FILL_LIMIT", "slot fill content count is outside configured bounds", ErrLimit)
	}
	encoded, err := json.Marshal(content)
	if err != nil || len(encoded) > w.maxMutationPayloadBytes() {
		return nil, nil, workspaceError("SLOT_FILL_LIMIT", "slot fill payload exceeds configured bounds", ErrLimit)
	}
	nodes := 0
	for _, node := range content {
		if !countNodeSpecBounded(node, &nodes, w.limits.MaxNodes) {
			return nil, nil, workspaceError("SLOT_FILL_LIMIT", "slot fill node count exceeds configured bounds", ErrLimit)
		}
	}
	use := findNodeByID(revision.parsed.AST.Root, target)
	if use == nil || use.Kind != paperlang.NodeUse {
		return nil, nil, workspaceError("INVALID_SLOT_FILL", "slot fill target must be a component use", paperedit.ErrInvalidOperation)
	}
	componentName, err := componentReference(use)
	if err != nil {
		return nil, nil, err
	}
	definition, err := uniqueComponentDefinition(revision.parsed.AST.Root, componentName)
	if err != nil {
		return nil, nil, err
	}
	slot, err := uniqueComponentSlot(definition, slotID)
	if err != nil {
		return nil, nil, err
	}
	for _, member := range use.Members {
		if member.Node != nil && member.Node.Kind == paperlang.NodeFill && member.Node.ID == slotID {
			return nil, nil, workspaceError("SLOT_CARDINALITY", "component use already has a fill for this slot", paperedit.ErrInvalidOperation)
		}
	}
	slotType, err := slotTypeForMutation(slot)
	if err != nil {
		return nil, nil, err
	}
	for _, node := range content {
		if !slotAcceptsMutation(slotType, node.Kind) {
			return nil, nil, workspaceError("SLOT_TYPE", "slot content does not satisfy the declared slot type", paperedit.ErrInvalidOperation)
		}
	}
	fill := paperedit.NodeSpec{Kind: paperlang.NodeFill, ID: slotID, Children: cloneNodeSpecsForMutation(content)}
	return paperedit.InsertNode{Parent: target, Node: fill}, []string{target, target + "/" + slotID}, nil
}

func countNodeSpecBounded(node paperedit.NodeSpec, count *int, limit int) bool {
	(*count)++
	if *count > limit {
		return false
	}
	for _, child := range node.Children {
		if !countNodeSpecBounded(child, count, limit) {
			return false
		}
	}
	return true
}

func componentReference(use *paperlang.Node) (string, error) {
	value := ""
	for _, member := range use.Members {
		if member.Property == nil || member.Property.Name != "component" {
			continue
		}
		if value != "" || member.Property.Value.StringValue == nil {
			return "", workspaceError("AMBIGUOUS_COMPONENT", "component use has an ambiguous component reference", paperedit.ErrInvalidOperation)
		}
		value = *member.Property.Value.StringValue
	}
	if value == "" {
		return "", workspaceError("INVALID_SLOT_FILL", "component use has no readable component reference", paperedit.ErrInvalidOperation)
	}
	return value, nil
}

func uniqueComponentDefinition(root *paperlang.Node, id string) (*paperlang.Node, error) {
	var found *paperlang.Node
	if root != nil {
		for _, member := range root.Members {
			if member.Node == nil || member.Node.Kind != paperlang.NodeComponent || member.Node.ID != id {
				continue
			}
			if found != nil {
				return nil, workspaceError("AMBIGUOUS_COMPONENT", "component reference resolves to more than one definition", paperedit.ErrInvalidOperation)
			}
			found = member.Node
		}
	}
	if found == nil {
		return nil, workspaceError("INVALID_COMPONENT", "referenced component definition was not found", paperedit.ErrInvalidOperation)
	}
	return found, nil
}

func uniqueComponentSlot(component *paperlang.Node, id string) (*paperlang.Node, error) {
	var found *paperlang.Node
	for _, member := range component.Members {
		if member.Node == nil || member.Node.Kind != paperlang.NodeSlot || member.Node.ID != id {
			continue
		}
		if found != nil {
			return nil, workspaceError("AMBIGUOUS_SLOT", "component declares the slot more than once", paperedit.ErrInvalidOperation)
		}
		found = member.Node
	}
	if found == nil {
		return nil, workspaceError("INVALID_SLOT_FILL", "component does not declare the requested slot", paperedit.ErrInvalidOperation)
	}
	return found, nil
}

func slotTypeForMutation(slot *paperlang.Node) (string, error) {
	slotType := "blocks"
	seen := false
	for _, member := range slot.Members {
		if member.Property == nil || member.Property.Name != "type" {
			continue
		}
		if seen || member.Property.Value.StringValue == nil {
			return "", workspaceError("AMBIGUOUS_SLOT", "slot has an ambiguous type contract", paperedit.ErrInvalidOperation)
		}
		seen = true
		slotType = strings.ToLower(strings.TrimSpace(*member.Property.Value.StringValue))
	}
	if slotType != "blocks" && slotType != "text" && slotType != "list" && slotType != "row-column" {
		return "", workspaceError("SLOT_TYPE", "slot declares an unsupported type", paperedit.ErrInvalidOperation)
	}
	return slotType, nil
}

func slotAcceptsMutation(slotType string, kind paperlang.NodeKind) bool {
	switch slotType {
	case "blocks":
		return kind == paperlang.NodeText || kind == paperlang.NodeParagraph || kind == paperlang.NodeHeading ||
			kind == paperlang.NodeList || kind == paperlang.NodePageBreak || kind == paperlang.NodeRow ||
			kind == paperlang.NodeColumn || kind == paperlang.NodeUse
	case "text":
		return kind == paperlang.NodeText || kind == paperlang.NodeParagraph || kind == paperlang.NodeHeading
	case "list":
		return kind == paperlang.NodeList
	case "row-column":
		return kind == paperlang.NodeRow || kind == paperlang.NodeColumn
	default:
		return false
	}
}

func cloneNodeSpecsForMutation(source []paperedit.NodeSpec) []paperedit.NodeSpec {
	result := make([]paperedit.NodeSpec, len(source))
	for i, node := range source {
		result[i] = cloneNodeSpec(node)
	}
	return result
}
