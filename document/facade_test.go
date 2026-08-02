// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestPaperDocumentFacadeLifecycleAndOutputs(t *testing.T) {
	doc := MustNew(WithNoCompression(), WithDeterministicOutput())
	if err := doc.SetProductionPolicy(DeterministicPolicy()); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetLimits(ServerSafeLimits()); err != nil {
		t.Fatal(err)
	}
	if err := doc.SetSecurityPolicy(SecurityPolicy{AllowFileAttachments: true, AllowRawWrites: true, MaxEmbeddedFileBytes: MaxAttachmentBytes}); err != nil {
		t.Fatal(err)
	}
	doc.EnableTaggedPDF()
	doc.SetComplianceMetadata(ComplianceMetadata{Title: "Facade document", Lang: "en"})
	doc.SetTitle("Facade document", false)
	doc.SetSubject("Paper-only facade", false)
	doc.SetAuthor("PaperRune", false)
	doc.SetAttachments(nil)
	if err := doc.SetOutputIntent(nil, ""); err == nil {
		t.Fatal("empty output intent was accepted")
	}
	doc.ClearError()
	result, err := doc.WritePaper("facade.paper", paperPipelineFixture)
	if err != nil || !result.OK() || !doc.Ok() || doc.Err() || doc.Error() != nil || doc.PageCount() != result.Pages {
		t.Fatalf("facade write = %#v, ok=%t err=%v stored=%v pages=%d", result, doc.Ok(), err, doc.Error(), doc.PageCount())
	}

	writerCalls := []struct {
		name string
		call func(*Document, *bytes.Buffer) error
	}{
		{"Output", func(value *Document, out *bytes.Buffer) error { return value.Output(out) }},
		{"OutputContext", func(value *Document, out *bytes.Buffer) error { return value.OutputContext(t.Context(), out) }},
		{"OutputWithOptions", func(value *Document, out *bytes.Buffer) error { return value.OutputWithOptions(out, OutputOptions{}) }},
		{"OutputWithOptionsContext", func(value *Document, out *bytes.Buffer) error {
			return value.OutputWithOptionsContext(t.Context(), out, OutputOptions{})
		}},
		{"OutputStream", func(value *Document, out *bytes.Buffer) error { return value.OutputStream(out) }},
		{"OutputStreamContext", func(value *Document, out *bytes.Buffer) error { return value.OutputStreamContext(t.Context(), out) }},
		{"OutputStreamWithOptions", func(value *Document, out *bytes.Buffer) error {
			return value.OutputStreamWithOptions(out, OutputOptions{})
		}},
		{"OutputStreamWithOptionsContext", func(value *Document, out *bytes.Buffer) error {
			return value.OutputStreamWithOptionsContext(t.Context(), out, OutputOptions{})
		}},
	}
	for _, call := range writerCalls {
		t.Run(call.name, func(t *testing.T) {
			value := facadeRenderedDocument(t)
			var out bytes.Buffer
			if err := call.call(value, &out); err != nil || !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) {
				t.Fatalf("output = %d bytes, %v", out.Len(), err)
			}
		})
	}

	fileCalls := []struct {
		name string
		call func(*Document, string) error
	}{
		{"OutputFile", func(value *Document, file string) error { return value.OutputFile(file) }},
		{"OutputFileContext", func(value *Document, file string) error { return value.OutputFileContext(t.Context(), file) }},
		{"OutputFileWithOptions", func(value *Document, file string) error { return value.OutputFileWithOptions(file, OutputOptions{}) }},
		{"OutputFileWithOptionsContext", func(value *Document, file string) error {
			return value.OutputFileWithOptionsContext(t.Context(), file, OutputOptions{})
		}},
		{"OutputFileStream", func(value *Document, file string) error { return value.OutputFileStream(file) }},
		{"OutputFileStreamContext", func(value *Document, file string) error { return value.OutputFileStreamContext(t.Context(), file) }},
		{"OutputFileStreamWithOptions", func(value *Document, file string) error {
			return value.OutputFileStreamWithOptions(file, OutputOptions{})
		}},
		{"OutputFileStreamWithOptionsContext", func(value *Document, file string) error {
			return value.OutputFileStreamWithOptionsContext(t.Context(), file, OutputOptions{})
		}},
		{"OutputFileAndClose", func(value *Document, file string) error { return value.OutputFileAndClose(file) }},
	}
	for _, call := range fileCalls {
		t.Run(call.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "facade.pdf")
			if err := call.call(facadeRenderedDocument(t), file); err != nil {
				t.Fatal(err)
			}
			payload, err := os.ReadFile(file)
			if err != nil || !bytes.HasPrefix(payload, []byte("%PDF-")) {
				t.Fatalf("file output = %d bytes, %v", len(payload), err)
			}
		})
	}
	doc.Close()
}

