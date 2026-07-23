package papercompile_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserFacingExamplesDoNotMixBindAndText(t *testing.T) {
	examplesRoot := filepath.Join("..", "..", "examples")
	err := filepath.WalkDir(examplesRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".paper" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Split(string(contents), "\n")
		for index, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "bind:") {
				continue
			}
			propertyIndent := len(line) - len(strings.TrimLeft(line, " \t"))
			for cursor := index - 1; cursor >= 0; cursor-- {
				if mixedExampleProperty(lines[cursor], propertyIndent) {
					t.Errorf("%s:%d mixes bind with text at line %d", path, index+1, cursor+1)
					break
				}
				if examplePropertyBlockEnded(lines[cursor], propertyIndent) {
					break
				}
			}
			for cursor := index + 1; cursor < len(lines); cursor++ {
				if mixedExampleProperty(lines[cursor], propertyIndent) {
					t.Errorf("%s:%d mixes bind with text at line %d", path, index+1, cursor+1)
					break
				}
				if examplePropertyBlockEnded(lines[cursor], propertyIndent) {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func mixedExampleProperty(line string, propertyIndent int) bool {
	return exampleLineIndent(line) == propertyIndent && strings.HasPrefix(strings.TrimSpace(line), "text:")
}

func examplePropertyBlockEnded(line string, propertyIndent int) bool {
	return strings.TrimSpace(line) != "" && exampleLineIndent(line) < propertyIndent
}

func exampleLineIndent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}
