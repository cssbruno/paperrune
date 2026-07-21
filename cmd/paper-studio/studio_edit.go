// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/cssbruno/paperrune/internal/papercompile"
	"github.com/cssbruno/paperrune/internal/paperd"
	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

const (
	studioEditFieldLimit = 256
	studioEditTextLimit  = 1 << 20
)

type studioTextRun struct {
	Target string `json:"target"`
	Text   string `json:"text"`
}

// studioEditRequest is deliberately not paperd's wire representation. The
// browser supplies review facts and a closed semantic intent; all opaque edit,
// candidate, revision, and authority handles remain inside the server.
type studioEditRequest struct {
	SourceRevision string                     `json:"source_revision"`
	PlanRevision   string                     `json:"plan_revision"`
	Scenario       string                     `json:"scenario,omitempty"`
	Operation      string                     `json:"operation"`
	Target         string                     `json:"target"`
	Property       string                     `json:"property"`
	Points         *float64                   `json:"points,omitempty"`
	Length         string                     `json:"length,omitempty"`
	Number         *float64                   `json:"number,omitempty"`
	Count          *uint32                    `json:"count,omitempty"`
	Width          *float64                   `json:"width_points,omitempty"`
	Height         *float64                   `json:"height_points,omitempty"`
	Color          string                     `json:"color,omitempty"`
	Kind           string                     `json:"kind,omitempty"`
	Weight         *uint32                    `json:"weight,omitempty"`
	Text           string                     `json:"text,omitempty"`
	Runs           []studioTextRun            `json:"runs,omitempty"`
	Bool           *bool                      `json:"bool,omitempty"`
	Split          string                     `json:"split,omitempty"`
	Path           string                     `json:"path,omitempty"`
	Required       *bool                      `json:"required,omitempty"`
	Format         string                     `json:"format,omitempty"`
	FormatLocale   string                     `json:"format_locale,omitempty"`
	FormatCurrency string                     `json:"format_currency,omitempty"`
	MinFraction    *uint32                    `json:"format_min_fraction,omitempty"`
	MaxFraction    *uint32                    `json:"format_max_fraction,omitempty"`
	Template       string                     `json:"template,omitempty"`
	Component      string                     `json:"component,omitempty"`
	ImportPath     string                     `json:"import_path,omitempty"`
	ID             string                     `json:"id,omitempty"`
	NewParent      string                     `json:"new_parent,omitempty"`
	Schema         string                     `json:"schema,omitempty"`
	Preset         string                     `json:"preset,omitempty"`
	Cases          []studioScenarioMatrixCase `json:"cases,omitempty"`
	Reset          bool                       `json:"reset,omitempty"`
}

type studioScenarioMatrixCase struct {
	Name   string `json:"name"`
	Preset string `json:"preset"`
}

func studioPageNumberProperty(property string) bool {
	switch property {
	case "page-numbers", "page-number-format", "page-total-alias", "page-number-align", "page-number-position", "page-number-hide-first", "page-number-start":
		return true
	default:
		return false
	}
}

type studioEditAuthorization struct {
	Actor   string                       `json:"actor"`
	Allowed bool                         `json:"allowed"`
	Effects []paperd.AuthorizationEffect `json:"effects"`
}

type studioEditResponse struct {
	OK                   bool                    `json:"ok"`
	Operation            string                  `json:"operation"`
	Target               string                  `json:"target"`
	Property             string                  `json:"property"`
	BeforeSourceRevision string                  `json:"before_source_revision"`
	SourceRevision       string                  `json:"source_revision"`
	BeforePlanRevision   string                  `json:"before_plan_revision"`
	PlanRevision         string                  `json:"plan_revision"`
	Applied              bool                    `json:"applied"`
	PatchCount           int                     `json:"patch_count"`
	Scenario             string                  `json:"scenario"`
	Authorization        studioEditAuthorization `json:"authorization"`
}

func studioSourceRevision(source string) string {
	return string(paperedit.SourceRevision(source))
}

func studioSnapshotSourceRevision(snapshot *studioSnapshot) string {
	if snapshot != nil && snapshot.sourceRevision != "" {
		return snapshot.sourceRevision
	}
	if snapshot == nil {
		return ""
	}
	return studioSourceRevision(snapshot.source)
}

func (s *studioServer) handleEdit(w http.ResponseWriter, r *http.Request) {
	s.handleStudioEdit(w, r, true)
}

func (s *studioServer) handleValidateEdit(w http.ResponseWriter, r *http.Request) {
	s.handleStudioEdit(w, r, false)
}

func (s *studioServer) handleStudioEdit(w http.ResponseWriter, r *http.Request, commit bool) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var request studioEditRequest
	if err := decodeStudioJSON(r, &request); err != nil {
		writeStudioError(w, http.StatusBadRequest, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()

	if commit {
		s.editMu.Lock()
		defer s.editMu.Unlock()
	} else {
		s.editMu.RLock()
		defer s.editMu.RUnlock()
	}
	result, err := s.applyStudioEdit(ctx, request, commit)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, paperd.ErrRevisionConflict) || errors.Is(err, errStudioStaleEdit) {
			status = http.StatusConflict
		} else if errors.Is(err, errStudioInvalidEdit) || errors.Is(err, paperd.ErrInvalidQuery) {
			status = http.StatusBadRequest
		}
		writeStudioError(w, status, err)
		return
	}
	writeStudioJSON(w, http.StatusOK, result)
}

