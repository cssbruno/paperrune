// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

//go:build !js || !wasm

package main

import (
	"encoding/json"
	"testing"
)

func TestPlaygroundEditJSONDataPreservesBindingTypes(t *testing.T) {
	source := `{"report":{"title":"Draft","results":[{"value":5.8,"approved":false}],"note":null}}`
	tests := []struct {
		pointer string
		text    string
		want    any
	}{
		{pointer: "/report/title", text: "Published", want: "Published"},
		{pointer: "/report/results/0/value", text: "6.2", want: 6.2},
		{pointer: "/report/results/0/approved", text: "true", want: true},
		{pointer: "/report/note", text: "reviewed", want: "reviewed"},
	}
	for _, test := range tests {
		result, err := playgroundEditJSONData(source, test.pointer, test.text)
		if err != nil {
			t.Fatalf("%s: %v", test.pointer, err)
		}
		if !result.Applied || result.JSONPointer != test.pointer {
			t.Fatalf("%s: result = %#v", test.pointer, result)
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(result.Data), &decoded); err != nil {
			t.Fatal(err)
		}
		value, err := playgroundDecodedJSONValue(decoded, test.pointer)
		if err != nil || value != test.want {
			t.Fatalf("%s: value = %#v, %v; want %#v", test.pointer, value, err, test.want)
		}
	}
}

func TestPlaygroundEditJSONDataChangesOnlySelectedScalar(t *testing.T) {
	source := "{\n\t\"untouched\": \"keep  spacing & <markup>\",\n\t\"report\": { \"title\" : \"Draft\", \"score\": 5.80 },\n\t\"tail\": true\n}\n"
	result, err := playgroundEditJSONData(source, "/report/title", "Published")
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n\t\"untouched\": \"keep  spacing & <markup>\",\n\t\"report\": { \"title\" : \"Published\", \"score\": 5.80 },\n\t\"tail\": true\n}\n"
	if result.Data != want {
		t.Fatalf("edited data reformatted unrelated bytes\n got: %q\nwant: %q", result.Data, want)
	}
}

func TestPlaygroundEditJSONDataRejectsUnsafeOrIncompatibleEdits(t *testing.T) {
	source := `{"report":{"items":[{"name":"one"}],"object":{"name":"value"},"active":false}}`
	for _, test := range []struct {
		pointer string
		text    string
	}{
		{pointer: "", text: "replacement"},
		{pointer: "report/active", text: "true"},
		{pointer: "/report/items/01/name", text: "two"},
		{pointer: "/report/missing", text: "value"},
		{pointer: "/report/object", text: "value"},
		{pointer: "/report/active", text: "yes"},
		{pointer: "/report/~2bad", text: "value"},
	} {
		if _, err := playgroundEditJSONData(source, test.pointer, test.text); err == nil {
			t.Fatalf("playgroundEditJSONData(%q, %q) succeeded", test.pointer, test.text)
		}
	}
}

func playgroundDecodedJSONValue(document any, pointer string) (any, error) {
	parts, err := playgroundJSONPointerParts(pointer)
	if err != nil {
		return nil, err
	}
	return playgroundJSONValue(document, parts)
}
