// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"strings"

	"github.com/cssbruno/paperrune/internal/layout"
)

// formDocument describes a test form converted into the private layout model.
type formDocument struct {
	Title    string        // Form title.
	Sections []formSection // Form sections in display order.
}

// formSection groups related form questions.
type formSection struct {
	Title        string         // Section title.
	Questions    []formQuestion // Questions in this section.
	BreakBefore  bool           // Insert a page break before this section.
	BreakAfter   bool           // Insert a page break after this section.
	KeepTogether bool           // Prefer to keep the section on one page.
}

// formQuestion stores one question and its answer.
type formQuestion struct {
	Label    string     // Question label.
	Answer   formAnswer // Question answer.
	Required bool       // Whether the question is required.
}

// formAnswer stores a plain, list, or table answer.
type formAnswer struct {
	Text  string     // Plain text answer.
	Items []string   // List answer items.
	Table [][]string // Table answer rows.
}

// formDocumentBlocks converts a form into private layout blocks.
func formDocumentBlocks(form formDocument) []layout.Block {
	blocks := make([]layout.Block, 0, 1+len(form.Sections))
	if strings.TrimSpace(form.Title) != "" {
		blocks = append(blocks, layout.HeadingBlock{Level: 1, Segments: []layout.TextSegment{{Text: form.Title}}})
	}
	for _, section := range form.Sections {
		sectionBlock := layout.SectionBlock{
			Title:             section.Title,
			KeepTitleWithBody: true,
			Box:               layout.BoxStyle{KeepTogether: section.KeepTogether},
		}
		if section.BreakBefore || section.BreakAfter {
			sectionBlock.Blocks = append(sectionBlock.Blocks, layout.PageBreakBlock{Before: section.BreakBefore})
		}
		for _, question := range section.Questions {
			sectionBlock.Blocks = append(sectionBlock.Blocks, formQuestionBlocks(question)...)
		}
		if section.BreakAfter {
			sectionBlock.Blocks = append(sectionBlock.Blocks, layout.PageBreakBlock{After: true})
		}
		blocks = append(blocks, sectionBlock)
	}
	return blocks
}

// formDocumentModel converts a form into the private layout model.
func formDocumentModel(form formDocument) *layout.LayoutDocument {
	doc := layout.NewLayoutDocument()
	doc.Title = form.Title
	doc.Body = formDocumentBlocks(form)
	return doc
}

func formQuestionBlocks(question formQuestion) []layout.Block {
	label := question.Label
	if question.Required {
		label += " *"
	}
	blocks := []layout.Block{
		layout.ParagraphBlock{
			Segments: []layout.TextSegment{{Text: label}},
			Style:    layout.TextStyle{Bold: true},
			Box:      layout.BoxStyle{KeepWithNext: true},
		},
	}
	blocks = append(blocks, formAnswerBlocks(question.Answer)...)
	return blocks
}

func formAnswerBlocks(answer formAnswer) []layout.Block {
	switch {
	case len(answer.Table) > 0:
		rows := make([]layout.TableRow, 0, len(answer.Table))
		for _, inputRow := range answer.Table {
			row := layout.TableRow{KeepTogether: true}
			for _, cell := range inputRow {
				row.Cells = append(row.Cells, layout.TableCell{
					Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: cell}}}},
				})
			}
			rows = append(rows, row)
		}
		return []layout.Block{layout.TableBlock{Body: rows}}
	case len(answer.Items) > 0:
		items := make([]layout.ListItem, 0, len(answer.Items))
		for _, item := range answer.Items {
			items = append(items, layout.ListItem{Blocks: []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: item}}}}})
		}
		return []layout.Block{layout.ListBlock{Items: items}}
	default:
		return []layout.Block{layout.ParagraphBlock{Segments: []layout.TextSegment{{Text: answer.Text}}}}
	}
}