var (
	errStudioInvalidEdit = errors.New("paper-studio: invalid edit request")
	errStudioStaleEdit   = errors.New("paper-studio: stale edit revision")
)

func normalizeStudioScenario(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "@")
}

func (s *studioServer) applyStudioEdit(ctx context.Context, request studioEditRequest, commit bool) (studioEditResponse, error) {
	if err := validateStudioEditRequest(request); err != nil {
		return studioEditResponse{}, err
	}
	snapshot, err := s.current(ctx, request.Scenario)
	if err != nil {
		return studioEditResponse{}, err
	}
	fontRepair := request.Operation == "text" && request.Property == "font"
	if request.SourceRevision != studioSnapshotSourceRevision(snapshot) || request.PlanRevision != snapshot.revision || (snapshot.pages == 0 && request.Operation != "template" && request.Operation != "import" && request.Operation != "schema" && request.Operation != "schema-object" && request.Operation != "schema-field" && request.Operation != "scenario" && request.Operation != "scenario-create" && request.Operation != "scenario-matrix" && request.Operation != "scenario-value" && request.Operation != "node" && !fontRepair) {
		return studioEditResponse{}, fmt.Errorf("%w: source or plan changed after selection", errStudioStaleEdit)
	}

	parsed := paperlang.Parse(snapshot.file, snapshot.source)
	target, parent := studioSourceTarget(parsed.AST.Root, request.Target)
	if target == nil {
		return studioEditResponse{}, fmt.Errorf("%w: target must resolve to one exact authored node", errStudioInvalidEdit)
	}
	directTargets := []string{request.Target}
	operation := paperd.MutationSetBoxProperty
	if request.Reset {
		operation = paperd.MutationResetProperty
	}
	switch request.Operation {
	case "content":
		if request.Property == "runs" {
			operation = paperd.MutationSetRichText
			for _, run := range request.Runs {
				directTargets = append(directTargets, run.Target)
			}
		} else {
			operation = paperd.MutationSetLiteral
			if target.Kind == paperlang.NodeParagraph || target.Kind == paperlang.NodeHeading {
				for _, member := range target.Members {
					if member.Node != nil && member.Node.Kind == paperlang.NodeText && member.Node.ID != "" {
						directTargets = append(directTargets, member.Node.ID)
					}
				}
			}
		}
	case "document":
		if !request.Reset {
			operation = paperd.MutationSetDocumentProperty
		}
	case "appearance":
		if !request.Reset {
			operation = paperd.MutationSetAppearance
		}
	case "condition":
		if !request.Reset {
			operation = paperd.MutationSetCondition
		}
	case "text":
		if !request.Reset {
			operation = paperd.MutationSetTextProperty
		}
	case "list":
		if !request.Reset {
			operation = paperd.MutationSetListProperty
		}
	case "layout-item":
		if !request.Reset {
			operation = paperd.MutationSetLayoutItem
		}
		if parent == nil || parent.ID == "" {
			return studioEditResponse{}, fmt.Errorf("%w: layout item requires an addressed row or column", errStudioInvalidEdit)
		}
		directTargets = append(directTargets, parent.ID)
	case "layout-container":
		if !request.Reset {
			operation = paperd.MutationSetLayoutContainer
		}
	case "image":
		if !request.Reset {
			operation = paperd.MutationSetImageProperty
		}
	case "table":
		if !request.Reset {
			operation = paperd.MutationSetTableProperty
		}
		table := studioTableAncestor(parsed.AST.Root, request.Target)
		if table == nil || table.ID == "" {
			return studioEditResponse{}, fmt.Errorf("%w: table target requires an addressed governing table", errStudioInvalidEdit)
		}
		if table.ID != request.Target {
			directTargets = append(directTargets, table.ID)
		}
	case "page":
		if request.Reset {
		} else if studioPageNumberProperty(request.Property) {
			operation = paperd.MutationSetPageNumbering
		} else {
			operation = paperd.MutationSetPageMargin
		}
	case "page-size":
		operation = paperd.MutationSetPageSize
	case "canvas":
		if !request.Reset {
			operation = paperd.MutationSetCanvasItem
		}
		if parent == nil || parent.Kind != paperlang.NodeCanvas || parent.ID == "" {
			return studioEditResponse{}, fmt.Errorf("%w: canvas target requires an addressed governing canvas", errStudioInvalidEdit)
		}
		directTargets = append(directTargets, parent.ID)
	case "canvas-container":
		if !request.Reset {
			operation = paperd.MutationSetCanvasProperty
		}
	case "region":
		if !request.Reset {
			operation = paperd.MutationSetPageRegion
		}
		if parent == nil || parent.Kind != paperlang.NodePage || parent.ID == "" || (target.Kind != paperlang.NodeHeader && target.Kind != paperlang.NodeFooter) {
			return studioEditResponse{}, fmt.Errorf("%w: region target requires an authored header/footer and governing page", errStudioInvalidEdit)
		}
		directTargets = append(directTargets, parent.ID)
	case "flow":
		operation = paperd.MutationMoveNode
		destination, _ := studioSourceTarget(parsed.AST.Root, request.NewParent)
		if destination == nil || !studioFlowParentKind(destination.Kind) || request.NewParent == request.Target || !studioFlowChildKind(destination.Kind, target.Kind) {
			return studioEditResponse{}, fmt.Errorf("%w: flow move requires an exact compatible body, row, or column destination", errStudioInvalidEdit)
		}
		directTargets = append(directTargets, request.NewParent)
	case "binding":
		if !request.Reset {
			operation = paperd.MutationSetBinding
		}
	case "template":
		operation = paperd.MutationInsertTemplate
	case "import":
		operation = paperd.MutationInsertTemplate
	case "schema":
		operation = paperd.MutationInsertTemplate
	case "schema-object":
		operation = paperd.MutationInsertTemplate
	case "scenario-create":
		operation = paperd.MutationCreateScenario
	case "scenario-matrix":
		operation = paperd.MutationCreateScenarioMatrix
	case "schema-field":
		operation = paperd.MutationAddSchemaField
	case "scenario-value":
		operation = paperd.MutationSetScenarioValue
	case "scenario":
		operation = paperd.MutationManageScenario
	case "node":
		operation = paperd.MutationManageNode
	}

	s.mu.Lock()
	assetResources := make([]papercompile.AssetResource, len(s.assets))
	for i, asset := range s.assets {
		assetResources[i] = papercompile.AssetResource{Name: asset.Name, MediaType: asset.MediaType, Digest: asset.Digest, Data: append([]byte(nil), asset.Data...), Family: asset.Family, Style: asset.Style, Weight: asset.Weight, License: asset.License}
	}
	s.mu.Unlock()
	workspace, err := paperd.NewWorkspaceWithOptions(paperd.WorkspaceOptions{
		DisclosureDomain:         paperd.DisclosureRestricted,
		RequireMutationAuthority: true,
		AssetResources:           assetResources,
		ImportResolver:           papercompile.ImportResolver(studioFileImportResolver(snapshot.file)),
	})
	if err != nil {
		return studioEditResponse{}, err
	}
	created, err := workspace.PaperCreate(paperd.PaperCreateRequest{File: snapshot.file, Source: snapshot.source})
	if err != nil {
		return studioEditResponse{}, err
	}
	opened, err := workspace.PaperOpen(paperd.PaperOpenRequest{
		Candidate: created.Candidate.Handle, Revision: created.Revision.Handle,
		ExpectedDigest: created.Revision.Revision, Mode: paperd.CapabilityEdit,
		DisclosureDomain: paperd.DisclosureRestricted,
	})
	if err != nil {
		return studioEditResponse{}, err
	}
	authority, err := workspace.GrantMutationAuthority(paperd.MutationAuthorityGrant{
		Open: opened.Handle, Actor: "studio:local-user", Operations: []paperd.MutationOperation{operation},
		NodeScopes: directTargets,
	})
	if err != nil {
		return studioEditResponse{}, err
	}
	fingerprint, instance, err := studioTargetPrecondition(snapshot.file, snapshot.source, request.Target)
	if err != nil {
		return studioEditResponse{}, fmt.Errorf("%w: target precondition: %w", errStudioInvalidEdit, err)
	}
	guard := paperd.PaperMutationGuard{
		Open: opened.Handle, Authority: authority.Handle, Candidate: created.Candidate.Handle,
		ExpectedHead: created.Revision.Handle, ExpectedDigest: created.Revision.Revision,
		Target: request.Target, ExpectedFingerprint: fingerprint, ExpectedInstance: instance,
		IdempotencyKey: studioEditIdempotencyKey(request),
	}
	parentID := ""
	if parent != nil {
		parentID = parent.ID
	}
	additionalTargets := []string{}
	if request.Operation == "layout-item" || request.Operation == "canvas" || request.Operation == "region" {
		if parentID == "" {
			return studioEditResponse{}, fmt.Errorf("%w: operation requires an addressed governing parent", errStudioInvalidEdit)
		}
	}
	if request.Operation == "layout-item" {
		additionalTargets = append(additionalTargets, parentID)
	} else if request.Operation == "canvas" {
		additionalTargets = append(additionalTargets, parentID)
	} else if request.Operation == "region" {
		additionalTargets = append(additionalTargets, parentID)
	} else if request.Operation == "flow" {
		additionalTargets = append(additionalTargets, request.NewParent)
	} else if request.Operation == "table" && len(directTargets) == 2 {
		additionalTargets = append(additionalTargets, directTargets[1])
	} else if request.Operation == "content" && len(directTargets) > 1 {
		additionalTargets = append(additionalTargets, directTargets[1:]...)
	}
	for _, additionalTarget := range additionalTargets {
		parentFingerprint, parentInstance, preconditionErr := studioTargetPrecondition(snapshot.file, snapshot.source, additionalTarget)
		if preconditionErr != nil {
			return studioEditResponse{}, fmt.Errorf("%w: parent precondition: %w", errStudioInvalidEdit, preconditionErr)
		}
		guard.TargetPreconditions = append(guard.TargetPreconditions, paperedit.TargetPrecondition{
			Target: additionalTarget, ExpectedFingerprint: parentFingerprint, ExpectedInstance: parentInstance,
		})
	}

	mutation, err := applyStudioSemanticMutation(workspace, guard, request)
	if err != nil {
		return studioEditResponse{}, err
	}
	if mutation.Edit.Diff == nil || len(mutation.Edit.Diff.Patches) == 0 || len(mutation.Edit.Diff.Patches) > 7 {
		return studioEditResponse{}, errors.New("paper-studio: semantic handle did not produce a bounded minimal source patch set")
	}
	if !commit {
		return studioEditResponse{OK: true, Operation: request.Operation, Target: request.Target, Property: request.Property,
			BeforeSourceRevision: request.SourceRevision, SourceRevision: studioSourceRevision(mutation.Edit.Source),
			BeforePlanRevision: request.PlanRevision, PlanRevision: request.PlanRevision, Scenario: request.Scenario,
			Applied: mutation.Edit.Applied, PatchCount: len(mutation.Edit.Diff.Patches)}, nil
	}
	if err := writeStudioSourceCAS(snapshot.file, snapshot.sourceHash, mutation.Edit.Source); err != nil {
		return studioEditResponse{}, err
	}
	journal, err := s.studioHistory(snapshot)
	if err != nil {
		return studioEditResponse{}, err
	}
	if _, _, err = journal.ApplySource(paperedit.SourceJournalRequest{ExpectedRevision: paperedit.SourceRevision(snapshot.source), Group: studioEditHistoryLabel(request), Source: mutation.Edit.Source}); err != nil {
		return studioEditResponse{}, err
	}
	afterScenario := request.Scenario
	if request.Operation == "scenario" && normalizeStudioScenario(request.Scenario) == normalizeStudioScenario(request.Target) {
		switch request.Property {
		case "rename":
			afterScenario = request.ID
		case "delete":
			afterScenario = ""
		}
	}
	after, err := s.current(ctx, afterScenario)
	if err != nil {
		return studioEditResponse{}, err
	}
	return studioEditResponse{
		OK: true, Operation: request.Operation, Target: request.Target, Property: request.Property,
		BeforeSourceRevision: request.SourceRevision, SourceRevision: studioSnapshotSourceRevision(after),
		BeforePlanRevision: request.PlanRevision, PlanRevision: after.revision,
		Scenario: afterScenario,
		Applied:  mutation.Edit.Applied, PatchCount: len(mutation.Edit.Diff.Patches),
		Authorization: studioEditAuthorization{Actor: mutation.Authorization.Actor, Allowed: mutation.Authorization.Allowed,
			Effects: append([]paperd.AuthorizationEffect(nil), mutation.Authorization.Effects...)},
	}, nil
}

