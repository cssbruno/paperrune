// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteSBOMUsesReleaseIdentityAndSortedModules(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "paper.cdx.json")
	if err := writeSBOM(binary, output, "paper", "v1.2.3"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var bom billOfMaterials
	if err := json.Unmarshal(data, &bom); err != nil {
		t.Fatal(err)
	}
	if bom.BOMFormat != "CycloneDX" || bom.SpecVersion != "1.6" || bom.Metadata.Component.Name != "paper" || bom.Metadata.Component.Version != "v1.2.3" {
		t.Fatalf("unexpected SBOM metadata: %+v", bom)
	}
	for index := 1; index < len(bom.Components); index++ {
		if bom.Components[index-1].Name > bom.Components[index].Name {
			t.Fatalf("components are not sorted: %q before %q", bom.Components[index-1].Name, bom.Components[index].Name)
		}
	}
}
