// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

// Command compliance-fixtures generates deterministic candidate PDFs for
// external standards validators.
package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/cssbruno/paperrune/document"
)

func main() {
	outDir := flag.String("out", filepath.Join("artifacts", "compliance"), "directory for generated compliance candidate PDFs")
	iccPath := flag.String("icc", "", "path to an sRGB ICC profile for PDF/A output intents")
	flag.Parse()

	// #nosec G301 -- compliance fixtures are deliberately readable by validator containers.
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		exitErr(err)
	}
	root, err := repoRoot()
	if err != nil {
		exitErr(err)
	}
	fontPath := filepath.Join(root, "assets", "static", "font", "DejaVuSansCondensed.ttf")
	boldFontPath := filepath.Join(root, "assets", "static", "font", "DejaVuSansCondensed-Bold.ttf")

	path := filepath.Join(*outDir, "pdfua2-arlington-metadata-foundation.pdf")
	if err := generatePDFUAArlingtonFoundation(path, root, fontPath, boldFontPath); err != nil {
		exitErr(err)
	}
	if err := makeArtifactReadable(path); err != nil {
		exitErr(err)
	}
	fmt.Printf("generated %s\n", path)

	if *iccPath == "" {
		fmt.Fprintln(os.Stderr, "SRGB_ICC/-icc not set; skipped PDF/A fixtures that require a real ICC profile")
		return
	}
	icc, err := os.ReadFile(*iccPath)
	if err != nil {
		exitErr(fmt.Errorf("read ICC profile: %w", err))
	}
	for _, fixture := range []struct {
		name       string
		mode       document.PDFAMode
		attachment bool
	}{
		{name: "pdfa4-metadata.pdf", mode: document.PDFAMode4},
		{name: "pdfa4f-attachment-metadata.pdf", mode: document.PDFAMode4F, attachment: true},
		{name: "pdfa4e-attachment-metadata.pdf", mode: document.PDFAMode4E, attachment: true},
	} {
		path := filepath.Join(*outDir, fixture.name)
		if err := generatePDFAFoundation(path, fontPath, boldFontPath, icc, fixture.mode, fixture.attachment); err != nil {
			exitErr(err)
		}
		if err := makeArtifactReadable(path); err != nil {
			exitErr(err)
		}
		fmt.Printf("generated %s\n", path)
	}
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve command source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("resolve repository root from %s: %w", file, err)
	}
	return root, nil
}

func generatePDFUAArlingtonFoundation(path, root, fontPath, boldFontPath string) error {
	pdf := baseDocument(fontPath, boldFontPath)
	pdf.SetTitle("PDF/UA-2 Arlington metadata foundation", false)
	pdf.SetSubject("Generated tagged PDF structure, metadata, and catalog markers for external validation workflow", false)
	pdf.SetComplianceMetadata(document.ComplianceMetadata{
		PDFUA2:     true,
		Arlington:  true,
		Lang:       "en-US",
		Identifier: "urn:uuid:paperrune-pdfua2-arlington-foundation",
	})
	logo, err := complianceLogoDataURI(root)
	if err != nil {
		return err
	}
	catalog, err := compliancePaperFontCatalog(fontPath, boldFontPath)
	if err != nil {
		return err
	}
	source := fmt.Sprintf(`document @pdfua:
  language: "en-US"
  style @base:
    font: "Compliance Sans"
    size: 12pt
    line-height: 16pt
  style @heading-style:
    style: "@base"
    size: 20pt
    line-height: 26pt
  page @sheet:
    width: 595pt
    height: 842pt
    margin: 36pt
    body @body:
      heading @heading:
        level: 1
        style: "@heading-style"
        text: "PDF/UA-2 tagged structure fixture"
      paragraph @description:
        style: "@base"
        text: "This file exercises PaperRune tagged PDF output, XMP metadata, catalog markers, parent tree entries, marked content IDs, images, lists, tables, and artifacts."
      image @logo:
        source: %q
        width: 68pt
        height: "auto"
        alt: "PaperRune logo"
      list @features:
        style: "@base"
        marker: "dash"
        item @feature-one:
          text: "Signed list label"
        item @feature-two:
          text: "Second semantic item"
      table @status-table:
        table-column @name-column:
          width: 34%%
        table-column @status-column:
          width: 22%%
        table-column @detail-column:
          width: 44%%
        table-header @status-header:
          table-row @status-header-row:
            cell @name-heading-cell:
              paragraph:
                style: "@base"
                text: "Name"
            cell @status-heading-cell:
              paragraph:
                style: "@base"
                text: "Status"
            cell @detail-heading-cell:
              paragraph:
                style: "@base"
                text: "Detail"
        table-row @structure-row:
          cell @structure-name-cell:
            paragraph:
              style: "@base"
              text: "Structure tree"
          cell @structure-status-cell:
            paragraph:
              style: "@base"
              text: "Generated"
          cell @structure-detail-cell:
            paragraph:
              style: "@base"
              text: "Tagged table"
        table-row @parent-row:
          cell @parent-name-cell:
            paragraph:
              style: "@base"
              text: "Parent tree"
          cell @parent-status-cell:
            paragraph:
              style: "@base"
              text: "OK"
          cell @parent-detail-cell:
            paragraph:
              style: "@base"
              text: "Accessible body"
`, logo)
	if rendered, err := pdf.WritePaperWithAssets("pdfua.paper", source, catalog); err != nil || !rendered.OK() {
		return fmt.Errorf("render PDF/UA Paper source: %w (%+v)", err, rendered.Diagnostics)
	}
	return pdf.OutputFileAndClose(path)
}