type studioHistoryRequest struct {
	SourceRevision string `json:"source_revision"`
	PlanRevision   string `json:"plan_revision"`
	Scenario       string `json:"scenario,omitempty"`
	Action         string `json:"action"`
}

type studioHistoryResponse struct {
	CanUndo        bool   `json:"can_undo"`
	CanRedo        bool   `json:"can_redo"`
	UndoCount      int    `json:"undo_count"`
	RedoCount      int    `json:"redo_count"`
	UndoLabel      string `json:"undo_label,omitempty"`
	RedoLabel      string `json:"redo_label,omitempty"`
	SourceRevision string `json:"source_revision"`
	PlanRevision   string `json:"plan_revision"`
}

func (s *studioServer) studioHistory(snapshot *studioSnapshot) (*paperedit.Journal, error) {
	revision := paperedit.SourceRevision(snapshot.source)
	if s.history != nil {
		_, current := s.history.Source()
		if current == revision {
			return s.history, nil
		}
	}
	journal, err := paperedit.NewJournal(snapshot.file, snapshot.source, paperedit.DefaultJournalLimits())
	if err != nil {
		return nil, err
	}
	s.history = journal
	return journal, nil
}

func studioHistoryResult(snapshot *studioSnapshot, journal *paperedit.Journal) studioHistoryResponse {
	status := journal.Snapshot()
	return studioHistoryResponse{CanUndo: status.CanUndo, CanRedo: status.CanRedo, UndoCount: status.UndoCount, RedoCount: status.RedoCount,
		UndoLabel: status.UndoLabel, RedoLabel: status.RedoLabel,
		SourceRevision: studioSnapshotSourceRevision(snapshot), PlanRevision: snapshot.revision}
}

