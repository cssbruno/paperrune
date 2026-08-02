// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPointSizeAndCacheUtilities(t *testing.T) {
	point := Point{X: 2, Y: 3}
	if got := point.Transform(4, 5); got != (Point{X: 6, Y: 8}) {
		t.Fatalf("transformed point = %+v", got)
	}
	if got := (*Size)(nil).Orientation(); got != "" {
		t.Fatalf("nil orientation = %q", got)
	}
	if got := (&Size{Wd: 10, Ht: 10}).Orientation(); got != "" {
		t.Fatalf("square orientation = %q", got)
	}
	if got := (&Size{Wd: 20, Ht: 10}).Orientation(); got != "L" {
		t.Fatalf("landscape orientation = %q", got)
	}
	portrait := Size{Wd: 10, Ht: 20}
	if got := portrait.Orientation(); got != "P" {
		t.Fatalf("portrait orientation = %q", got)
	}
	if got := portrait.ScaleBy(2); got != (Size{Wd: 20, Ht: 40}) {
		t.Fatalf("scaled size = %+v", got)
	}
	if got := portrait.ScaleToWidth(5); got != (Size{Wd: 5, Ht: 10}) {
		t.Fatalf("width-scaled size = %+v", got)
	}
	if got := portrait.ScaleToHeight(10); got != (Size{Wd: 5, Ht: 10}) {
		t.Fatalf("height-scaled size = %+v", got)
	}

	var nilCache *FontCache
	if stats := nilCache.Stats(); stats != (CacheStats{}) {
		t.Fatalf("nil font cache stats = %+v", stats)
	}
	nilCache.Clear()
	cache := NewFontCache()
	cache.fonts["body"] = cachedUTF8Font{data: []byte("font")}
	if stats := cache.Stats(); stats != (CacheStats{Entries: 1, Bytes: 4}) {
		t.Fatalf("font cache stats = %+v", stats)
	}
	cache.Clear()
	if stats := cache.Stats(); stats != (CacheStats{}) {
		t.Fatalf("cleared font cache stats = %+v", stats)
	}
}

func TestFileResourceLoaderReportsBoundedFileMetadata(t *testing.T) {
	file := filepath.Join(t.TempDir(), "resource.bin")
	if err := os.WriteFile(file, []byte("paper"), 0o600); err != nil {
		t.Fatal(err)
	}
	reader, info, err := (FileResourceLoader{}).OpenResource(t.Context(), ResourceAttachment, file)
	if err != nil {
		t.Fatal(err)
	}
	payload, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil || closeErr != nil || string(payload) != "paper" || info.Size != int64(len(payload)) || info.ModTime.IsZero() {
		t.Fatalf("resource = %q info=%+v read=%v close=%v", payload, info, readErr, closeErr)
	}
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, _, err := (FileResourceLoader{}).OpenResource(canceled, ResourceAttachment, file); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled resource open = %v", err)
	}
	if _, _, err := (FileResourceLoader{}).OpenResource(t.Context(), ResourceAttachment, filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing resource was accepted")
	}
}