func TestPaperDocumentFacadeAuthoringVariants(t *testing.T) {
	if _, err := NewDocument(WithResourceCachePolicy(ResourceCachePolicy(99))); err == nil {
		t.Fatal("invalid constructor option was accepted")
	}
	if _, err := NewDocumentWithDefaults(DefaultSettings(), WithResourceCachePolicy(ResourceCachePolicy(99))); err == nil {
		t.Fatal("invalid defaults constructor option was accepted")
	}
	if value := MustNew(); value == nil {
		t.Fatal("MustNew returned nil")
	}
	assertRejected := func(name string, call func(*Document) (PaperRenderResult, error)) {
		t.Helper()
		result, err := call(MustNew())
		if err == nil || result.OK() {
			t.Fatalf("%s accepted invalid Paper source: %#v, %v", name, result, err)
		}
	}
	assertRejected("WritePaperWithAssets", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperWithAssets("bad.paper", "invalid", PaperAssetCatalog{})
	})
	assertRejected("WritePaperWithImports", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperWithImports("bad.paper", "invalid", nil)
	})
	assertRejected("WritePaperWithAssetsAndImports", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperWithAssetsAndImports("bad.paper", "invalid", PaperAssetCatalog{}, nil)
	})
	assertRejected("WritePaperScenario", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperScenario("bad.paper", "invalid", "missing")
	})
	assertRejected("WritePaperScenarioWithAssets", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperScenarioWithAssets("bad.paper", "invalid", "missing", PaperAssetCatalog{})
	})
	assertRejected("WritePaperScenarioWithImports", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperScenarioWithImports("bad.paper", "invalid", "missing", nil)
	})
	assertRejected("WritePaperScenarioWithAssetsAndImports", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperScenarioWithAssetsAndImports("bad.paper", "invalid", "missing", PaperAssetCatalog{}, nil)
	})
	assertRejected("WritePaperJSON", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperJSON("bad.paper", "invalid", nil)
	})
	assertRejected("WritePaperJSONWithOptions", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperJSONWithOptions("bad.paper", "invalid", nil, PaperJSONOptions{})
	})
	assertRejected("WritePaperJSONWithAssetsAndImports", func(value *Document) (PaperRenderResult, error) {
		return value.WritePaperJSONWithAssetsAndImports("bad.paper", "invalid", nil, PaperJSONOptions{}, PaperAssetCatalog{}, nil)
	})

	plan, result, err := PlanPaper("facade-plan.paper", paperPipelineFixture)
	if err != nil || !result.OK() {
		t.Fatal(err)
	}
	if rendered, err := MustNew().WritePaperPlan(plan); err != nil || !rendered.OK() {
		t.Fatalf("WritePaperPlan() = %#v, %v", rendered, err)
	}

	defer func() {
		if recover() == nil {
			t.Error("nil facade did not panic")
		}
	}()
	var nilDocument *Document
	_ = nilDocument.Ok()
}

func facadeRenderedDocument(t *testing.T) *Document {
	t.Helper()
	doc, err := NewDocument(WithNoCompression(), WithDeterministicOutput())
	if err != nil {
		t.Fatal(err)
	}
	result, err := doc.WritePaper("facade-output.paper", paperPipelineFixture)
	if err != nil || !result.OK() {
		t.Fatalf("WritePaper() = %#v, %v", result, err)
	}
	return doc
}