func studioEditHistoryLabel(request studioEditRequest) string {
	action := "Edit"
	if request.Reset {
		action = "Reset"
	} else if request.Operation == "template" || request.Operation == "import" || request.Operation == "schema" || request.Operation == "schema-object" || request.Operation == "schema-field" {
		action = "Create"
	} else if request.Operation == "flow" {
		action = "Move"
	} else if request.Operation == "node" {
		if request.Property == "delete" {
			action = "Delete"
		} else {
			action = "Rename"
		}
	}
	detail := strings.ReplaceAll(request.Operation, "-", " ")
	if request.Property != "" {
		detail = strings.ReplaceAll(request.Property, "-", " ")
	}
	return fmt.Sprintf("%s %s on %s", action, detail, request.Target)
}

func (s *studioServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	s.editMu.Lock()
	defer s.editMu.Unlock()
	if r.Method == http.MethodGet {
		snapshot, err := s.current(ctx, r.URL.Query().Get("scenario"))
		if err != nil {
			writeStudioError(w, http.StatusUnprocessableEntity, err)
			return
		}
		journal, err := s.studioHistory(snapshot)
		if err != nil {
			writeStudioError(w, http.StatusUnprocessableEntity, err)
			return
		}
		writeStudioJSON(w, http.StatusOK, studioHistoryResult(snapshot, journal))
		return
	}
	var request studioHistoryRequest
	if err := decodeStudioJSON(r, &request); err != nil {
		writeStudioError(w, http.StatusBadRequest, err)
		return
	}
	snapshot, err := s.current(ctx, request.Scenario)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if request.SourceRevision != studioSnapshotSourceRevision(snapshot) || request.PlanRevision != snapshot.revision {
		writeStudioError(w, http.StatusConflict, fmt.Errorf("%w: source or plan changed before history action", errStudioStaleEdit))
		return
	}
	journal, err := s.studioHistory(snapshot)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	expected := paperedit.SourceRevision(snapshot.source)
	switch request.Action {
	case "undo":
		_, _, err = journal.Undo(expected)
	case "redo":
		_, _, err = journal.Redo(expected)
	default:
		writeStudioError(w, http.StatusBadRequest, fmt.Errorf("%w: history action must be undo or redo", errStudioInvalidEdit))
		return
	}
	if err != nil {
		writeStudioError(w, http.StatusConflict, err)
		return
	}
	source, revision := journal.Source()
	if err = writeStudioSourceCAS(snapshot.file, snapshot.sourceHash, source); err != nil {
		switch request.Action {
		case "undo":
			_, _, _ = journal.Redo(revision)
		case "redo":
			_, _, _ = journal.Undo(revision)
		}
		writeStudioError(w, http.StatusConflict, err)
		return
	}
	after, err := s.current(ctx, request.Scenario)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	writeStudioJSON(w, http.StatusOK, studioHistoryResult(after, journal))
}

