// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"compress/zlib"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionPolicyAppliesSupportedSettings(t *testing.T) {
	policy := ServerSafePolicy()
	policy.Limits.MaxAttachmentBytes = 123
	pdf, err := newPDFDocument(WithProductionPolicy(policy))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	if pdf.resourceCachePolicy != ResourceCacheDocument {
		t.Fatalf("resourceCachePolicy = %v, want ResourceCacheDocument", pdf.resourceCachePolicy)
	}
	if pdf.imageCache == nil || pdf.fontCache == nil {
		t.Fatal("ServerSafePolicy should install document-local image and font caches")
	}
	gotCompression := pdf.CompressionPolicy()
	if gotCompression.Level != zlib.BestSpeed || gotCompression.PageWorkers != 4 || gotCompression.AttachmentWorkers != 2 {
		t.Fatalf("CompressionPolicy() = %#v, want server-safe compression", gotCompression)
	}
	if pdf.maxAttachmentBytes != 123 {
		t.Fatalf("maxAttachmentBytes = %d, want 123", pdf.maxAttachmentBytes)
	}
	if !pdf.securityPolicySet {
		t.Fatal("ServerSafePolicy should install a security policy")
	}
}

func TestWithServerSafeDefaultsAppliesServerPolicy(t *testing.T) {
	pdf, err := newPDFDocument(WithServerSafeDefaults())
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	if pdf.resourceCachePolicy != ResourceCacheDocument {
		t.Fatalf("resourceCachePolicy = %v, want ResourceCacheDocument", pdf.resourceCachePolicy)
	}
	gotCompression := pdf.CompressionPolicy()
	if gotCompression.PageWorkers != 4 || gotCompression.AttachmentWorkers != 2 {
		t.Fatalf("CompressionPolicy() = %#v, want server-safe workers", gotCompression)
	}
	if !pdf.securityPolicySet {
		t.Fatal("WithServerSafeDefaults should install a security policy")
	}
}

func TestBatchPolicyUsesTrustedOfflineLimits(t *testing.T) {
	limits := BatchLimits()
	server := ServerSafeLimits()
	policy := BatchPolicy()
	if limits.MaxPages <= server.MaxPages || limits.MaxAttachmentBytes <= server.MaxAttachmentBytes {
		t.Fatalf("batch limits are not larger than server limits: %+v", limits)
	}
	if policy.Limits != limits || policy.Cache != ResourceCacheShared || !policy.CacheSet || !policy.Security.AllowFileAttachments {
		t.Fatalf("batch policy = %+v", policy)
	}
}

func TestSetProductionPolicyAppliesToExistingDocument(t *testing.T) {
	pdf := mustNewPDFDocument()
	policy := ServerSafePolicy()
	policy.Limits.MaxPages = 1
	if err := pdf.SetProductionPolicy(policy); err != nil {
		t.Fatalf("SetProductionPolicy() error = %v", err)
	}
	if pdf.resourceCachePolicy != ResourceCacheDocument {
		t.Fatalf("resourceCachePolicy = %v, want ResourceCacheDocument", pdf.resourceCachePolicy)
	}
	pdf.AddPage()
	pdf.AddPage()
	if !errors.Is(pdf.Error(), ErrPageLimitExceeded) {
		t.Fatalf("AddPage() error = %v, want ErrPageLimitExceeded", pdf.Error())
	}
}

func TestPartialProductionPolicyUsesDocumentLocalCache(t *testing.T) {
	pdf, err := newPDFDocument(WithProductionPolicy(ProductionPolicy{Limits: ServerSafeLimits()}))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	if pdf.resourceCachePolicy != ResourceCacheDocument {
		t.Fatalf("resourceCachePolicy = %v, want ResourceCacheDocument for partial policy", pdf.resourceCachePolicy)
	}
}

