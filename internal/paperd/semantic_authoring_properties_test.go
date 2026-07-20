// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"strings"
	"testing"
)

const authoringPropertiesFixture = `document @report:
  title: "Old title"
  language: "en"
  theme: "@print"
  style @body-style:
    font: "Helvetica"
  theme @print:
    token @ink:
      type: "color"
      value: "#112233"
  page @sheet:
    page-numbers: false
    body @body:
      paragraph @copy:
        text: "Copy"
      canvas @diagram:
        width: 100pt
        height: 80pt
        anchor @badge:
          width: 20pt
          height: 10pt
          left: "canvas.left"
          top: "canvas.top"
`

const bindingResetFixture = `document @report:
  schema invoice:
    number total
  page @sheet:
    body @body:
      paragraph @amount:
        bind: "total"
        bind-required: true
        format: "currency"
        format-locale: "pt-BR"
        format-currency: "BRL"
        format-min-fraction: 2
        format-max-fraction: 2
        text: "Amount"
`

func TestPaperSetDocumentAndPagePropertiesPublishTypedValues(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, authoringPropertiesFixture, "@report", "document-title", CapabilityEdit)
	result, err := workspace.PaperSetDocumentProperty(PaperSetDocumentPropertyRequest{Guard: guard, Property: PaperDocumentTitle, Text: "Final report"})
	if err != nil || !strings.Contains(result.Revision.Source, `title: "Final report"`) {
		t.Fatalf("document property = %#v, %v", result, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, authoringPropertiesFixture, "@sheet", "page-format", CapabilityEdit)
	result, err = workspace.PaperSetPageNumbering(PaperSetPageNumberingRequest{Guard: guard, Property: PaperPageNumberFormat, Text: "Page %d of {pages}"})
	if err != nil || !strings.Contains(result.Revision.Source, `page-number-format: "Page %d of {pages}"`) {
		t.Fatalf("page numbering = %#v, %v", result, err)
	}
}

func TestPaperSetCanvasAndCanvasItemPropertiesRejectInvalidDimensions(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, authoringPropertiesFixture, "@diagram", "canvas-default", CapabilityEdit)
	result, err := workspace.PaperSetCanvasProperty(PaperSetCanvasPropertyRequest{Guard: guard, Property: PaperCanvasDefaultHorizontal, Kind: "center-x"})
	if err != nil || !strings.Contains(result.Revision.Source, `default-horizontal: "center-x"`) {
		t.Fatalf("canvas property = %#v, %v", result, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, authoringPropertiesFixture, "@badge", "canvas-item-width", CapabilityEdit)
	guard.TargetPreconditions = append(guard.TargetPreconditions, exactTargetPrecondition(t, "mutation.paper", authoringPropertiesFixture, "@diagram"))
	result, err = workspace.PaperSetCanvasItem(PaperSetCanvasItemRequest{Guard: guard, Property: string(PaperCanvasItemWidth), Points: 42})
	if err != nil || !strings.Contains(result.Revision.Source, "width: 42pt") {
		t.Fatalf("canvas item = %#v, %v", result, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, authoringPropertiesFixture, "@badge", "canvas-item-invalid", CapabilityEdit)
	guard.TargetPreconditions = append(guard.TargetPreconditions, exactTargetPrecondition(t, "mutation.paper", authoringPropertiesFixture, "@diagram"))
	if _, err := workspace.PaperSetCanvasItem(PaperSetCanvasItemRequest{Guard: guard, Property: string(PaperCanvasItemHeight), Points: 0}); errorCode(err) != "INVALID_CANVAS_ITEM_VALUE" {
		t.Fatalf("zero canvas item height error = %v", err)
	}
}

func TestPaperSetAppearanceAndConditionPublishReadableProperties(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, authoringPropertiesFixture, "@copy", "appearance-style", CapabilityEdit)
	result, err := workspace.PaperSetAppearance(PaperSetAppearanceRequest{Guard: guard, Property: PaperAppearanceStyle, Text: "@body-style"})
	if err != nil || !strings.Contains(result.Revision.Source, `style: "@body-style"`) {
		t.Fatalf("appearance = %#v, %v", result, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, authoringPropertiesFixture, "@copy", "condition", CapabilityEdit)
	result, err = workspace.PaperSetCondition(PaperSetConditionRequest{Guard: guard, Expression: "patient.active == true"})
	if err != nil || !strings.Contains(result.Revision.Source, `when: "patient.active == true"`) {
		t.Fatalf("condition = %#v, %v", result, err)
	}

	workspace = mustWorkspace(t, Limits{})
	guard, _, _ = mutationGuard(t, workspace, authoringPropertiesFixture, "@copy", "condition-invalid", CapabilityEdit)
	if _, err := workspace.PaperSetCondition(PaperSetConditionRequest{Guard: guard, Expression: "patient.active &&"}); errorCode(err) != "INVALID_CONDITION_VALUE" {
		t.Fatalf("invalid condition syntax error = %v", err)
	}
}

func TestPaperResetBindingRemovesAllBindingMetadata(t *testing.T) {
	workspace := mustWorkspace(t, Limits{})
	guard, _, _ := mutationGuard(t, workspace, bindingResetFixture, "@amount", "reset-binding", CapabilityEdit)
	result, err := workspace.PaperResetProperty(PaperResetPropertyRequest{Guard: guard, Category: "binding", Property: "bind"})
	if err != nil {
		t.Fatalf("PaperResetProperty(binding) error = %v", err)
	}
	for _, property := range []string{"bind:", "bind-required:", "format:", "format-locale:", "format-currency:", "format-min-fraction:", "format-max-fraction:"} {
		if strings.Contains(result.Revision.Source, property) {
			t.Fatalf("reset binding retained %s in:\n%s", property, result.Revision.Source)
		}
	}
}
