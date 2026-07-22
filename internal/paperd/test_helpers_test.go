// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"errors"

	"github.com/cssbruno/paperrune/internal/paperedit"
)

const slotMutationFixture = "document @report:\n" +
	"  component @card:\n" +
	"    slot @content:\n" +
	"      type: \"text\"\n" +
	"      required: true\n" +
	"  page @sheet:\n" +
	"    body @body:\n" +
	"      use @instance:\n" +
	"        component: \"@card\"\n"

func valuePointer(value paperedit.Value) *paperedit.Value { return &value }

func errorCode(err error) string {
	var workspaceErr *Error
	if errors.As(err, &workspaceErr) {
		return workspaceErr.Code
	}
	return ""
}
