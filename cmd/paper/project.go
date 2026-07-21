// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const projectFileName = "paper.project.json"

type projectConfig struct {
	Source string `json:"source"`
	Data   string `json:"data,omitempty"`
	Output string `json:"output,omitempty"`
	Format string `json:"format,omitempty"`
	Locale string `json:"locale,omitempty"`
	Assets string `json:"assets,omitempty"`
}

type resolvedProject struct {
	config projectConfig
	file   string
	dir    string
	source string
}

func parseProjectFile(set *flag.FlagSet, args []string) (string, *resolvedProject, int) {
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return "", nil, exitOK
		}
		return "", nil, exitUsage
	}
	if set.NArg() > 1 {
		_, _ = fmt.Fprintf(set.Output(), "paper %s: expected at most one FILE\n", strings.TrimPrefix(set.Name(), "paper "))
		return "", nil, exitUsage
	}
	if set.NArg() == 1 {
		return set.Arg(0), nil, -1
	}
	if source := strings.TrimSpace(os.Getenv("PAPER_SOURCE")); source != "" {
		return source, nil, -1
	}
	project, err := loadDiscoveredProject(".")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, _ = fmt.Fprintf(set.Output(), "paper %s: no FILE was provided and %s was not found\n", strings.TrimPrefix(set.Name(), "paper "), projectFileName)
		} else {
			_, _ = fmt.Fprintf(set.Output(), "paper %s: load project: %v\n", strings.TrimPrefix(set.Name(), "paper "), err)
		}
		return "", nil, exitUsage
	}
	return project.source, project, -1
}

func loadDiscoveredProject(start string) (*resolvedProject, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	for {
		candidate := filepath.Join(abs, projectFileName)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return loadProject(candidate)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return nil, statErr
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return nil, os.ErrNotExist
		}
		abs = parent
	}
}

func loadProject(name string) (*resolvedProject, error) {
	data, err := readBoundedFile(name, strings.NewReader(""), 1<<20, "project file")
	if err != nil {
		return nil, err
	}
	var config projectConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("decode %s: %w", name, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("decode %s: trailing JSON", name)
	}
	if strings.TrimSpace(config.Source) == "" {
		return nil, fmt.Errorf("%s: source is required", name)
	}
	if config.Format != "" && config.Format != "pdf" && config.Format != "html" {
		return nil, fmt.Errorf("%s: format must be pdf or html", name)
	}
	abs, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(abs)
	return &resolvedProject{config: config, file: abs, dir: dir, source: projectPath(dir, config.Source)}, nil
}

func projectPath(dir, value string) string {
	if value == "" || value == "-" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(dir, filepath.FromSlash(value))
}

func applyRenderProject(project *resolvedProject, set *flag.FlagSet, output, format *string, data dataOptions, assets assetOptions) {
	applyStringOption(set, "o", "PAPER_OUTPUT", projectValue(project, func(config projectConfig) string { return config.Output }), output, project)
	applyStringOption(set, "format", "PAPER_FORMAT", projectValue(project, func(config projectConfig) string { return config.Format }), format, nil)
	applyStringOption(set, "data", "PAPER_DATA", projectValue(project, func(config projectConfig) string { return config.Data }), data.file, project)
	applyStringOption(set, "locale", "PAPER_LOCALE", projectValue(project, func(config projectConfig) string { return config.Locale }), data.locale, nil)
	applyStringOption(set, "assets", "PAPER_ASSETS", projectValue(project, func(config projectConfig) string { return config.Assets }), assets.manifest, project)
}

func applyCheckProject(project *resolvedProject, set *flag.FlagSet, data dataOptions, assets assetOptions) {
	applyStringOption(set, "data", "PAPER_DATA", projectValue(project, func(config projectConfig) string { return config.Data }), data.file, project)
	applyStringOption(set, "locale", "PAPER_LOCALE", projectValue(project, func(config projectConfig) string { return config.Locale }), data.locale, nil)
	applyStringOption(set, "assets", "PAPER_ASSETS", projectValue(project, func(config projectConfig) string { return config.Assets }), assets.manifest, project)
}

func projectValue(project *resolvedProject, selectValue func(projectConfig) string) string {
	if project == nil {
		return ""
	}
	return selectValue(project.config)
}

func applyStringOption(set *flag.FlagSet, flagName, environment, projectDefault string, target *string, project *resolvedProject) {
	if flagWasSet(set, flagName) {
		return
	}
	value := strings.TrimSpace(os.Getenv(environment))
	if value == "" {
		value = projectDefault
		if project != nil {
			value = projectPath(project.dir, value)
		}
	}
	if value != "" {
		*target = value
	}
}