func validateStudioEditRequest(request studioEditRequest) error {
	fields := []string{request.SourceRevision, request.PlanRevision, request.Scenario, request.Operation, request.Target, request.Property, request.Length, request.Color, request.Kind, request.Split, request.Path, request.Format, request.FormatLocale, request.FormatCurrency, request.Template, request.Component, request.ImportPath, request.ID, request.NewParent, request.Schema, request.Preset}
	for _, field := range fields {
		if len(field) > studioEditFieldLimit || !utf8.ValidString(field) {
			return fmt.Errorf("%w: edit field exceeds its bound", errStudioInvalidEdit)
		}
	}
	if len(request.Text) > studioEditTextLimit || !utf8.ValidString(request.Text) {
		return fmt.Errorf("%w: edit text exceeds its bound", errStudioInvalidEdit)
	}
	if len(request.Runs) > 7 {
		return fmt.Errorf("%w: rich text is limited to seven addressed runs per edit", errStudioInvalidEdit)
	}
	totalRunBytes := 0
	seenRuns := make(map[string]struct{}, len(request.Runs))
	for _, run := range request.Runs {
		totalRunBytes += len(run.Text)
		if len(run.Target) > studioEditFieldLimit || !utf8.ValidString(run.Target) || !utf8.ValidString(run.Text) || run.Target == "" || run.Target[0] != '@' {
			return fmt.Errorf("%w: rich-text runs require bounded readable targets and valid text", errStudioInvalidEdit)
		}
		if _, duplicate := seenRuns[run.Target]; duplicate {
			return fmt.Errorf("%w: rich-text run targets must be unique", errStudioInvalidEdit)
		}
		seenRuns[run.Target] = struct{}{}
	}
	if totalRunBytes > studioEditTextLimit {
		return fmt.Errorf("%w: rich-text content exceeds its bound", errStudioInvalidEdit)
	}
	if request.SourceRevision == "" || request.PlanRevision == "" || request.Target == "" || request.Target[0] != '@' || strings.ContainsAny(request.Target, " \t\r\n") {
		return fmt.Errorf("%w: exact revisions and readable target are required", errStudioInvalidEdit)
	}
	if request.Points != nil && (math.IsNaN(*request.Points) || math.IsInf(*request.Points, 0)) {
		return fmt.Errorf("%w: points must be finite", errStudioInvalidEdit)
	}
	if request.Number != nil && (math.IsNaN(*request.Number) || math.IsInf(*request.Number, 0)) {
		return fmt.Errorf("%w: number must be finite", errStudioInvalidEdit)
	}
	if request.MinFraction != nil && *request.MinFraction > 18 || request.MaxFraction != nil && *request.MaxFraction > 18 {
		return fmt.Errorf("%w: binding fraction digits must be between 0 and 18", errStudioInvalidEdit)
	}
	if request.Width != nil && (math.IsNaN(*request.Width) || math.IsInf(*request.Width, 0)) || request.Height != nil && (math.IsNaN(*request.Height) || math.IsInf(*request.Height, 0)) {
		return fmt.Errorf("%w: page dimensions must be finite", errStudioInvalidEdit)
	}
	if request.Operation != "content" && request.Operation != "document" && request.Operation != "appearance" && request.Operation != "condition" && request.Operation != "box" && request.Operation != "text" && request.Operation != "list" && request.Operation != "layout-item" && request.Operation != "layout-container" && request.Operation != "image" && request.Operation != "table" && request.Operation != "page" && request.Operation != "page-size" && request.Operation != "canvas" && request.Operation != "canvas-container" && request.Operation != "region" && request.Operation != "binding" && request.Operation != "template" && request.Operation != "import" && request.Operation != "schema" && request.Operation != "schema-object" && request.Operation != "schema-field" && request.Operation != "scenario-create" && request.Operation != "scenario-matrix" && request.Operation != "scenario-value" && request.Operation != "scenario" && request.Operation != "node" && request.Operation != "flow" {
		return fmt.Errorf("%w: operation is outside the closed Studio authoring vocabulary", errStudioInvalidEdit)
	}
	if request.Operation == "content" {
		if request.Property != "text" && request.Property != "runs" {
			return fmt.Errorf("%w: content edit must target text or addressed runs", errStudioInvalidEdit)
		}
		if request.Reset || (request.Property == "runs" && len(request.Runs) == 0) || (request.Property == "text" && len(request.Runs) != 0) {
			return fmt.Errorf("%w: content payload does not match its representation", errStudioInvalidEdit)
		}
	}
	if request.Operation == "scenario" {
		if request.Property != "rename" && request.Property != "delete" {
			return fmt.Errorf("%w: scenario action must be rename or delete", errStudioInvalidEdit)
		}
		if request.Property == "rename" && request.ID == "" {
			return fmt.Errorf("%w: scenario rename requires a new readable @id", errStudioInvalidEdit)
		}
		if request.Property == "delete" && request.ID != "" {
			return fmt.Errorf("%w: scenario delete cannot carry a replacement @id", errStudioInvalidEdit)
		}
	}
	if request.Operation == "node" {
		if request.Property != "rename" && request.Property != "delete" {
			return fmt.Errorf("%w: node action must be rename or delete", errStudioInvalidEdit)
		}
		if request.Property == "rename" && request.ID == "" {
			return fmt.Errorf("%w: node rename requires a new readable @id", errStudioInvalidEdit)
		}
		if request.Property == "delete" && request.ID != "" {
			return fmt.Errorf("%w: node delete cannot carry a replacement @id", errStudioInvalidEdit)
		}
	}
	if request.Operation == "scenario-value" && request.Path == "" {
		return fmt.Errorf("%w: scenario value operation requires a fixture path", errStudioInvalidEdit)
	}
	if request.Operation == "scenario-matrix" && (len(request.Cases) == 0 || len(request.Cases) > 16) {
		return fmt.Errorf("%w: scenario matrix requires between one and sixteen cases", errStudioInvalidEdit)
	}
	if request.Operation == "flow" && (request.NewParent == "" || request.NewParent[0] != '@') {
		return fmt.Errorf("%w: flow operation requires a readable destination @id", errStudioInvalidEdit)
	}
	return nil
}

