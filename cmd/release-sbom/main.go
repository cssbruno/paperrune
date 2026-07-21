// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

// Command release-sbom creates a compact CycloneDX inventory from Go build
// information embedded in a PaperRune release executable.
package main

import (
	"debug/buildinfo"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
)

type property struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type component struct {
	Type       string     `json:"type"`
	Name       string     `json:"name"`
	Version    string     `json:"version,omitempty"`
	Properties []property `json:"properties,omitempty"`
}

type billOfMaterials struct {
	BOMFormat   string `json:"bomFormat"`
	SpecVersion string `json:"specVersion"`
	Version     int    `json:"version"`
	Metadata    struct {
		Component component `json:"component"`
	} `json:"metadata"`
	Components []component `json:"components,omitempty"`
}

func main() {
	binary := flag.String("binary", "", "release executable to inspect")
	output := flag.String("output", "", "CycloneDX JSON file to create")
	name := flag.String("name", "", "released application name")
	version := flag.String("version", "", "released application version")
	flag.Parse()
	if *binary == "" || *output == "" || *name == "" || *version == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: release-sbom -binary FILE -output FILE.cdx.json -name NAME -version VERSION")
		os.Exit(2)
	}
	if err := writeSBOM(*binary, *output, *name, *version); err != nil {
		fmt.Fprintf(os.Stderr, "release-sbom: %v\n", err)
		os.Exit(1)
	}
}

func writeSBOM(binary, output, name, version string) error {
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return err
	}
	if info.Main.Path == "" {
		return errors.New("binary has no Go module build information")
	}
	bom := billOfMaterials{BOMFormat: "CycloneDX", SpecVersion: "1.6", Version: 1}
	bom.Metadata.Component = component{Type: "application", Name: name, Version: version,
		Properties: []property{{Name: "cdx:gomod:module", Value: info.Main.Path}}}
	for _, dependency := range info.Deps {
		bom.Components = append(bom.Components, moduleComponent(dependency.Path, dependency.Version, dependency.Sum, "library"))
	}
	sort.Slice(bom.Components, func(i, j int) bool { return bom.Components[i].Name < bom.Components[j].Name })
	encoded, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(output, encoded, 0o644) // #nosec G306 -- release SBOMs are intentionally public artifacts.
}

func moduleComponent(path, version, sum, kind string) component {
	item := component{Type: kind, Name: path, Version: version}
	if sum != "" {
		item.Properties = []property{{Name: "cdx:gomod:checksum", Value: sum}}
	}
	return item
}