func TestSecurityPolicyGatesFeatures(t *testing.T) {
	t.Run("JavaScript", func(t *testing.T) {
		pdf, err := newPDFDocument(WithSecurityPolicy(SecurityPolicy{}))
		if err != nil {
			t.Fatalf("NewDocument() error = %v", err)
		}
		_ = pdf.SetJavascriptError("app.alert('blocked')")
		if !errors.Is(pdf.Error(), ErrJavaScriptUnsupported) {
			t.Fatalf("SetJavascript() error = %v, want ErrJavaScriptUnsupported", pdf.Error())
		}
	})

	t.Run("raw writes", func(t *testing.T) {
		pdf, err := newPDFDocument(WithSecurityPolicy(SecurityPolicy{}))
		if err != nil {
			t.Fatalf("NewDocument() error = %v", err)
		}
		if err := pdf.RawWriteStrError("% blocked"); !errors.Is(err, ErrSecurityPolicyDenied) {
			t.Fatalf("RawWriteStrError() error = %v, want ErrSecurityPolicyDenied", err)
		}
	})

	t.Run("file attachments", func(t *testing.T) {
		dir := t.TempDir()
		fileStr := filepath.Join(dir, "payload.txt")
		if err := os.WriteFile(fileStr, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		pdf, err := newPDFDocument(WithSecurityPolicy(SecurityPolicy{}))
		if err != nil {
			t.Fatalf("NewDocument() error = %v", err)
		}
		pdf.AddPage()
		pdf.SetAttachments([]Attachment{AttachmentFromFile(fileStr)})
		var out bytes.Buffer
		err = pdf.Output(&out)
		if !errors.Is(err, ErrSecurityPolicyDenied) {
			t.Fatalf("Output() error = %v, want ErrSecurityPolicyDenied", err)
		}
	})

}

func TestOutputOptionsApplyAttachmentLimits(t *testing.T) {
	dir := t.TempDir()
	fileStr := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(fileStr, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	pdf, err := newPDFDocument()
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.AddPage()
	pdf.SetAttachments([]Attachment{AttachmentFromFile(fileStr)})
	var out bytes.Buffer
	err = pdf.OutputWithOptions(&out, OutputOptions{Limits: Limits{MaxAttachmentBytes: 3}})
	if !errors.Is(err, ErrAttachmentTooLarge) {
		t.Fatalf("OutputWithOptions() error = %v, want ErrAttachmentTooLarge", err)
	}
}

func TestLimitsApplyPageLimit(t *testing.T) {
	pdf, err := newPDFDocument(WithLimits(Limits{MaxPages: 1}))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.AddPage()
	pdf.AddPage()
	if !errors.Is(pdf.Error(), ErrPageLimitExceeded) {
		t.Fatalf("AddPage() error = %v, want ErrPageLimitExceeded", pdf.Error())
	}
}

func TestLimitsApplyImageSourceLimit(t *testing.T) {
	dir := t.TempDir()
	fileStr := filepath.Join(dir, "payload.png")
	if err := os.WriteFile(fileStr, []byte("not a png but too large"), 0o644); err != nil {
		t.Fatal(err)
	}
	pdf, err := newPDFDocument(WithLimits(Limits{MaxImageSourceBytes: 3}))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	_, err = pdf.RegisterImageOptionsError(fileStr, ImageOptions{ImageType: "png"})
	if !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("RegisterImageOptionsError() error = %v, want ErrImageTooLarge", err)
	}
}

func TestOutputOptionsApplyCompression(t *testing.T) {
	pdf, err := newPDFDocument(WithNoCompression())
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 12)
	pdf.Cell(10, 10, "compressed")
	var out bytes.Buffer
	policy := CompressionPolicy{Level: zlib.BestCompression}
	if err := pdf.OutputWithOptions(&out, OutputOptions{Compression: policy}); err != nil {
		t.Fatalf("OutputWithOptions() error = %v", err)
	}
	got := pdf.CompressionPolicy()
	if got.Level != zlib.BestCompression || got.Mode != CompressionEnabled {
		t.Fatalf("CompressionPolicy() = %#v, want best compression enabled", got)
	}
}

func TestOutputContextCanceledBeforeOutput(t *testing.T) {
	pdf, err := newPDFDocument()
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	err = pdf.OutputContext(ctx, &out)
	if !errors.Is(err, ErrOutputCanceled) || !errors.Is(err, context.Canceled) {
		t.Fatalf("OutputContext() error = %v, want ErrOutputCanceled and context.Canceled", err)
	}
	if out.Len() != 0 {
		t.Fatalf("OutputContext() wrote %d bytes after cancellation", out.Len())
	}
}

