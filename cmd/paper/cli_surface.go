// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	"golang.org/x/term"
)

type commandHelp struct {
	summary  string
	usage    string
	examples []string
}

var commandHelpText = map[string]commandHelp{
	"init":     {"Create a runnable Paper project from a built-in template.", "paper init [blank|invoice|report|table-report|letter] DIR", []string{"paper init invoice my-invoice", "paper init blank ."}},
	"fmt":      {"Format Paper source into its canonical representation.", "paper fmt [options] FILE", []string{"paper fmt -w invoice.paper", "paper fmt - < invoice.paper"}},
	"check":    {"Parse, compile, validate data, and plan a document.", "paper check [options] [FILE]", []string{"paper check", "paper check --data data.json invoice.paper"}},
	"render":   {"Render a Paper document as deterministic PDF or standalone HTML.", "paper render [options] [FILE]", []string{"paper render", "paper render -o report.pdf report.paper", "paper render -o - report.paper > report.pdf"}},
	"studio":   {"Open Paper Studio for a source file or project.", "paper studio [paper-studio options] [FILE|DIR]", []string{"paper studio", "paper studio invoice.paper", "paper studio --no-open ."}},
	"capture":  {"Capture deterministic SVG evidence from a planned document.", "paper capture [options] FILE", []string{"paper capture -o page.svg report.paper", "paper capture --json --contact-sheet report.paper"}},
	"explain":  {"Inspect selected nodes, fragments, instances, or pages.", "paper explain [options] FILE", []string{"paper explain --page 1 report.paper", "paper explain --key totals report.paper"}},
	"scenario": {"List scenarios or inspect one resolved fixture.", "paper scenario [options] FILE", []string{"paper scenario report.paper", "paper scenario --scenario enterprise --json report.paper"}},
	"version":  {"Print PaperRune version and build information.", "paper version [--json]", []string{"paper version", "paper version --json"}},
}

func printCommandHelp(w io.Writer, name string, set *flag.FlagSet) {
	help := commandHelpText[name]
	_, _ = fmt.Fprintf(w, "%s\n\nUsage:\n  %s\n", help.summary, help.usage)
	if set != nil {
		_, _ = fmt.Fprintln(w, "\nOptions:")
		set.PrintDefaults()
	}
	if len(help.examples) != 0 {
		_, _ = fmt.Fprintln(w, "\nExamples:")
		for _, example := range help.examples {
			_, _ = fmt.Fprintf(w, "  %s\n", example)
		}
	}
}

func unknownCommand(name string, commands map[string]func([]string, io.Reader, io.Writer, io.Writer) int, stderr io.Writer) int {
	names := make([]string, 0, len(commands))
	for candidate := range commands {
		names = append(names, candidate)
	}
	sort.Strings(names)
	best, distance := "", len(name)+1
	for _, candidate := range names {
		if value := editDistance(name, candidate); value < distance {
			best, distance = candidate, value
		}
	}
	_, _ = fmt.Fprintf(stderr, "paper: unknown command %q\n", name)
	if best != "" && distance <= 3 {
		_, _ = fmt.Fprintf(stderr, "Did you mean %q?\n", best)
	}
	_, _ = fmt.Fprintln(stderr, "Run \"paper help\" to list commands.")
	return exitUsage
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for i := range previous {
		previous[i] = i
	}
	for i, l := range left {
		current := make([]int, len(right)+1)
		current[0] = i + 1
		j := 0
		for _, r := range right {
			j++
			cost := 0
			if l != r {
				cost = 1
			}
			current[j] = min(current[j-1]+1, previous[j]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(previous)-1]
}

func resolvedVersion() string {
	if version != "" && version != "devel" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "devel"
}

func runVersion(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	set := flags("version", stderr)
	jsonMode := set.Bool("json", false, "write version information as JSON")
	if err := set.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if set.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "paper version: expected no arguments")
		return exitUsage
	}
	value := resolvedVersion()
	if *jsonMode {
		return writeJSON(stdout, stderr, struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		}{"paper", value})
	}
	_, _ = fmt.Fprintf(stdout, "paper %s\n", value)
	return exitOK
}

func outputIsTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func defaultOutputFile(source, format string) string {
	extension := filepath.Ext(source)
	return strings.TrimSuffix(source, extension) + "." + format
}

func runStudio(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		printCommandHelp(stdout, "studio", nil)
		return exitOK
	}
	resolved, err := resolveStudioArgs(args)
	if err != nil {
		return commandError(false, stdout, stderr, "studio", err)
	}
	executable, err := studioExecutable()
	if err != nil {
		return commandError(false, stdout, stderr, "studio", err)
	}
	command := exec.Command(executable, resolved...) // #nosec G204,G702 -- executable is a fixed sibling or PATH lookup; arguments are passed without a shell.
	command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
	if err := command.Run(); err != nil {
		var exited *exec.ExitError
		if errors.As(err, &exited) {
			return exited.ExitCode()
		}
		return commandError(false, stdout, stderr, "studio", err)
	}
	return exitOK
}

func resolveStudioArgs(args []string) ([]string, error) {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if studioOptionTakesValue(arg) {
			index++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			info, err := os.Stat(arg) // #nosec G703 -- this explicitly supplied CLI path is only classified as a directory before constrained project loading.
			if err == nil && info.IsDir() {
				project, loadErr := loadDiscoveredProject(arg)
				if loadErr != nil {
					return nil, loadErr
				}
				return studioProjectArgs(replaceArgument(args, arg, project.source), project), nil
			}
			return args, nil
		}
	}
	project, err := loadDiscoveredProject(".")
	if err != nil {
		return nil, errors.New("no FILE was provided and paper.project.json was not found")
	}
	return studioProjectArgs(append(args, project.source), project), nil
}

func studioOptionTakesValue(argument string) bool {
	if strings.Contains(argument, "=") {
		return false
	}
	switch argument {
	case "-addr", "--addr", "-scenario", "--scenario", "-assets", "--assets", "-asset-root", "--asset-root":
		return true
	default:
		return false
	}
}

func studioProjectArgs(args []string, project *resolvedProject) []string {
	if project.config.Assets == "" {
		return args
	}
	for _, argument := range args {
		if argument == "-assets" || argument == "--assets" || strings.HasPrefix(argument, "-assets=") || strings.HasPrefix(argument, "--assets=") {
			return args
		}
	}
	return append([]string{"--assets", projectPath(project.dir, project.config.Assets)}, args...)
}

func replaceArgument(args []string, old, replacement string) []string {
	result := append([]string(nil), args...)
	for index, arg := range result {
		if arg == old {
			result[index] = replacement
			break
		}
	}
	return result
}

func studioExecutable() (string, error) {
	name := "paper-studio"
	if os.PathSeparator == '\\' {
		name += ".exe"
	}
	if current, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(current), name)
		if info, statErr := os.Stat(sibling); statErr == nil && !info.IsDir() {
			return sibling, nil
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found, nil
	}
	return "", errors.New("paper-studio executable not found; install the PaperRune release bundle or run paper-studio separately")
}

func marshalIndent(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	return append(data, '\n'), err
}