func flagWasSet(set *flag.FlagSet, name string) bool {
	found := false
	set.Visit(func(item *flag.Flag) {
		if item.Name == name {
			found = true
		}
	})
	return found
}

type initTemplate struct {
	source string
	data   string
}

var initTemplates = map[string]initTemplate{
	"blank": {`document @document:
  language: "en"
  schema blank:
    optional string title
  page @page:
    margin: 36pt
    size: "A4"
    body @content:
      heading @title:
        bind: "title"
        bind-required: false
        level: 1
        text: "Untitled document"
`, "{\n  \"title\": \"Untitled document\"\n}\n"},
	"invoice": {`document @invoice:
  language: "en"
  schema invoice_data:
    string customer
    string invoice_number
  page @page:
    margin: 36pt
    size: "A4"
    body @content:
      heading @title:
        level: 1
        text: "Invoice"
      paragraph @number:
        bind: "invoice_number"
        text: "Invoice number"
      paragraph @customer:
        bind: "customer"
        text: "Customer"
`, "{\n  \"customer\": \"Ada Lovelace\",\n  \"invoice_number\": \"INV-001\"\n}\n"},
	"report": {`document @report:
  language: "en"
  schema report_data:
    string title
    string summary
  page @page:
    margin: 36pt
    size: "A4"
    body @content:
      heading @title:
        bind: "title"
        level: 1
        text: "Report"
      paragraph @summary:
        bind: "summary"
        text: "Summary"
`, "{\n  \"title\": \"Quarterly report\",\n  \"summary\": \"Replace this text with your report summary.\"\n}\n"},
	"table-report": {`document @table_report:
  language: "en"
  schema report:
    string title
    list object items:
      max-items: 20
      string name
  page @page:
    margin: 36pt
    size: "A4"
    body @content:
      heading @title:
        bind: "title"
        level: 1
        text: "Table report"
      repeat @items:
        source: "items"
        instance-prefix: "items"
        max-items: 20
        paragraph @name:
          bind: "name"
          text: "Item"
`, "{\n  \"title\": \"Inventory\",\n  \"items\": [\n    {\"name\": \"First item\"},\n    {\"name\": \"Second item\"}\n  ]\n}\n"},
	"letter": {`document @letter:
  language: "en"
  schema letter_data:
    string recipient
    string message
  page @page:
    margin: 54pt
    size: "A4"
    body @content:
      heading @recipient:
        bind: "recipient"
        level: 2
        text: "Recipient"
      paragraph @message:
        bind: "message"
        text: "Message"
`, "{\n  \"recipient\": \"Ada Lovelace\",\n  \"message\": \"Write your letter here.\"\n}\n"},
}

func runInit(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	set := flags("init", stderr)
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if set.NArg() != 2 {
		_, _ = fmt.Fprintln(stderr, "paper init: expected TEMPLATE and DIR")
		return exitUsage
	}
	name, dir := set.Arg(0), set.Arg(1)
	template, ok := initTemplates[name]
	if !ok {
		return commandError(false, stdout, stderr, "init", fmt.Errorf("unknown template %q (choose blank, invoice, report, table-report, or letter)", name))
	}
	if err := createProject(dir, name, template); err != nil {
		return commandError(false, stdout, stderr, "init", err)
	}
	_, _ = fmt.Fprintf(stdout, "Created %s project in %s\nNext: cd %s && paper render\n", name, dir, dir)
	return exitOK
}

func createProject(dir, name string, template initTemplate) error {
	info, err := os.Stat(dir)
	if err == nil {
		if !info.IsDir() {
			return errors.New("destination exists and is not a directory")
		}
		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			return readErr
		}
		if len(entries) != 0 {
			return errors.New("destination directory is not empty")
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
	} else {
		return err
	}
	sourceName := name + ".paper"
	config := projectConfig{Source: sourceName, Data: "data.json", Output: filepath.ToSlash(filepath.Join("dist", name+".pdf")), Format: "pdf", Locale: "en"}
	manifest, err := marshalIndent(config)
	if err != nil {
		return err
	}
	readme := fmt.Sprintf("# %s\n\nRender this project with:\n\n```sh\npaper check\npaper render\npaper studio\n```\n", name)
	files := map[string][]byte{sourceName: []byte(template.source), "data.json": []byte(template.data), projectFileName: manifest, "README.md": []byte(readme)}
	for file, data := range files {
		if err := atomicWrite(filepath.Join(dir, file), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