func applyStudioSemanticMutation(workspace *paperd.Workspace, guard paperd.PaperMutationGuard, request studioEditRequest) (paperd.PaperMutationResult, error) {
	if request.Reset {
		return workspace.PaperResetProperty(paperd.PaperResetPropertyRequest{Guard: guard, Category: request.Operation, Property: request.Property})
	}
	if request.Operation == "content" {
		if request.Property == "runs" {
			runs := make([]paperd.PaperRichTextRun, len(request.Runs))
			for index, run := range request.Runs {
				runs[index] = paperd.PaperRichTextRun{Target: run.Target, Text: run.Text}
			}
			return workspace.PaperSetRichText(paperd.PaperSetRichTextRequest{Guard: guard, Runs: runs})
		}
		return workspace.PaperSetLiteral(paperd.PaperSetLiteralRequest{Guard: guard, Text: request.Text})
	}
	if request.Operation == "document" {
		return workspace.PaperSetDocumentProperty(paperd.PaperSetDocumentPropertyRequest{Guard: guard, Property: paperd.PaperDocumentProperty(request.Property), Text: request.Text})
	}
	if request.Operation == "appearance" {
		return workspace.PaperSetAppearance(paperd.PaperSetAppearanceRequest{Guard: guard, Property: paperd.PaperAppearanceProperty(request.Property), Text: request.Text})
	}
	if request.Operation == "condition" {
		return workspace.PaperSetCondition(paperd.PaperSetConditionRequest{Guard: guard, Expression: request.Text})
	}
	if request.Operation == "binding" {
		return workspace.PaperSetBinding(paperd.PaperSetBindingRequest{
			Guard: guard, Path: request.Path, Required: request.Required,
			Format: request.Format, FormatLocale: request.FormatLocale, FormatCurrency: request.FormatCurrency,
			MinFractionDigits: request.MinFraction, MaxFractionDigits: request.MaxFraction,
		})
	}
	if request.Operation == "template" {
		return workspace.PaperInsertTemplate(paperd.PaperInsertTemplateRequest{Guard: guard, Template: request.Template, ID: request.ID, Component: request.Component, Preset: request.Preset, Path: request.Path})
	}
	if request.Operation == "import" {
		return workspace.PaperInsertTemplate(paperd.PaperInsertTemplateRequest{Guard: guard, Template: "import", ImportPath: request.ImportPath})
	}
	if request.Operation == "schema" {
		return workspace.PaperInsertTemplate(paperd.PaperInsertTemplateRequest{Guard: guard, Template: "schema", ID: request.ID})
	}
	if request.Operation == "schema-object" {
		return workspace.PaperInsertTemplate(paperd.PaperInsertTemplateRequest{Guard: guard, Template: "schema-object", ID: request.ID})
	}
	if request.Operation == "scenario-create" {
		return workspace.PaperCreateScenario(paperd.PaperCreateScenarioRequest{Guard: guard, Name: request.ID, Schema: request.Schema, Preset: request.Preset})
	}
	if request.Operation == "scenario-matrix" {
		cases := make([]paperd.PaperScenarioMatrixCase, len(request.Cases))
		for index, matrixCase := range request.Cases {
			cases[index] = paperd.PaperScenarioMatrixCase{Name: matrixCase.Name, Preset: matrixCase.Preset}
		}
		return workspace.PaperCreateScenarioMatrix(paperd.PaperCreateScenarioMatrixRequest{Guard: guard, Schema: request.Schema, Cases: cases})
	}
	if request.Operation == "schema-field" {
		maxItems := uint32(0)
		if request.Weight != nil {
			maxItems = *request.Weight
		}
		return workspace.PaperAddSchemaField(paperd.PaperAddSchemaFieldRequest{Guard: guard, ID: request.ID, Type: request.Kind, ItemType: request.Text, MaxItems: maxItems})
	}
	if request.Operation == "scenario-value" {
		return workspace.PaperSetScenarioFixtureValue(paperd.PaperSetScenarioFixtureValueRequest{Guard: guard, Path: request.Path, Kind: request.Kind, Text: request.Text, Bool: request.Bool, Number: request.Number})
	}
	if request.Operation == "scenario" {
		return workspace.PaperManageScenario(paperd.PaperManageScenarioRequest{Guard: guard, Action: request.Property, NewName: request.ID})
	}
	if request.Operation == "node" {
		return workspace.PaperManageNode(paperd.PaperManageNodeRequest{Guard: guard, Action: request.Property, NewName: request.ID})
	}
	if request.Operation == "flow" {
		return workspace.PaperMoveNode(paperd.PaperMoveNodeRequest{Guard: guard, NewParent: request.NewParent})
	}
	if request.Operation == "box" {
		points := 0.0
		if request.Points != nil {
			points = *request.Points
		}
		return workspace.PaperSetBoxProperty(paperd.PaperSetBoxPropertyRequest{
			Guard: guard, Property: paperd.PaperBoxProperty(request.Property), Points: points, Color: request.Color,
		})
	}
	if request.Operation == "text" {
		points, boolean, count := 0.0, false, uint32(0)
		if request.Points != nil {
			points = *request.Points
		}
		if request.Bool != nil {
			boolean = *request.Bool
		}
		if request.Count != nil {
			count = *request.Count
		}
		return workspace.PaperSetTextProperty(paperd.PaperSetTextPropertyRequest{
			Guard: guard, Property: paperd.PaperTextProperty(request.Property), Text: request.Text,
			Points: points, Length: request.Length, Color: request.Color, Kind: request.Kind, Bool: boolean, Count: count,
		})
	}
	if request.Operation == "list" {
		boolean := false
		if request.Bool != nil {
			boolean = *request.Bool
		}
		return workspace.PaperSetListProperty(paperd.PaperSetListPropertyRequest{
			Guard: guard, Property: paperd.PaperListProperty(request.Property), Marker: request.Kind, Bool: boolean,
		})
	}
	if request.Operation == "image" {
		number, points, boolean := 0.0, 0.0, false
		if request.Number != nil {
			number = *request.Number
		}
		if request.Points != nil {
			points = *request.Points
		}
		if request.Bool != nil {
			boolean = *request.Bool
		}
		return workspace.PaperSetImageProperty(paperd.PaperSetImagePropertyRequest{
			Guard: guard, Property: paperd.PaperImageProperty(request.Property), Fit: request.Kind,
			Number: number, Points: points, Length: request.Length, Text: request.Text, Bool: boolean,
		})
	}
	if request.Operation == "table" {
		points, boolean, count := 0.0, false, uint32(0)
		if request.Points != nil {
			points = *request.Points
		}
		if request.Bool != nil {
			boolean = *request.Bool
		}
		if request.Count != nil {
			count = *request.Count
		}
		return workspace.PaperSetTableProperty(paperd.PaperSetTablePropertyRequest{
			Guard: guard, Property: paperd.PaperTableProperty(request.Property), Split: request.Split, Points: points, Length: request.Length,
			Text: request.Text, Kind: request.Kind, Bool: boolean, Count: count,
		})
	}
	if request.Operation == "page" {
		if studioPageNumberProperty(request.Property) {
			boolean := false
			if request.Bool != nil {
				boolean = *request.Bool
			}
			count := uint32(0)
			if request.Count != nil {
				count = *request.Count
			}
			return workspace.PaperSetPageNumbering(paperd.PaperSetPageNumberingRequest{Guard: guard, Property: paperd.PaperPageNumberingProperty(request.Property), Text: request.Text, Kind: request.Kind, Bool: boolean, Count: count})
		}
		points := 0.0
		if request.Points != nil {
			points = *request.Points
		}
		return workspace.PaperSetPageMargin(paperd.PaperSetPageMarginRequest{
			Guard: guard, Property: paperd.PaperPageMarginProperty(request.Property), Points: points,
		})
	}
	if request.Operation == "page-size" {
		if request.Width == nil || request.Height == nil {
			return paperd.PaperMutationResult{}, fmt.Errorf("%w: page size requires width and height", errStudioInvalidEdit)
		}
		return workspace.PaperSetPageSize(paperd.PaperSetPageSizeRequest{Guard: guard, WidthPoints: *request.Width, HeightPoints: *request.Height})
	}
	if request.Operation == "canvas" {
		points := 0.0
		if request.Points != nil {
			points = *request.Points
		}
		item := paperd.PaperSetCanvasItemRequest{Guard: guard, Property: request.Property}
		switch request.Property {
		case "width", "height":
			item.Points = points
		case "alt":
			item.Text = request.Text
		default:
			item.Reference, item.TargetAnchor, item.Offset = request.Text, paperd.PaperCanvasAnchorProperty(request.Kind), points
		}
		return workspace.PaperSetCanvasItem(item)
	}
	if request.Operation == "canvas-container" {
		points := 0.0
		if request.Points != nil {
			points = *request.Points
		}
		return workspace.PaperSetCanvasProperty(paperd.PaperSetCanvasPropertyRequest{Guard: guard, Property: paperd.PaperCanvasProperty(request.Property), Points: points, Kind: request.Kind})
	}
	if request.Operation == "region" {
		points, boolean := 0.0, false
		if request.Points != nil {
			points = *request.Points
		}
		if request.Bool != nil {
			boolean = *request.Bool
		}
		return workspace.PaperSetPageRegion(paperd.PaperSetPageRegionRequest{Guard: guard, Property: request.Property, Points: points, Color: request.Color, Bool: boolean})
	}
	if request.Operation == "layout-container" {
		points, boolean := 0.0, false
		if request.Points != nil {
			points = *request.Points
		}
		if request.Bool != nil {
			boolean = *request.Bool
		}
		return workspace.PaperSetLayoutContainer(paperd.PaperSetLayoutContainerRequest{
			Guard: guard, Property: paperd.PaperLayoutContainerProperty(request.Property), Points: points, Length: request.Length, Kind: request.Kind, Bool: boolean,
		})
	}
	points := 0.0
	if request.Points != nil {
		points = *request.Points
	}
	factor := 0.0
	if request.Number != nil {
		factor = *request.Number
	}
	return workspace.PaperSetLayoutItem(paperd.PaperSetLayoutItemRequest{
		Guard: guard, Property: paperd.PaperLayoutItemProperty(request.Property), Kind: request.Kind, Points: points, Length: request.Length, Factor: factor,
	})
}

