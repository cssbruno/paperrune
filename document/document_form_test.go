// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/layout"
)

func testFormDocument() formDocument {
	return formDocument{
		Title: "Form Title",
		Sections: []formSection{
			{
				Title:        "Profile",
				KeepTogether: true,
				Questions: []formQuestion{
					{Label: "Name", Required: true, Answer: formAnswer{Text: "Alex Example"}},
					{Label: "Options", Answer: formAnswer{Items: []string{"One", "Two"}}},
					{Label: "Scores", Answer: formAnswer{Table: [][]string{{"Name", "Score"}, {"A", "10"}}}},
				},
			},
			{
				Title:       "Next page",
				BreakBefore: true,
				Questions: []formQuestion{
					{Label: "Comment", Answer: formAnswer{Text: "Continue"}},
				},
			},
		},
	}
}

func TestFormDocumentBlocks(t *testing.T) {
	blocks := formDocumentBlocks(testFormDocument())
	if len(blocks) != 3 {
		t.Fatalf("blocks = %d, want title plus two sections", len(blocks))
	}
	if got := blocks[0].DocumentBlockKind(); got != layout.BlockKindHeading {
		t.Fatalf("first block kind = %q, want heading", got)
	}
	section, ok := blocks[1].(layout.SectionBlock)
	if !ok {
		t.Fatalf("second block = %T, want layout.SectionBlock", blocks[1])
	}
	if !section.Box.KeepTogether {
		t.Fatal("first section should keep grouped questions together")
	}
}

func TestWriteDocumentRendersFormDocumentModel(t *testing.T) {
	pdf := mustNewPDFDocument()
	pdf.SetCompression(false)
	doc := formDocumentModel(testFormDocument())

	pdf.WriteDocument(doc)
	var out bytes.Buffer
	if err := pdf.Output(&out); err != nil {
		t.Fatalf("Output() error = %v", err)
	}
	content := extractedDocumentText(t, out.Bytes())
	for _, want := range []string{"Form Title", "Name *", "Alex Example", "Options", "Score", "Comment"} {
		if !strings.Contains(content, want) {
			t.Fatalf("PDF output missing form content %q", want)
		}
	}
	if pdf.PageCount() < 2 {
		t.Fatalf("PageCount() = %d, want at least 2 from form page-break policy", pdf.PageCount())
	}
}
