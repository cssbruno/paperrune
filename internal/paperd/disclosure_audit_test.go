// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestDisclosureDenialsAreHashOnlyAuditedAndEmitted(t *testing.T) {
	var emitted []DisclosureAuditEntry
	workspace, err := NewWorkspaceWithOptions(WorkspaceOptions{
		DisclosureDomain:    DisclosureRestricted,
		DisclosureAuditSink: func(entry DisclosureAuditEntry) { emitted = append(emitted, entry) },
	})
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.PaperCreate(PaperCreateRequest{File: "audit.paper", Source: workspaceFixture})
	if err != nil {
		t.Fatal(err)
	}
	_, err = workspace.PaperOpen(PaperOpenRequest{Revision: created.Revision.Handle, ExpectedDigest: created.Revision.Revision, Mode: CapabilityRead, DisclosureDomain: DisclosureDomain("customer-secret-domain")})
	if !errors.Is(err, ErrDisclosureDenied) {
		t.Fatalf("disclosure denial = %v", err)
	}
	audit, err := workspace.DisclosureAudit(8)
	if err != nil || len(audit) != 1 || len(emitted) != 1 || audit[0] != emitted[0] || audit[0].Reason != "domain_mismatch" ||
		audit[0].RequestedHash != disclosureIdentityHash("customer-secret-domain") || audit[0].ExpectedHash != disclosureIdentityHash(string(DisclosureRestricted)) {
		t.Fatalf("audit/emitted = %#v / %#v / %v", audit, emitted, err)
	}
	encoded, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("customer-secret-domain")) || bytes.Contains(encoded, []byte(DisclosureRestricted)) {
		t.Fatalf("disclosure audit leaked raw domains: %s", encoded)
	}
}

func TestDisclosureAuditSinkPanicCannotChangeDenial(t *testing.T) {
	workspace, err := NewWorkspaceWithOptions(WorkspaceOptions{DisclosureDomain: DisclosureRestricted, DisclosureAuditSink: func(DisclosureAuditEntry) { panic("sink secret") }})
	if err != nil {
		t.Fatal(err)
	}
	created, err := workspace.PaperCreate(PaperCreateRequest{File: "audit.paper", Source: workspaceFixture})
	if err != nil {
		t.Fatal(err)
	}
	_, err = workspace.PaperOpen(PaperOpenRequest{Revision: created.Revision.Handle, ExpectedDigest: created.Revision.Revision, Mode: CapabilityRead, DisclosureDomain: DisclosurePublic})
	if !errors.Is(err, ErrDisclosureDenied) {
		t.Fatalf("denial after sink panic = %v", err)
	}
	audit, _ := workspace.DisclosureAudit(1)
	if len(audit) != 1 {
		t.Fatalf("audit after sink panic = %#v", audit)
	}
}
