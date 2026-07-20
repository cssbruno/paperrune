// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
)

const maxPaperHTMLBytes = 64 << 20

// ExportHTML returns a deterministic standalone HTML document containing the
// exact planned Paper pages as inline SVG. Browsers display the immutable
// Paper layout; they do not measure, wrap, paginate, or otherwise author it.
func (p PaperPlan) ExportHTML() ([]byte, error) {
	return p.ExportHTMLContext(context.Background())
}

// ExportHTMLContext is the cancellation-aware form of ExportHTML.
func (p PaperPlan) ExportHTMLContext(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		return nil, errors.New("document: nil Paper HTML export context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.hash == "" || p.pages <= 0 {
		return nil, errors.New("document: empty paper plan")
	}
	title := p.title
	if title == "" {
		title = "Paper document"
	}
	language := p.language
	if language == "" {
		language = "en"
	}

	var output bytes.Buffer
	write := func(value string) error {
		if len(value) > maxPaperHTMLBytes-output.Len() {
			return fmt.Errorf("document: Paper HTML export exceeds %d bytes", maxPaperHTMLBytes)
		}
		_, _ = output.WriteString(value)
		return nil
	}
	if err := write("<!doctype html>\n<html lang=\"" + html.EscapeString(language) + "\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<meta name=\"paperrune-plan-hash\" content=\"" + p.hash + "\">\n<title>" + html.EscapeString(title) + "</title>\n<style>\nhtml{background:#e5e7eb}body{margin:0;padding:24px;font-family:system-ui,sans-serif}.paper-page{margin:0 auto 24px;background:#fff;box-shadow:0 2px 12px #0002}.paper-page svg{display:block;width:100%;height:auto}@media print{html{background:#fff}body{padding:0}.paper-page{margin:0;box-shadow:none;break-after:page}.paper-page:last-child{break-after:auto}}\n</style>\n</head>\n<body>\n"); err != nil {
		return nil, err
	}
	for page := 1; page <= p.pages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		capture, err := p.CaptureDisplayPageSVG(ctx, uint32(page), nil) // #nosec G115 -- page is bounded by the retained plan page count.
		if err != nil {
			return nil, fmt.Errorf("document: export Paper HTML page %d: %w", page, err)
		}
		svg := strings.TrimPrefix(string(capture.SVG), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		width := fixedPointCSS(capture.PageWidth, capture.FixedScale)
		section := "<section class=\"paper-page\" aria-label=\"Page " + strconv.Itoa(page) + " of " + strconv.Itoa(p.pages) + "\" style=\"max-width:" + width + "pt\">\n" + svg + "\n</section>\n"
		if err := write(section); err != nil {
			return nil, err
		}
	}
	if err := write("</body>\n</html>\n"); err != nil {
		return nil, err
	}
	return append([]byte(nil), output.Bytes()...), nil
}

func fixedPointCSS(value, scale int64) string {
	if scale <= 0 {
		return "0"
	}
	return strconv.FormatFloat(float64(value)/float64(scale), 'f', 3, 64)
}
