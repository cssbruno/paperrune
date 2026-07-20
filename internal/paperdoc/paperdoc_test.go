// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperdoc

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/cssbruno/paperrune/internal/paperassets"
)

var tinyPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDAT\x08\xd7c\xf8\xcf\xc0\x0f\x00\x05\x00\x01\xff\x89\x99=\x1d\x00\x00\x00\x00IEND\xaeB`\x82")

func TestEncodeDecodeIsDeterministicAndSelfContained(t *testing.T) {
	digest := sha256.Sum256(tinyPNG)
	document := Document{Source: "document @report:\n  import: \"styles.paper\"\n", Imports: map[string]string{"styles.paper": "document @styles:\n"}, Resources: []paperassets.ProjectResource{{
		Name: "hero", MediaType: "image/png", Digest: hex.EncodeToString(digest[:]), Data: tinyPNG,
	}}}
	options := EncodeOptions{Compression: CompressionDeflate}
	first, err := EncodeWithOptions(document, options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncodeWithOptions(document, options)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("deterministic encode = %v, equal %v", err, bytes.Equal(first, second))
	}
	reader, err := zip.NewReader(bytes.NewReader(first), int64(len(first)))
	if err != nil || len(reader.File) != 5 || reader.File[0].Name != mimetypePath || reader.File[0].Method != zip.Store {
		t.Fatalf("archive envelope = %v, %#v", err, reader.File)
	}
	methods := make(map[string]uint16, len(reader.File))
	for _, file := range reader.File {
		methods[file.Name] = file.Method
	}
	if methods[manifestPath] != zip.Deflate || methods[documentSourcePath] != zip.Deflate ||
		methods[importArchivePath("styles.paper")] != zip.Deflate {
		t.Fatalf("text entry compression methods = %#v", methods)
	}
	resourcePath, err := resourcePath(document.Resources[0])
	if err != nil || methods[resourcePath] != zip.Store {
		t.Fatalf("resource compression method = %d, %v", methods[resourcePath], err)
	}
	decoded, err := Decode(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Source != document.Source || decoded.Imports["styles.paper"] != document.Imports["styles.paper"] || len(decoded.Resources) != 1 || decoded.Resources[0].Name != "hero" || !bytes.Equal(decoded.Resources[0].Data, tinyPNG) {
		t.Fatalf("decoded = %#v", decoded)
	}
	decoded.Resources[0].Data[0] ^= 0xff
	again, err := Decode(context.Background(), first)
	if err != nil || !bytes.Equal(again.Resources[0].Data, tinyPNG) {
		t.Fatal("decoded resources were not detached")
	}
}

func TestEncodeCompressesPaperSourcesWithoutChangingThem(t *testing.T) {
	source := "document @report:\n" + strings.Repeat("  paragraph:\n    text: \"PaperRune source remains editable\"\n", 256)
	importSource := "document @styles:\n" + strings.Repeat("  style: \"body\"\n", 256)
	encoded, err := EncodeWithOptions(Document{Source: source, Imports: map[string]string{"styles.paper": importSource}}, EncodeOptions{Compression: CompressionDeflate})
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Name != documentSourcePath && file.Name != importArchivePath("styles.paper") {
			continue
		}
		if file.Method != zip.Deflate || file.CompressedSize64 >= file.UncompressedSize64 {
			t.Fatalf("%s compression = method %d, compressed %d, uncompressed %d", file.Name, file.Method, file.CompressedSize64, file.UncompressedSize64)
		}
	}
	decoded, err := Decode(context.Background(), encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Source != source || decoded.Imports["styles.paper"] != importSource {
		t.Fatal("compressed archive did not preserve exact Paper source")
	}
}

func TestEncodeCompressionIsOptional(t *testing.T) {
	document := Document{Source: "document @report:\n", Imports: map[string]string{"styles.paper": "document @styles:\n"}}
	encoded, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range reader.File {
		if file.Method != zip.Store {
			t.Fatalf("default compression method for %s = %d, want Store", file.Name, file.Method)
		}
	}
	if _, err := EncodeWithOptions(document, EncodeOptions{Compression: Compression(255)}); err == nil {
		t.Fatal("unsupported compression option accepted")
	}
}

func TestDecodeRejectsTamperedAndUndeclaredContent(t *testing.T) {
	document, err := Encode(Document{Source: "document @report:\n"})
	if err != nil {
		t.Fatal(err)
	}
	tampered := rewriteArchive(t, document, func(files map[string][]byte) { files[documentSourcePath] = []byte("changed") })
	if _, err := Decode(context.Background(), tampered); err == nil {
		t.Fatal("tampered source accepted")
	}
	extra := rewriteArchive(t, document, func(files map[string][]byte) { files["extra.bin"] = []byte("undeclared") })
	if _, err := Decode(context.Background(), extra); err == nil {
		t.Fatal("undeclared entry accepted")
	}
}

func rewriteArchive(t *testing.T, encoded []byte, mutate func(map[string][]byte)) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string][]byte, len(reader.File))
	for _, file := range reader.File {
		input, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		data := new(bytes.Buffer)
		_, copyErr := data.ReadFrom(input)
		_ = input.Close()
		if copyErr != nil {
			t.Fatal(copyErr)
		}
		files[file.Name] = data.Bytes()
	}
	mutate(files)
	result, err := encodeArchive(files, EncodeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