func studioFlowParentKind(kind paperlang.NodeKind) bool {
	return kind == paperlang.NodeBody || kind == paperlang.NodeRow || kind == paperlang.NodeColumn
}

func studioFlowChildKind(parent, child paperlang.NodeKind) bool {
	switch parent {
	case paperlang.NodeBody:
		return child == paperlang.NodeHeading || child == paperlang.NodeParagraph || child == paperlang.NodeList ||
			child == paperlang.NodePageBreak || child == paperlang.NodeText || child == paperlang.NodeRow ||
			child == paperlang.NodeColumn || child == paperlang.NodeImage || child == paperlang.NodeTable ||
			child == paperlang.NodeCanvas || child == paperlang.NodeUse || child == paperlang.NodeRepeat || child == paperlang.NodeLoop
	case paperlang.NodeRow, paperlang.NodeColumn:
		return child == paperlang.NodeHeading || child == paperlang.NodeParagraph || child == paperlang.NodeUse
	default:
		return false
	}
}

func studioTableAncestor(root *paperlang.Node, target string) *paperlang.Node {
	var found, table *paperlang.Node
	var matches int
	var walk func(*paperlang.Node, *paperlang.Node)
	walk = func(node, governing *paperlang.Node) {
		if node == nil {
			return
		}
		if node.Kind == paperlang.NodeTable {
			governing = node
		}
		if node.ID == target {
			matches++
			found, table = node, governing
		}
		for _, member := range node.Members {
			walk(member.Node, governing)
		}
	}
	walk(root, nil)
	if matches != 1 || found == nil {
		return nil
	}
	return table
}

