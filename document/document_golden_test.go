// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/cssbruno/paperrune/internal/layout"
)

func TestWriteDocumentGoldenPDFs(t *testing.T) {
	cases := []struct {
		name string
		doc  *layout.LayoutDocument
		want string
	}{
		{name: "structured-report", doc: goldenStructuredReportDocument(), want: "dd417ef2dc9925879d0f283ee548bea44acfbffb980d0f9bb6e1db4c39f01ecc"},
		{name: "tabular-report", doc: goldenTabularReportDocument(), want: "ea7e8e07e961af2c36916ec8867e9331acf6c2aaef871d6628ef3595d410393f"},
		{name: "transactional", doc: goldenTransactionalDocument(), want: "2cd05e59aec1de12d709333ab0f02011c1a5b3812e1ac220c071cd90e856e441"},
		{name: "attestation", doc: goldenAttestationDocument(), want: "2300c804a6b2f3a8b9519bd33701b842accc270c225eeb955b1edaa8e322caf8"},
		{name: "statement", doc: goldenStatementDocument(), want: "25f331110f903590f6224b86c93e2141e0cfa8c7d68fbadebbb076d5b6379538"},
		{name: "generic-free-text", doc: goldenGenericDocument(), want: "bd238196e0a5d749f3fd2c1b18ae1fc12753c162981a46e14e589279ea3d7595"},
		{name: "long-form", doc: goldenLongFormDocument(), want: "bafe188c79f0141751034561b2158844e5fc133209e6cd82e99232c0cd8211df"},
		{name: "form", doc: formDocumentModel(testFormDocument()), want: "1ef6d29cee511222f0d4d2893f762f6a22e2a109b96ab2382dc626ae35657b04"},
		{name: "qr-signature", doc: goldenQRSignatureDocument(), want: "5453e5a26f0120d1f675a6da48a6176ae7b01dad50a1e0d9a19af5dcb1c38ee9"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := goldenDocumentPDFSHA(t, tc.doc)
			if got != tc.want {
				t.Fatalf("golden SHA = %s, want %s", got, tc.want)
			}
		})
	}
}

func goldenDocumentPDFSHA(t *testing.T, doc *layout.LayoutDocument) string {
	t.Helper()
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	pdf.SetCatalogSort(true)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pdf.SetCreationDate(fixed)
	pdf.SetModificationDate(fixed)
	pdf.SetProducer("Document golden", false)
	pdf.SetCreator("Document golden test", false)
	pdf.WriteDocument(doc)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(out.Bytes()))
}

func goldenStructuredReportDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "Structured Report"
	doc.PageTemplate.Header = &layout.HeaderBlock{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Structured Header"}}, Style: layout.TextStyle{FontSize: 9}}}}
	doc.PageTemplate.Footer = &layout.FooterBlock{ReservePageArea: true}
	doc.PageTemplate.PageNumbers = layout.PageNumberOptions{Enabled: true, TotalPageAlias: "{total}"}
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Structured Report"}}},
		layout.MetadataGridBlock{Fields: []layout.MetadataField{{Label: "ID", Value: "SR-001"}, {Label: "Status", Value: "Final"}}},
		layout.SectionBlock{Title: "Summary", Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "A deterministic structured report."}}}}},
	}
	return doc
}

func goldenTabularReportDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "Tabular Report"
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Tabular Report"}}},
		layout.TableBlock{
			Caption: "Metrics",
			Header:  []layout.TableRow{{Cells: []layout.TableCell{{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Metric"}}}}}, {Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Value"}}}}}}}},
			Body: []layout.TableRow{
				{Cells: []layout.TableCell{{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Alpha"}}}}}, {Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "10"}}}}}}},
				{Cells: []layout.TableCell{{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Beta"}}}}}, {Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "20"}}}}}}},
			},
		},
	}
	return doc
}

func goldenTransactionalDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "Transaction"
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Transaction Receipt"}}},
		layout.MetadataGridBlock{Fields: []layout.MetadataField{{Label: "Reference", Value: "TX-001"}, {Label: "Amount", Value: "100.00"}}},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Transaction completed."}}},
	}
	return doc
}

func goldenAttestationDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "Attestation"
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Attestation"}}},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "This attests that the described facts are recorded."}}},
	}
	doc.Signature = &layout.SignatureBlock{Rows: []layout.SignatureRowBlock{{Columns: []layout.SignatureColumn{{Label: "Responsible", Name: "A. Example"}}}}}
	return doc
}

func goldenStatementDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "Statement"
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Statement"}}},
		layout.NoteBoxBlock{Title: "Notice", Body: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "This is a deterministic statement."}}}}},
	}
	return doc
}

func goldenGenericDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "Generic"
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Generic Document"}}},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Free text content for generic rendering."}}},
	}
	return doc
}

func goldenLongFormDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "Long Form"
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "Long Form"}}},
		layout.HeadingBlock{Level: 2, Segments: []layout.TextSegment{{Text: "Clause"}}},
		layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: "Long-form text."}}},
	}
	doc.PageTemplate.Footer = &layout.FooterBlock{
		Blocks: []layout.Block{layout.ParagraphBlock{
			Segments: []layout.TextSegment{{Text: "Long footer"}},
			Style:    layout.TextStyle{FontFamily: "Helvetica", FontSize: 9, Align: "C"},
		}},
		ReservePageArea: true,
	}
	return doc
}

func goldenQRSignatureDocument() *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = "QR Signature"
	doc.Body = []layout.Block{
		layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: "QR Signature"}}},
		layout.QRVerificationBlock{QR: layout.QRBlock{Label: "Verify", URL: "https://example.test/verify/1", Size: 22}},
	}
	doc.Signature = &layout.SignatureBlock{Rows: []layout.SignatureRowBlock{{Columns: []layout.SignatureColumn{
		{Label: "Primary", Name: "A. Example", Role: "Signer"},
		{Label: "Secondary", Name: "B. Example", Role: "Reviewer"},
	}}}}
	return doc
}
