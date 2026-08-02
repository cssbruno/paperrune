// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package document

import (
	"strings"

	"github.com/cssbruno/paperrune/internal/layout"
)

func textAlign(align string) string {
	switch strings.ToUpper(align) {
	case "C", "CENTER":
		return "C"
	case "R", "RIGHT":
		return "R"
	default:
		return "L"
	}
}

func signatureColumnText(column layout.SignatureColumn) string {
	lines := make([]string, 0, 3+len(column.Metadata))
	if column.Label != "" {
		lines = append(lines, column.Label)
	}
	if column.Name != "" && column.Name != column.Label {
		lines = append(lines, column.Name)
	}
	if column.Role != "" && column.Role != column.Label {
		lines = append(lines, column.Role)
	}
	for _, field := range column.Metadata {
		if field.Label == "" && field.Value == "" {
			continue
		}
		switch {
		case field.Label == "":
			lines = append(lines, field.Value)
		case field.Value == "":
			lines = append(lines, field.Label)
		default:
			lines = append(lines, field.Label+": "+field.Value)
		}
	}
	return strings.Join(lines, "\n")
}

func metadataFieldText(field layout.MetadataField) string {
	if field.Value == "" {
		return field.Label
	}
	return field.Label + ": " + field.Value
}
