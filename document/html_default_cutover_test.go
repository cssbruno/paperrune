// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func htmlCutoverDocument(observed *[]typedRouteObservation) *pdfDocument {
	options := []Option{WithUnit(UnitPoint), WithCustomPageSize(Size{Wd: 200, Ht: 160}), WithNoCompression(), WithDeterministicOutput()}
	if observed != nil {
		options = append(options, WithHooks(Hooks{OnLayoutEngineRoute: func(entryPoint, engine, reason string) {
			*observed = append(*observed, typedRouteObservation{entryPoint: entryPoint, engine: engine, reason: reason})
		}}))
	}
	pdf := mustNewPDFDocument(options...)
	pdf.SetMargins(18, 18, 18)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 10)
	return pdf
}

func TestHTMLDefaultCutoverRoutesEveryRenderingEntryPoint(t *testing.T) {
	compiled, err := compileHTML(`<table><tr><td>planned table</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	template, err := compileHTMLTemplate(`<p>hello {{name}}</p>`)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		entry string
		write func(*htmlRenderer) error
	}{
		{"write", "HTML.Write", func(html *htmlRenderer) error { html.Write(10, `<p>write</p>`); return html.pdf.Error() }},
		{"context", "HTML.WriteContext", func(html *htmlRenderer) error { return html.WriteContext(t.Context(), 10, `<p>context</p>`) }},
		{"compiled-table", "HTML.WriteCompiled", func(html *htmlRenderer) error { html.WriteCompiled(10, compiled); return html.pdf.Error() }},
		{"template", "HTML.WriteTemplate", func(html *htmlRenderer) error {
			html.WriteTemplate(10, template, htmlTemplateValues{"name": "template"})
			return html.pdf.Error()
		}},
		{"template-context", "HTML.WriteTemplateContext", func(html *htmlRenderer) error {
			return html.WriteTemplateContext(t.Context(), 10, template, htmlTemplateValues{"name": "context"})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var observed []typedRouteObservation
			pdf := htmlCutoverDocument(&observed)
			html := pdf.htmlNew()
			startY := pdf.GetY()
			if err := test.write(&html); err != nil {
				t.Fatal(err)
			}
			if len(observed) != 1 || observed[0] != (typedRouteObservation{entryPoint: test.entry, engine: "unified"}) {
				t.Fatalf("route = %#v", observed)
			}
			if pdf.GetY() <= startY || pdf.PageCount() == 0 {
				t.Fatalf("StartFrame cursor/pages = %.3f/%d", pdf.GetY(), pdf.PageCount())
			}
		})
	}
}

func TestHTMLCharacterizationCorpusUsesUnifiedRoute(t *testing.T) {
	for _, fixture := range htmlCharacterizationFixtures() {
		if fixture.Classification != "recognized-rendered" && fixture.Classification != "recognized-ignored-metadata" {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			var observed []typedRouteObservation
			pdf := htmlCutoverDocument(&observed)
			html := pdf.htmlNew()
			if err := html.WriteContext(t.Context(), 10, fixture.Source); err != nil {
				t.Fatal(err)
			}
			if len(observed) != 1 || observed[0] != (typedRouteObservation{entryPoint: "HTML.WriteContext", engine: "unified", reason: ""}) {
				t.Fatalf("route = %#v", observed)
			}
		})
	}
}

func TestHTMLDefaultCutoverFallbackCategoriesArePrivateAndRateReady(t *testing.T) {
	const secret = "customer-secret-selector-and-path"
	var observed []typedRouteObservation
	write := func(source string) error {
		pdf := htmlCutoverDocument(&observed)
		html := pdf.htmlNew()
		return html.WriteContext(context.Background(), 10, source)
	}
	if err := write(`<p>unified one</p>`); err != nil {
		t.Fatal(err)
	}
	if err := write(`<table><tr><td>unified table</td></tr></table>`); err != nil {
		t.Fatal(err)
	}
	if err := write(`<div><p>malformed</div>`); err == nil {
		t.Fatal("malformed HTML unexpectedly rendered after legacy deletion")
	}
	if err := write(`<form id="` + secret + `"><label>legacy form<input></label></form>`); err == nil {
		t.Fatal("unsupported form unexpectedly rendered after legacy deletion")
	}
	if len(observed) != 2 {
		t.Fatalf("route events = %#v", observed)
	}
	for _, event := range observed {
		if event.engine != "unified" || event.reason != "" || strings.Contains(event.reason, secret) {
			t.Fatalf("unexpected route event: %#v", event)
		}
	}
}

func TestHTMLDefaultCutoverMatchesExplicitWholePlanCorpus(t *testing.T) {
	corpus := map[string]string{
		"text-list": `<main><h2>Title</h2><p>body</p><ol><li>one</li><li>two</li></ol></main>`,
		"table":     `<table><tr><th>Head</th></tr><tr><td>Body</td></tr></table>`,
		"flex":      `<div style="display:flex;gap:2pt"><section><p>A</p></section><section><p>B</p></section></div>`,
	}
	for name, source := range corpus {
		t.Run(name, func(t *testing.T) {
			compiled, err := compileHTML(source)
			if err != nil {
				t.Fatal(err)
			}
			planner := mustNewPDFDocument(WithUnit(UnitPoint), WithCustomPageSize(Size{Wd: 200, Ht: 160}), WithNoCompression(), WithDeterministicOutput())
			planner.SetMargins(18, 18, 18)
			plan, err := planner.planCompiledHTML(10, compiled)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Hash() == "" {
				t.Fatal("explicit whole-fragment plan has no hash")
			}
			explicit := htmlCutoverDocument(nil)
			explicitHTML := explicit.htmlNew()
			if handled, err := explicitHTML.writeCompiledUnifiedFragmentContext(t.Context(), 10, compiled); err != nil || !handled {
				t.Fatalf("explicit unified fragment route handled=%t err=%v", handled, err)
			}
			direct := htmlCutoverDocument(nil)
			html := direct.htmlNew()
			if err := html.WriteContext(t.Context(), 10, source); err != nil {
				t.Fatal(err)
			}
			if got, want := deterministicDocumentBytes(t, direct), deterministicDocumentBytes(t, explicit); !bytes.Equal(got, want) {
				t.Fatalf("default/explicit unified fragment output differs: %x / %x", planDigest(got), planDigest(want))
			}
		})
	}
}

func TestHTMLDefaultCutoverPolicyAndCancellationFailWithoutFallback(t *testing.T) {
	var observed []typedRouteObservation
	pdf := htmlCutoverDocument(&observed)
	before := append([]byte(nil), pdf.pages[1].Bytes()...)
	html := pdf.htmlNew()
	if err := html.WriteContext(t.Context(), 10, `<a href="javascript:alert(1)">unsafe</a>`); err == nil {
		t.Fatalf("policy error = %v", err)
	}
	if len(observed) != 0 || !bytes.Equal(before, pdf.pages[1].Bytes()) {
		t.Fatalf("policy failure route=%#v mutated=%t", observed, !bytes.Equal(before, pdf.pages[1].Bytes()))
	}

	var canceledObserved []typedRouteObservation
	canceledPDF := htmlCutoverDocument(&canceledObserved)
	canceledBefore := append([]byte(nil), canceledPDF.pages[1].Bytes()...)
	canceledHTML := canceledPDF.htmlNew()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := canceledHTML.WriteContext(ctx, 10, `<p>cancel</p>`); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	if len(canceledObserved) != 0 || !bytes.Equal(canceledBefore, canceledPDF.pages[1].Bytes()) {
		t.Fatalf("canceled route=%#v mutated=%t", canceledObserved, !bytes.Equal(canceledBefore, canceledPDF.pages[1].Bytes()))
	}
}

func BenchmarkHTMLUnifiedDefaultWriteCompiled(b *testing.B) {
	compiled, err := compileHTML(`<style>.row{display:flex;gap:6pt}.grow{flex:1}</style><div class="row"><p>Default route</p><p class="grow">Reusable compiled fragment</p></div>`)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pdf := htmlCutoverDocument(nil)
		html := pdf.htmlNew()
		html.WriteCompiled(12, compiled)
		if pdf.Error() != nil {
			b.Fatal(pdf.Error())
		}
		var output bytes.Buffer
		if err := pdf.OutputWithOptions(&output, OutputOptions{Deterministic: true}); err != nil {
			b.Fatal(err)
		}
	}
}