func studioSourceTarget(root *paperlang.Node, id string) (found, parent *paperlang.Node) {
	var matches int
	var walk func(*paperlang.Node, *paperlang.Node)
	walk = func(node, owner *paperlang.Node) {
		if node == nil {
			return
		}
		if node.ID == id {
			matches++
			found, parent = node, owner
		}
		for _, member := range node.Members {
			walk(member.Node, node)
		}
	}
	walk(root, nil)
	if matches != 1 {
		return nil, nil
	}
	return found, parent
}

func studioTargetPrecondition(file, source, target string) (paperedit.NodeFingerprint, string, error) {
	fingerprint, err := paperedit.FingerprintNode(file, source, target)
	if err != nil {
		return "", "", err
	}
	instance, err := paperedit.SourceInstance(file, source, target)
	return fingerprint, instance, err
}

func studioEditIdempotencyKey(request studioEditRequest) string {
	encoded, err := json.Marshal(request)
	if err != nil {
		encoded = []byte(fmt.Sprintf("%#v", request))
	}
	digest := sha256.Sum256(encoded)
	return "studio-edit-" + hex.EncodeToString(digest[:16])
}

func writeStudioSourceCAS(file string, expected [32]byte, source string) error {
	if isStudioPaperDocument(file) {
		return writeStudioPaperDocumentSourceCAS(file, expected, source)
	}
	_, actual, err := readStudioSource(file)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("%w: file changed before commit", errStudioStaleEdit)
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	directory := filepath.Dir(file)
	temporary, err := os.CreateTemp(directory, ".paper-studio-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(source); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	_, actual, err = readStudioSource(file)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("%w: file changed during commit", errStudioStaleEdit)
	}
	if err := os.Rename(temporaryName, file); err != nil {
		return err
	}
	if directoryHandle, openErr := os.Open(directory); openErr == nil { // #nosec G304 -- directory is the validated source file's parent.
		_ = directoryHandle.Sync()
		_ = directoryHandle.Close()
	}
	return nil
}
