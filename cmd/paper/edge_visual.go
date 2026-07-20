// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/inspect"
)

const maxEdgeRasterBytes = 32 << 20

type edgePDFRasterizer struct {
	executable  string
	environment []string
	fontCache   string
}

func newEdgePDFRasterizer() (*edgePDFRasterizer, error) {
	pdftoppm, err := exec.LookPath("pdftoppm")
	if err != nil {
		return nil, errors.New("pdftoppm is required for --edge-visual; install Poppler or run without visual evidence")
	}
	fontCache, err := os.MkdirTemp("", "paperrune-fontconfig-*")
	if err != nil {
		return nil, fmt.Errorf("create font cache: %w", err)
	}
	commandEnvironment := append(os.Environ(), "XDG_CACHE_HOME="+fontCache)
	fontConfig := filepath.Clean(filepath.Join(filepath.Dir(pdftoppm), "..", "..", "native", "poppler", "poppler", "etc", "fonts", "fonts.conf"))
	if info, statErr := os.Stat(fontConfig); statErr == nil && !info.IsDir() {
		commandEnvironment = append(commandEnvironment, "FONTCONFIG_FILE="+fontConfig)
	}
	return &edgePDFRasterizer{executable: pdftoppm, environment: commandEnvironment, fontCache: fontCache}, nil
}

func (r *edgePDFRasterizer) Close() {
	if r != nil && r.fontCache != "" {
		_ = os.RemoveAll(r.fontCache)
		r.fontCache = ""
	}
}

func (r *edgePDFRasterizer) Rasterize(pdfFile, outputDir, baseName string, pages int, dpi uint) ([]edgeCheckRasterPage, error) {
	if r == nil || r.executable == "" {
		return nil, errors.New("PDF rasterizer is not initialized")
	}
	if pages <= 0 || pages > 1000 {
		return nil, fmt.Errorf("cannot rasterize %d pages", pages)
	}
	result := make([]edgeCheckRasterPage, 0, pages)
	for page := 1; page <= pages; page++ {
		name := fmt.Sprintf("%s-page-%03d.png", baseName, page)
		finalPath := filepath.Join(outputDir, name)
		temporary, err := os.CreateTemp(outputDir, ".edge-raster-*")
		if err != nil {
			return nil, fmt.Errorf("create raster temporary file: %w", err)
		}
		temporaryPrefix := temporary.Name()
		if err := temporary.Close(); err != nil {
			return nil, fmt.Errorf("close raster temporary file: %w", err)
		}
		if err := os.Remove(temporaryPrefix); err != nil {
			return nil, fmt.Errorf("prepare raster temporary path: %w", err)
		}
		temporaryPNG := temporaryPrefix + ".png"
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		command := exec.CommandContext(ctx, r.executable,
			"-png", "-r", strconv.FormatUint(uint64(dpi), 10),
			"-f", strconv.Itoa(page), "-l", strconv.Itoa(page), "-singlefile",
			pdfFile, temporaryPrefix,
		) // #nosec G204 -- executable is resolved explicitly and every argument is passed without a shell.
		command.Env = r.environment
		commandError := edgeDiagnosticWriter{remaining: 64 << 10}
		command.Stderr = &commandError
		runErr := command.Run()
		contextErr := ctx.Err()
		cancel()
		if runErr != nil {
			_ = os.Remove(temporaryPNG)
			if contextErr != nil {
				return nil, fmt.Errorf("rasterize page %d: %w", page, contextErr)
			}
			return nil, fmt.Errorf("rasterize page %d: %w: %s", page, runErr, commandError.String())
		}
		payload, readErr := readBoundedRaster(temporaryPNG)
		_ = os.Remove(temporaryPNG)
		if readErr != nil {
			return nil, fmt.Errorf("read raster page %d: %w", page, readErr)
		}
		configuration, decodeErr := png.DecodeConfig(bytes.NewReader(payload))
		if decodeErr != nil || configuration.Width <= 0 || configuration.Height <= 0 {
			return nil, fmt.Errorf("decode raster page %d: %w", page, decodeErr)
		}
		if err := atomicWrite(finalPath, payload, 0o644); err != nil {
			return nil, fmt.Errorf("write raster page %d: %w", page, err)
		}
		result = append(result, edgeCheckRasterPage{
			Page: page, File: name, SHA256: edgeSHA256(payload), Bytes: len(payload),
			Width: configuration.Width, Height: configuration.Height,
		})
	}
	return result, nil
}

