// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"context"
	"io"
)

// Document is a PDF build session whose only supported authoring surface is
// Paper. The low-level direct-placement and HTML engines are private;
// callers cannot add pages, text, cells, or drawing commands directly.
type Document struct {
	core *pdfDocument
}

// NewDocument creates a Paper document using functional options.
func NewDocument(options ...Option) (*Document, error) {
	core, err := newPDFDocument(options...)
	if err != nil {
		return nil, err
	}
	return &Document{core: core}, nil
}

// MustNew creates a Paper document and panics if construction fails.
func MustNew(options ...Option) *Document {
	doc, err := NewDocument(options...)
	if err != nil {
		panic(err)
	}
	return doc
}

// NewDocumentWithDefaults creates a Paper document with explicit
// per-document defaults.
func NewDocumentWithDefaults(defaults Defaults, options ...Option) (*Document, error) {
	core, err := newPDFDocumentWithDefaults(defaults, options...)
	if err != nil {
		return nil, err
	}
	return &Document{core: core}, nil
}

func (d *Document) engine() *pdfDocument {
	if d == nil {
		return nil
	}
	return d.core
}

func (d *Document) WritePaper(file, source string) (PaperRenderResult, error) {
	return d.engine().WritePaper(file, source)
}

func (d *Document) WritePaperWithAssets(file, source string, assets PaperAssetCatalog) (PaperRenderResult, error) {
	return d.engine().WritePaperWithAssets(file, source, assets)
}

func (d *Document) WritePaperWithImports(file, source string, resolver PaperImportResolver) (PaperRenderResult, error) {
	return d.engine().WritePaperWithImports(file, source, resolver)
}

func (d *Document) WritePaperWithAssetsAndImports(file, source string, assets PaperAssetCatalog, resolver PaperImportResolver) (PaperRenderResult, error) {
	return d.engine().WritePaperWithAssetsAndImports(file, source, assets, resolver)
}

func (d *Document) WritePaperScenario(file, source, scenario string) (PaperRenderResult, error) {
	return d.engine().WritePaperScenario(file, source, scenario)
}

func (d *Document) WritePaperScenarioWithAssets(file, source, scenario string, assets PaperAssetCatalog) (PaperRenderResult, error) {
	return d.engine().WritePaperScenarioWithAssets(file, source, scenario, assets)
}

func (d *Document) WritePaperScenarioWithImports(file, source, scenario string, resolver PaperImportResolver) (PaperRenderResult, error) {
	return d.engine().WritePaperScenarioWithImports(file, source, scenario, resolver)
}

func (d *Document) WritePaperScenarioWithAssetsAndImports(file, source, scenario string, assets PaperAssetCatalog, resolver PaperImportResolver) (PaperRenderResult, error) {
	return d.engine().WritePaperScenarioWithAssetsAndImports(file, source, scenario, assets, resolver)
}

func (d *Document) WritePaperJSON(file, source string, data []byte) (PaperRenderResult, error) {
	return d.engine().WritePaperJSON(file, source, data)
}

func (d *Document) WritePaperJSONWithOptions(file, source string, data []byte, options PaperJSONOptions) (PaperRenderResult, error) {
	return d.engine().WritePaperJSONWithOptions(file, source, data, options)
}

func (d *Document) WritePaperJSONWithAssetsAndImports(file, source string, data []byte, options PaperJSONOptions, assets PaperAssetCatalog, resolver PaperImportResolver) (PaperRenderResult, error) {
	return d.engine().WritePaperJSONWithAssetsAndImports(file, source, data, options, assets, resolver)
}

func (d *Document) WritePaperPlan(plan PaperPlan) (PaperRenderResult, error) {
	return d.engine().WritePaperPlan(plan)
}

func (d *Document) Ok() bool                                 { return d.engine().Ok() }
func (d *Document) Err() bool                                { return d.engine().Err() }
func (d *Document) Error() error                             { return d.engine().Error() }
func (d *Document) ClearError()                              { d.engine().ClearError() }
func (d *Document) PageCount() int                           { return d.engine().PageCount() }
func (d *Document) Close()                                   { d.engine().Close() }
func (d *Document) SetLimits(v Limits) error                 { return d.engine().SetLimits(v) }
func (d *Document) SetSecurityPolicy(v SecurityPolicy) error { return d.engine().SetSecurityPolicy(v) }
func (d *Document) SetProductionPolicy(v ProductionPolicy) error {
	return d.engine().SetProductionPolicy(v)
}
func (d *Document) EnableTaggedPDF() { d.engine().EnableTaggedPDF() }
func (d *Document) SetComplianceMetadata(v ComplianceMetadata) {
	d.engine().SetComplianceMetadata(v)
}
func (d *Document) SetTitle(value string, utf8 bool)   { d.engine().SetTitle(value, utf8) }
func (d *Document) SetSubject(value string, utf8 bool) { d.engine().SetSubject(value, utf8) }

func (d *Document) SetAuthor(value string, utf8 bool) { d.engine().SetAuthor(value, utf8) }
func (d *Document) SetOutputIntent(profile []byte, identifier string) error {
	return d.engine().SetOutputIntent(profile, identifier)
}
func (d *Document) SetAttachments(attachments []Attachment) { d.engine().SetAttachments(attachments) }
func (d *Document) Output(w io.Writer) error                { return d.engine().Output(w) }
func (d *Document) OutputContext(ctx context.Context, w io.Writer) error {
	return d.engine().OutputContext(ctx, w)
}
func (d *Document) OutputWithOptions(w io.Writer, options OutputOptions) error {
	return d.engine().OutputWithOptions(w, options)
}
func (d *Document) OutputWithOptionsContext(ctx context.Context, w io.Writer, options OutputOptions) error {
	return d.engine().OutputWithOptionsContext(ctx, w, options)
}
func (d *Document) OutputStream(w io.Writer) error { return d.engine().OutputStream(w) }
func (d *Document) OutputStreamContext(ctx context.Context, w io.Writer) error {
	return d.engine().OutputStreamContext(ctx, w)
}
func (d *Document) OutputStreamWithOptions(w io.Writer, options OutputOptions) error {
	return d.engine().OutputStreamWithOptions(w, options)
}
func (d *Document) OutputStreamWithOptionsContext(ctx context.Context, w io.Writer, options OutputOptions) error {
	return d.engine().OutputStreamWithOptionsContext(ctx, w, options)
}

func (d *Document) OutputFile(file string) error { return d.engine().OutputFile(file) }
func (d *Document) OutputFileAndClose(file string) error {
	return d.engine().OutputFileAndClose(file)
}
func (d *Document) OutputFileContext(ctx context.Context, file string) error {
	return d.engine().OutputFileContext(ctx, file)
}
func (d *Document) OutputFileWithOptions(file string, options OutputOptions) error {
	return d.engine().OutputFileWithOptions(file, options)
}
func (d *Document) OutputFileWithOptionsContext(ctx context.Context, file string, options OutputOptions) error {
	return d.engine().OutputFileWithOptionsContext(ctx, file, options)
}
func (d *Document) OutputFileStream(file string) error { return d.engine().OutputFileStream(file) }
func (d *Document) OutputFileStreamContext(ctx context.Context, file string) error {
	return d.engine().OutputFileStreamContext(ctx, file)
}
func (d *Document) OutputFileStreamWithOptions(file string, options OutputOptions) error {
	return d.engine().OutputFileStreamWithOptions(file, options)
}
func (d *Document) OutputFileStreamWithOptionsContext(ctx context.Context, file string, options OutputOptions) error {
	return d.engine().OutputFileStreamWithOptionsContext(ctx, file, options)
}
