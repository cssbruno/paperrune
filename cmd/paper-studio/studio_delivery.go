// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cssbruno/paperrune/document"
)

type studioDeliveryResponse struct {
	FormatVersion  uint16 `json:"format_version"`
	Revision       string `json:"revision"`
	SourceRevision string `json:"source_revision"`
	PlanHash       string `json:"plan_hash,omitempty"`
	Scenario       string `json:"scenario,omitempty"`
	Preflight      struct {
		Status     string `json:"status"`
		IssueCount int    `json:"issue_count"`
		PageCount  int    `json:"page_count"`
		Failure    string `json:"failure,omitempty"`
	} `json:"preflight"`
	Export struct {
		Status   string `json:"status"`
		Endpoint string `json:"endpoint,omitempty"`
		Failure  string `json:"failure,omitempty"`
	} `json:"export"`
	Publish struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	} `json:"publish"`
}

func renderStudioTaggedPDF(ctx context.Context, plan document.PaperPlan) ([]byte, error) {
	pdf, err := document.NewDocument(document.WithDeterministicOutput())
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := pdf.WritePaperPlan(plan)
	if err != nil {
		return nil, err
	}
	if !result.OK() {
		return nil, errors.New("paper-studio: current plan could not be rendered")
	}
	var output bytes.Buffer
	if err := pdf.OutputWithOptionsContext(ctx, &output, document.OutputOptions{Deterministic: true}); err != nil {
		return nil, fmt.Errorf("paper-studio: generate PDF: %w", err)
	}
	return output.Bytes(), nil
}

func (s *studioServer) handleDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	snapshot, err := s.current(ctx, r.URL.Query().Get("scenario"))
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if r.URL.Query().Get("revision") != snapshot.revision {
		writeStudioError(w, http.StatusConflict, errors.New("paper-studio: stale delivery revision"))
		return
	}
	response := buildStudioDeliveryResponse(snapshot)
	if response.Preflight.Status == "ready" {
		response.Export.Status = "ready"
	} else {
		response.Export.Status = "blocked"
		response.Export.Failure = "export requires a valid current plan"
	}
	writeStudioJSON(w, http.StatusOK, response)
}

func buildStudioDeliveryResponse(snapshot *studioSnapshot) studioDeliveryResponse {
	result := studioDeliveryResponse{FormatVersion: 1, Revision: snapshot.revision,
		SourceRevision: studioSnapshotSourceRevision(snapshot), PlanHash: snapshot.plan.Hash(), Scenario: snapshot.scenario}
	result.Preflight.PageCount = snapshot.pages
	result.Preflight.IssueCount = len(snapshot.diagnostics)
	if snapshot.pages == 0 {
		result.Preflight.Status = "unavailable"
		result.Preflight.Failure = "a valid plan is required"
	} else if len(snapshot.diagnostics) != 0 {
		result.Preflight.Status = "blocked"
		result.Preflight.Failure = "current source has diagnostics"
	} else {
		result.Preflight.Status = "ready"
	}
	result.Export.Status = "pending"
	result.Publish.Status = "separate_authorized_capability"
	result.Publish.Reason = "publish is never implied by local export"
	return result
}

func (s *studioServer) handleExportPDF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), studioAPITimeout)
	defer cancel()
	snapshot, err := s.current(ctx, r.URL.Query().Get("scenario"))
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	if snapshot.pages == 0 || r.URL.Query().Get("revision") != snapshot.revision {
		writeStudioError(w, http.StatusConflict, errors.New("paper-studio: export requires the exact current plan revision"))
		return
	}
	if len(snapshot.diagnostics) != 0 {
		writeStudioError(w, http.StatusUnprocessableEntity, errors.New("paper-studio: export is blocked by current diagnostics"))
		return
	}
	pdf, err := renderStudioTaggedPDF(ctx, snapshot.plan)
	if err != nil {
		writeStudioError(w, http.StatusUnprocessableEntity, err)
		return
	}
	name := strings.TrimSuffix(filepath.Base(snapshot.file), filepath.Ext(snapshot.file)) + ".pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}