type edgeDiagnosticWriter struct {
	buffer    bytes.Buffer
	remaining int
	truncated bool
}

func (w *edgeDiagnosticWriter) Write(payload []byte) (int, error) {
	accepted := len(payload)
	if w.remaining > 0 {
		stored := payload
		if len(stored) > w.remaining {
			stored = stored[:w.remaining]
			w.truncated = true
		}
		_, _ = w.buffer.Write(stored)
		w.remaining -= len(stored)
	} else if accepted != 0 {
		w.truncated = true
	}
	return accepted, nil
}

func (w *edgeDiagnosticWriter) String() string {
	if w.truncated {
		return w.buffer.String() + "\n[diagnostics truncated]"
	}
	return w.buffer.String()
}

func readBoundedRaster(file string) ([]byte, error) {
	info, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	if info.Size() <= 0 || info.Size() > maxEdgeRasterBytes {
		return nil, fmt.Errorf("raster size %d is outside 1..%d bytes", info.Size(), maxEdgeRasterBytes)
	}
	opened, err := os.Open(file) // #nosec G304 -- the path is a private temporary created by this process.
	if err != nil {
		return nil, err
	}
	defer func() { _ = opened.Close() }()
	payload, err := io.ReadAll(io.LimitReader(opened, maxEdgeRasterBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxEdgeRasterBytes {
		return nil, fmt.Errorf("raster exceeds %d-byte limit", maxEdgeRasterBytes)
	}
	return payload, nil
}

func writeEdgeVisualReview(outputDir string, report edgeCheckResult) error {
	pdf, err := document.NewDocument(document.WithDeterministicOutput())
	if err != nil {
		return fmt.Errorf("create edge visual review: %w", err)
	}
	passed := 0
	pageCount := 0
	for _, item := range report.Cases {
		if item.OK {
			passed++
		}
		pageCount += len(item.RasterPages)
	}
	var source strings.Builder
	source.WriteString("document @edge-visual-review:\n" +
		"  title: \"PaperRune visual review\"\n" +
		"  language: \"en\"\n" +
		"  style @review-base:\n" +
		"    font: \"Helvetica\"\n" +
		"    size: 11pt\n" +
		"    line-height: 15pt\n" +
		"    color: \"#14232A\"\n" +
		"  style @review-title:\n" +
		"    style: \"@review-base\"\n" +
		"    size: 22pt\n" +
		"    line-height: 28pt\n" +
		"    bold: true\n" +
		"    color: \"#14323E\"\n" +
		"  style @review-pass:\n" +
		"    style: \"@review-base\"\n" +
		"    bold: true\n" +
		"    color: \"#16846F\"\n" +
		"  style @review-fail:\n" +
		"    style: \"@review-base\"\n" +
		"    bold: true\n" +
		"    color: \"#B94141\"\n" +
		"  page @review-page:\n" +
		"    size: \"A4\"\n" +
		"    margin: 24pt\n" +
		"    body @review-body:\n" +
		"      heading @review-heading:\n" +
		"        level: 1\n" +
		"        style: \"@review-title\"\n" +
		"        text: \"PaperRune visual review\"\n" +
		"      paragraph @review-subtitle:\n" +
		"        style: \"@review-base\"\n" +
		"        text: \"Raster evidence from the final generated PDF files\"\n")
	fmt.Fprintf(&source, "      paragraph @review-schema:\n        style: \"@review-base\"\n        text: %s\n", strconv.Quote("Schema: "+report.Schema))
	summary := fmt.Sprintf("Cases: %d | Passed: %d | Failed: %d | Raster pages: %d | Max layout issues: %d | Minimum text runes: %d | Maximum PDF pages: %d",
		len(report.Cases), passed, len(report.Cases)-passed, pageCount, report.Thresholds.MaxPageIssues,
		report.Thresholds.MinTextRunes, report.Thresholds.MaxPages)
	fmt.Fprintf(&source, "      paragraph @review-summary:\n        style: \"@review-base\"\n        text: %s\n", strconv.Quote(summary))
	resources := make([]document.PaperAssetResource, 0, pageCount)
	resourceIndex := 0
	for _, item := range report.Cases {
		for _, raster := range item.RasterPages {
			width, height := fitEdgeRaster(raster.Width, raster.Height, 547, 720)
			statusStyle := "@review-fail"
			if item.OK {
				statusStyle = "@review-pass"
			}
			label := fmt.Sprintf("%s | PDF page %d of %d", item.Name, raster.Page, item.Pages)
			path := filepath.Join(outputDir, raster.File)
			payload, readErr := readBoundedRaster(path)
			if readErr != nil {
				return fmt.Errorf("read edge visual review raster %s: %w", raster.File, readErr)
			}
			resourceIndex++
			name := fmt.Sprintf("edge-raster-%03d", resourceIndex)
			resources = append(resources, document.PaperAssetResource{Name: name, MediaType: "image/png", Digest: edgeSHA256(payload), Data: payload})
			fmt.Fprintf(&source, "      page-break @review-break-%03d:\n", resourceIndex)
			fmt.Fprintf(&source, "      paragraph @review-label-%03d:\n        style: %s\n        text: %s\n", resourceIndex, strconv.Quote(statusStyle), strconv.Quote(label))
			fmt.Fprintf(&source, "      image @review-image-%03d:\n        source: %s\n        width: %.3fpt\n        height: %.3fpt\n        fit: \"contain\"\n        alt: %s\n",
				resourceIndex, strconv.Quote("asset:"+name), width, height,
				strconv.Quote(fmt.Sprintf("Rendered PDF page %d for edge case %s", raster.Page, item.Name)))
		}
	}
	catalog, err := document.NewPaperAssetCatalog(resources)
	if err != nil {
		return fmt.Errorf("create edge visual review assets: %w", err)
	}
	if rendered, renderErr := pdf.WritePaperWithAssets("edge-visual-review.paper", source.String(), catalog); renderErr != nil || !rendered.OK() {
		return fmt.Errorf("compose edge visual review with Paper: %w", renderErr)
	}
	var encoded bytes.Buffer
	limited := &limitWriter{w: &encoded, remaining: maxPDFBytes}
	if err := pdf.OutputWithOptions(limited, document.OutputOptions{Deterministic: true}); err != nil {
		return fmt.Errorf("encode edge visual review: %w", err)
	}
	if err := inspect.ValidateStructure(encoded.Bytes()); err != nil {
		return fmt.Errorf("validate edge visual review: %w", err)
	}
	pages, err := inspect.PageCount(encoded.Bytes())
	if err != nil || pages != pageCount+1 {
		return fmt.Errorf("validate edge visual review page count: got %d, want %d: %w", pages, pageCount+1, err)
	}
	if err := atomicWrite(filepath.Join(outputDir, report.VisualReviewFile), encoded.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write edge visual review: %w", err)
	}
	return nil
}

func fitEdgeRaster(pixelWidth, pixelHeight int, maxWidth, maxHeight float64) (float64, float64) {
	if pixelWidth <= 0 || pixelHeight <= 0 {
		return 0, 0
	}
	width := maxWidth
	height := width * float64(pixelHeight) / float64(pixelWidth)
	if height > maxHeight {
		height = maxHeight
		width = height * float64(pixelWidth) / float64(pixelHeight)
	}
	return width, height
}
