// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type playgroundDataEditResult struct {
	Applied     bool   `json:"applied"`
	Data        string `json:"data"`
	JSONPointer string `json:"json_pointer"`
}

func playgroundEditJSONData(data, pointer, text string) (playgroundDataEditResult, error) {
	parts, err := playgroundJSONPointerParts(pointer)
	if err != nil {
		return playgroundDataEditResult{}, err
	}
	if len(parts) == 0 {
		return playgroundDataEditResult{}, errors.New("paper-studio-wasm: editText cannot replace the JSON document root")
	}

	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return playgroundDataEditResult{}, fmt.Errorf("paper-studio-wasm: editText data is invalid: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return playgroundDataEditResult{}, errors.New("paper-studio-wasm: editText data contains multiple JSON values")
		}
		return playgroundDataEditResult{}, fmt.Errorf("paper-studio-wasm: editText data is invalid: %w", err)
	}
	if _, ok := document.(map[string]any); !ok {
		return playgroundDataEditResult{}, errors.New("paper-studio-wasm: editText data must be a JSON object")
	}

	parent := document
	for _, part := range parts[:len(parts)-1] {
		parent, err = playgroundJSONMember(parent, part)
		if err != nil {
			return playgroundDataEditResult{}, fmt.Errorf("paper-studio-wasm: editText pointer %s does not resolve", pointer)
		}
	}
	leaf := parts[len(parts)-1]
	switch value := parent.(type) {
	case map[string]any:
		current, exists := value[leaf]
		if !exists {
			return playgroundDataEditResult{}, fmt.Errorf("paper-studio-wasm: editText pointer %s does not resolve", pointer)
		}
		value[leaf], err = playgroundJSONTextValue(current, text)
	case []any:
		index, indexErr := playgroundJSONArrayIndex(leaf, len(value))
		if indexErr != nil {
			return playgroundDataEditResult{}, fmt.Errorf("paper-studio-wasm: editText pointer %s does not resolve", pointer)
		}
		value[index], err = playgroundJSONTextValue(value[index], text)
	default:
		return playgroundDataEditResult{}, fmt.Errorf("paper-studio-wasm: editText pointer %s does not resolve", pointer)
	}
	if err != nil {
		return playgroundDataEditResult{}, err
	}

	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(document); err != nil {
		return playgroundDataEditResult{}, fmt.Errorf("paper-studio-wasm: encode edited JSON data: %w", err)
	}
	return playgroundDataEditResult{Applied: true, Data: encoded.String(), JSONPointer: pointer}, nil
}

func playgroundJSONPointerParts(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("paper-studio-wasm: editText jsonPointer must be an RFC 6901 pointer")
	}
	raw := strings.Split(pointer[1:], "/")
	parts := make([]string, len(raw))
	for index, part := range raw {
		for cursor := 0; cursor < len(part); cursor++ {
			if part[cursor] != '~' {
				continue
			}
			if cursor+1 >= len(part) || (part[cursor+1] != '0' && part[cursor+1] != '1') {
				return nil, errors.New("paper-studio-wasm: editText jsonPointer has an invalid escape")
			}
			cursor++
		}
		parts[index] = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func playgroundJSONMember(parent any, part string) (any, error) {
	switch value := parent.(type) {
	case map[string]any:
		child, exists := value[part]
		if !exists {
			return nil, errors.New("missing object member")
		}
		return child, nil
	case []any:
		index, err := playgroundJSONArrayIndex(part, len(value))
		if err != nil {
			return nil, err
		}
		return value[index], nil
	default:
		return nil, errors.New("scalar has no child member")
	}
}

func playgroundJSONArrayIndex(raw string, length int) (int, error) {
	if raw == "" || (len(raw) > 1 && raw[0] == '0') {
		return 0, errors.New("invalid array index")
	}
	index, err := strconv.Atoi(raw)
	if err != nil || index < 0 || index >= length {
		return 0, errors.New("array index is outside the value")
	}
	return index, nil
}

func playgroundJSONTextValue(current any, text string) (any, error) {
	switch current.(type) {
	case string:
		return text, nil
	case json.Number:
		var number json.Number
		if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &number); err != nil {
			return nil, errors.New("paper-studio-wasm: editText requires a valid number for this binding")
		}
		return number, nil
	case bool:
		switch text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, errors.New("paper-studio-wasm: editText requires true or false for this binding")
		}
	case nil:
		if text == "null" {
			return nil, nil
		}
		return text, nil
	default:
		return nil, errors.New("paper-studio-wasm: editText cannot replace object or list bindings")
	}
}