func TestOutputWithOptionsContextCanceledRestoresOutputSettings(t *testing.T) {
	pdf, err := newPDFDocument()
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	before := pdf.CompressionPolicy()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	err = pdf.OutputWithOptionsContext(ctx, &out, OutputOptions{
		Compression: CompressionPolicy{Level: zlib.BestCompression},
	})
	if !errors.Is(err, ErrOutputCanceled) {
		t.Fatalf("OutputWithOptionsContext() error = %v, want ErrOutputCanceled", err)
	}
	after := pdf.CompressionPolicy()
	if after != before {
		t.Fatalf("CompressionPolicy after canceled output = %#v, want restored %#v", after, before)
	}
}

func TestOutputOptionsValidationIsAtomic(t *testing.T) {
	pdf, err := newPDFDocument(WithNoCompression())
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	before := pdf.CompressionPolicy()
	var out bytes.Buffer
	err = pdf.OutputWithOptions(&out, OutputOptions{
		Compression: CompressionPolicy{Level: zlib.BestCompression},
		Limits:      Limits{MaxPages: -1},
	})
	if err == nil {
		t.Fatal("OutputWithOptions() error = nil, want invalid limits error")
	}
	pdf.ClearError()
	if after := pdf.CompressionPolicy(); after != before {
		t.Fatalf("CompressionPolicy after invalid options = %#v, want %#v", after, before)
	}
}

func TestOutputContextCanceledDuringPageCompression(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pdf, err := newPDFDocument(
		WithCompressionPolicy(CompressionPolicy{
			Mode:                     CompressionEnabled,
			Level:                    zlib.BestSpeed,
			PageWorkers:              2,
			TinyStreamThresholdBytes: 1,
		}),
		WithHooks(Hooks{
			OnPageCompressed: func(page int, inputBytes, outputBytes int) {
				cancel()
			},
		}),
	)
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	for page := 0; page < 8; page++ {
		pdf.AddPage()
		pdf.RawWriteStr(strings.Repeat("0 0 m 1 1 l S\n", 1024))
	}

	var out bytes.Buffer
	err = pdf.OutputContext(ctx, &out)
	if !errors.Is(err, ErrOutputCanceled) || !errors.Is(err, context.Canceled) {
		t.Fatalf("OutputContext() error = %v, want ErrOutputCanceled and context.Canceled", err)
	}
	if out.Len() != 0 {
		t.Fatalf("OutputContext() wrote %d bytes after cancellation", out.Len())
	}
}

func TestHooksObserveAttachmentLoad(t *testing.T) {
	dir := t.TempDir()
	fileStr := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(fileStr, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotName string
	var gotBytes int64
	pdf, err := newPDFDocument(WithHooks(Hooks{
		OnAttachmentLoaded: func(filename string, bytes int64) {
			gotName = filename
			gotBytes = bytes
		},
	}))
	if err != nil {
		t.Fatalf("NewDocument() error = %v", err)
	}
	pdf.AddPage()
	pdf.SetAttachments([]Attachment{AttachmentFromFile(fileStr)})
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	if gotName != "payload.txt" || gotBytes != int64(len("payload")) {
		t.Fatalf("OnAttachmentLoaded = (%q, %d), want (payload.txt, %d)", gotName, gotBytes, len("payload"))
	}
}

type validationFunc func([]byte) (ComplianceValidationReport, error)

func (fn validationFunc) ValidatePDF(data []byte) (ComplianceValidationReport, error) {
	return fn(data)
}

func TestPublicProductionContracts(t *testing.T) {
	var validator Validator = validationFunc(func([]byte) (ComplianceValidationReport, error) {
		return ComplianceValidationReport{}, nil
	})
	report, err := validator.ValidatePDF([]byte("%PDF-2.0"))
	if err != nil {
		t.Fatalf("ValidatePDF() error = %v", err)
	}
	if report.Failed() {
		t.Fatal("empty validation report should not fail")
	}
}