func generatePDFAFoundation(path, fontPath, boldFontPath string, icc []byte, mode document.PDFAMode, attachment bool) error {
	pdf := baseDocument(fontPath, boldFontPath)
	pdf.SetTitle("PDF/A-4 metadata foundation", false)
	pdf.SetSubject("Generated PDF/A-4 metadata, catalog, output intent, and font embedding fixture", false)
	pdf.SetComplianceMetadata(document.ComplianceMetadata{
		PDFA:       mode,
		Lang:       "en-US",
		Identifier: "urn:uuid:paperrune-" + string(mode) + "-foundation",
	})
	if err := pdf.SetOutputIntent(icc, "sRGB IEC61966-2.1"); err != nil {
		return err
	}
	if attachment {
		pdf.SetAttachments([]document.Attachment{{
			Filename:    "note.txt",
			Description: "PDF/A-4f attachment fixture",
			MIMEType:    "text/plain",
			Content:     []byte("Attachment used to exercise PDF/A-4f generation."),
		}})
	}
	catalog, err := compliancePaperFontCatalog(fontPath, boldFontPath)
	if err != nil {
		return err
	}
	source := "document @pdfa:\n" +
		"  language: \"en-US\"\n" +
		"  page @sheet:\n" +
		"    width: 595pt\n" +
		"    height: 842pt\n" +
		"    margin: 36pt\n" +
		"    body @body:\n" +
		"      heading @title:\n" +
		"        level: 1\n" +
		"        font: \"Compliance Sans\"\n" +
		"        text: \"PDF/A-4 metadata foundation\"\n" +
		"      paragraph @copy:\n" +
		"        font: \"Compliance Sans\"\n" +
		"        text: \"This file exercises PaperRune PDF/A-4 metadata, catalog output intent, and embedded UTF-8 font generation.\"\n"
	if rendered, err := pdf.WritePaperWithAssets("pdfa.paper", source, catalog); err != nil || !rendered.OK() {
		return fmt.Errorf("render PDF/A Paper source: %w", err)
	}
	return pdf.OutputFileAndClose(path)
}

func baseDocument(_, _ string) *document.Document {
	return document.MustNew(document.WithDeterministicOutput(), document.WithNoCompression())
}

func complianceLogoDataURI(root string) (string, error) {
	rootDir, err := os.OpenRoot(root)
	if err != nil {
		return "", fmt.Errorf("open compliance root: %w", err)
	}
	defer func() { _ = rootDir.Close() }()
	data, err := rootDir.ReadFile("assets/static/image/logo.png")
	if err != nil {
		return "", fmt.Errorf("read compliance logo: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), nil
}

func compliancePaperFontCatalog(fontPath, _ string) (document.PaperAssetCatalog, error) {
	// Paper manifests deliberately cap each immutable resource at 512 KiB. The
	// compact checked-in TrueType fixture covers this ASCII compliance content.
	compact := filepath.Join(filepath.Dir(fontPath), "calligra.ttf")
	resources := make([]document.PaperAssetResource, 0, 2)
	for _, item := range []struct {
		name, path, style string
		weight            uint16
	}{{"body-font", compact, "normal", 400}, {"body-font-bold", compact, "normal", 700}} {
		data, err := os.ReadFile(item.path)
		if err != nil {
			return document.PaperAssetCatalog{}, fmt.Errorf("read compliance font: %w", err)
		}
		digest := sha256.Sum256(data)
		resources = append(resources, document.PaperAssetResource{Name: item.name, MediaType: "font/ttf", Digest: hex.EncodeToString(digest[:]), Data: data, Family: "Compliance Sans", Style: item.style, Weight: item.weight, License: "fixture"})
	}
	return document.NewPaperAssetCatalog(resources)
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func makeArtifactReadable(path string) error {
	// #nosec G302 -- compliance artifacts must be readable by external validator containers.
	if err := os.Chmod(path, 0o644); err != nil {
		return fmt.Errorf("make artifact readable %s: %w", path, err)
	}
	return nil
}
