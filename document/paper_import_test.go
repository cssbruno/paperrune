// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestPlanPaperWithImportsUsesResolverForNamedStyle(t *testing.T) {
	source := `document:
  import: "design.paper"
  page:
    body:
      paragraph:
        style: "@body"
        text: "Imported design rule"
`
	plan, result, err := PlanPaperWithImports("main.paper", source, func(_ context.Context, importerFile, importPath string) (string, string, error) {
		if importerFile != "main.paper" || importPath != "design.paper" {
			return "", "", fmt.Errorf("unexpected import %s from %s", importPath, importerFile)
		}
		return "design.paper", `document:
  style @body:
    font: "Courier"
    size: 11pt
`, nil
	})
	if err != nil || !result.OK() {
		t.Fatalf("PlanPaperWithImports() = %#v, %v", result, err)
	}
	if plan.PageCount() != 1 || plan.Hash() == "" {
		t.Fatalf("plan = pages %d hash %q", plan.PageCount(), plan.Hash())
	}
}

func TestImportedContentChangesSourceManifestRevision(t *testing.T) {
	t.Parallel()
	source := `document:
  import: "design.paper"
  page:
    body:
      paragraph:
        style: "@body"
        text: "Imported design rule"
`
	planWith := func(size string) (PaperPlan, PaperPlanResult) {
		plan, result, err := PlanPaperWithImports("main.paper", source, func(_ context.Context, _, _ string) (string, string, error) {
			return "design.paper", "document:\n  style @body:\n    font: \"Courier\"\n    size: " + size + "\n", nil
		})
		if err != nil || !result.OK() {
			t.Fatalf("PlanPaperWithImports(%s) = %#v, %v", size, result, err)
		}
		return plan, result
	}
	before, beforeResult := planWith("10pt")
	after, afterResult := planWith("18pt")
	rootOnly := PaperSourceManifestRevision(source, nil)
	if before.SourceRevision() == rootOnly || after.SourceRevision() == rootOnly {
		t.Fatal("imported plan retained the root-only source revision")
	}
	if before.SourceRevision() == after.SourceRevision() || beforeResult.SourceRevision == afterResult.SourceRevision {
		t.Fatal("import-only edit did not change source manifest revision")
	}
	if len(beforeResult.Dependencies) != 1 || beforeResult.Dependencies[0].File != "design.paper" {
		t.Fatalf("dependencies = %#v", beforeResult.Dependencies)
	}
}

func TestPlanImportResolverHonorsPlanningContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	source := "document:\n  import: \"blocked.paper\"\n"
	started := time.Now()
	_, result, err := PlanPaperWithImportsContext(ctx, "main.paper", source, func(ctx context.Context, _, _ string) (string, string, error) {
		<-ctx.Done()
		return "", "", ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > time.Second {
		t.Fatalf("PlanPaperWithImportsContext() result=%#v error=%v duration=%v", result, err, time.Since(started))
	}
}
