// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package papercompile

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"testing"
)

func TestAssetCatalogIsCanonicalContentAddressedAndDetached(t *testing.T) {
	png, _ := base64.StdEncoding.DecodeString(paperImagePNG)
	digest := sha256.Sum256(png)
	original := png[8]
	input := []AssetResource{{Name: "hero-image", MediaType: "image/png", Digest: hex.EncodeToString(digest[:]), Data: png}}
	catalog, err := NewAssetCatalog(input)
	if err != nil || catalog.Len() != 1 {
		t.Fatalf("NewAssetCatalog() = %#v, %v", catalog, err)
	}
	input[0].Data[8] = 'X'
	first, ok := catalog.Resolve("hero-image")
	if !ok || first.Data[8] != original {
		t.Fatalf("catalog retained caller bytes: %#v", first)
	}
	first.Data[8] ^= 0xff
	second, _ := catalog.Resolve("hero-image")
	if second.Data[8] != original {
		t.Fatal("Resolve exposed catalog storage")
	}
	if missing, ok := catalog.Resolve("missing"); ok || len(missing.Data) != 0 {
		t.Fatalf("missing resolve = %#v, %v", missing, ok)
	}
}

func TestAssetCatalogFontMetadataLookupAvoidsExposingOrCopyingFontBytes(t *testing.T) {
	regularData := []byte{0, 1, 0, 0, 1}
	boldData := []byte{0, 1, 0, 0, 2}
	regularDigest := sha256.Sum256(regularData)
	boldDigest := sha256.Sum256(boldData)
	catalog, err := NewAssetCatalog([]AssetResource{
		{Name: "body-regular", MediaType: "font/ttf", Digest: hex.EncodeToString(regularDigest[:]), Data: regularData, Family: "Specimen Sans", Style: "normal", Weight: 400},
		{Name: "body-bold", MediaType: "font/ttf", Digest: hex.EncodeToString(boldDigest[:]), Data: boldData, Family: "Specimen Sans", Style: "normal", Weight: 700},
	})
	if err != nil {
		t.Fatal(err)
	}
	if name, ok := catalog.resolveFontName("Specimen Sans"); !ok || name != "body-regular" {
		t.Fatalf("family metadata lookup = %q, %v", name, ok)
	}
	if name, ok := catalog.resolveFontName("body-bold"); !ok || name != "body-bold" {
		t.Fatalf("exact metadata lookup = %q, %v", name, ok)
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		if _, ok := catalog.resolveFontName("Specimen Sans"); !ok {
			t.Fatal("font disappeared during metadata lookup")
		}
	}); allocations != 0 {
		t.Fatalf("metadata-only font lookup allocations = %v, want 0", allocations)
	}

	resolved, ok := catalog.ResolveFont("Specimen Sans")
	if !ok {
		t.Fatal("public font lookup failed")
	}
	resolved.Data[0] ^= 0xff
	again, _ := catalog.ResolveFont("Specimen Sans")
	if again.Data[0] != regularData[0] {
		t.Fatal("public ResolveFont exposed catalog storage")
	}
}

func TestAssetCatalogRejectsAmbiguousUnsafeAndUnverifiedResources(t *testing.T) {
	png, _ := base64.StdEncoding.DecodeString(paperImagePNG)
	digest := sha256.Sum256(png)
	valid := AssetResource{Name: "hero", MediaType: "image/png", Digest: hex.EncodeToString(digest[:]), Data: png}
	tests := []struct {
		name string
		edit func(*AssetResource)
	}{
		{"path", func(resource *AssetResource) { resource.Name = "../hero" }},
		{"uppercase", func(resource *AssetResource) { resource.Name = "Hero" }},
		{"media", func(resource *AssetResource) { resource.MediaType = "image/svg+xml" }},
		{"digest", func(resource *AssetResource) { resource.Digest = string(make([]byte, 64)) }},
		{"signature", func(resource *AssetResource) {
			resource.Data = []byte("not a png")
			sum := sha256.Sum256(resource.Data)
			resource.Digest = hex.EncodeToString(sum[:])
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resource := valid
			resource.Data = append([]byte(nil), valid.Data...)
			test.edit(&resource)
			if catalog, err := NewAssetCatalog([]AssetResource{resource}); !errors.Is(err, ErrAssetCatalog) || catalog.Len() != 0 {
				t.Fatalf("invalid catalog = %#v, %v", catalog, err)
			}
		})
	}
	if catalog, err := NewAssetCatalog([]AssetResource{valid, valid}); !errors.Is(err, ErrAssetCatalog) || catalog.Len() != 0 {
		t.Fatalf("duplicate catalog = %#v, %v", catalog, err)
	}
	oversized := append([]byte(nil), png...)
	binary.BigEndian.PutUint32(oversized[16:20], 100_000)
	binary.BigEndian.PutUint32(oversized[20:24], 100_000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	oversizedDigest := sha256.Sum256(oversized)
	resource := AssetResource{Name: "huge", MediaType: "image/png", Digest: hex.EncodeToString(oversizedDigest[:]), Data: oversized}
	if catalog, err := NewAssetCatalog([]AssetResource{resource}); !errors.Is(err, ErrAssetCatalog) || catalog.Len() != 0 {
		t.Fatalf("decoded-pixel bomb catalog = %#v, %v", catalog, err)
	}
}
