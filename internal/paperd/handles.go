// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

package paperd

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

type DisclosureDomain string

const (
	DisclosureProject    DisclosureDomain = "project"
	DisclosureRestricted DisclosureDomain = "restricted"
	DisclosurePublic     DisclosureDomain = "public"
)

type handleKind uint8

const (
	handleRevision handleKind = iota + 1
	handleCandidate
	handleMutationAuthority
	handlePlan
	handleOpen
)

type handleCapability uint8

const (
	capabilityRead handleCapability = iota + 1
	capabilityEdit
	capabilityRender
	capabilityAuthorize
)

type revocationReason uint8

const (
	revokedExplicitly revocationReason = iota + 1
	revokedExpired
)

type revocationRecord struct {
	reason revocationReason
	at     time.Time
}

var nextHandleNonce atomic.Uint64

func normalizeDisclosureDomain(domain DisclosureDomain) (DisclosureDomain, uint64, error) {
	if domain == "" {
		domain = DisclosureProject
	}
	if !utf8.ValidString(string(domain)) || len(domain) > MaxQueryBytesHard || strings.TrimSpace(string(domain)) != string(domain) {
		return "", 0, workspaceError("INVALID_DISCLOSURE_DOMAIN", "disclosure domain must be bounded valid UTF-8 without surrounding whitespace", ErrDisclosureDenied)
	}
	sum := sha256.Sum256([]byte("paperd/disclosure/v1\x00" + string(domain)))
	return domain, binary.BigEndian.Uint64(sum[:8]), nil
}

func (w *Workspace) newHandle(kind handleKind, capability handleCapability, serial uint64) scopedHandle {
	return scopedHandle{scope: w.scope, serial: serial, nonce: nextHandleNonce.Add(1), domain: w.disclosureTag, kind: kind, capability: capability}
}

func (w *Workspace) expiresAt(ttl time.Duration) time.Time { return w.now().Add(ttl) }

func (w *Workspace) validateHandle(value scopedHandle, kind handleKind, capability handleCapability, allowEitherOpenMode bool) error {
	if value.serial == 0 || value.nonce == 0 {
		return workspaceError("INVALID_HANDLE", "handle is unavailable", ErrInvalidHandle)
	}
	if value.scope != w.scope || value.domain != w.disclosureTag {
		return workspaceError("WRONG_WORKSPACE", "handle is unavailable", ErrWrongWorkspace)
	}
	if value.kind != kind || (!allowEitherOpenMode && value.capability != capability) ||
		(allowEitherOpenMode && value.capability != capabilityRead && value.capability != capabilityEdit) {
		return workspaceError("INVALID_HANDLE", "handle is unavailable", ErrInvalidHandle)
	}
	return nil
}

func (w *Workspace) unavailableHandle(value scopedHandle, notFound error) error {
	if tombstone, ok := w.revocations[value]; ok {
		if tombstone.reason == revokedExpired {
			if value.kind == handlePlan {
				return workspaceError("HANDLE_EXPIRED", "handle is unavailable", errors.Join(ErrHandleExpired, ErrPlanExpired, ErrPlanNotFound))
			}
			return workspaceError("HANDLE_EXPIRED", "handle is unavailable", errors.Join(ErrHandleExpired, notFound))
		}
		return workspaceError("HANDLE_REVOKED", "handle is unavailable", errors.Join(ErrHandleRevoked, notFound))
	}
	return workspaceError("HANDLE_NOT_FOUND", "handle is unavailable", notFound)
}

func (w *Workspace) ensureLive(_ scopedHandle, expires time.Time) error {
	if !expires.After(w.now()) {
		return workspaceError("HANDLE_EXPIRED", "handle is unavailable", ErrHandleExpired)
	}
	return nil
}

func (w *Workspace) recordRevocationLocked(value scopedHandle, reason revocationReason, now time.Time) {
	if _, exists := w.revocations[value]; exists {
		return
	}
	for len(w.revocationOrder) >= w.limits.MaxRevocations {
		oldest := w.revocationOrder[0]
		w.revocationOrder = w.revocationOrder[1:]
		delete(w.revocations, oldest)
	}
	w.revocations[value] = revocationRecord{reason: reason, at: now}
	w.revocationOrder = append(w.revocationOrder, value)
}

func (w *Workspace) PruneExpiredHandles() int {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pruneExpiredHandlesLocked(w.now())
}

func (w *Workspace) pruneExpiredHandlesLocked(now time.Time) int {
	removed := 0
	prune := func(value scopedHandle, expires time.Time, remove func()) {
		if expires.After(now) {
			return
		}
		remove()
		w.recordRevocationLocked(value, revokedExpired, now)
		removed++
	}
	for serial, record := range w.revisions {
		prune(record.handle.value, record.expires, func() { delete(w.revisions, serial) })
	}
	for serial, record := range w.candidates {
		prune(record.handle.value, record.expires, func() { delete(w.candidates, serial) })
	}
	for serial, record := range w.mutationAuthorities {
		prune(record.handle.value, record.expires, func() { delete(w.mutationAuthorities, serial) })
	}
	for serial, record := range w.plans {
		prune(record.handle.value, record.expires, func() { delete(w.plans, serial) })
	}
	for serial, record := range w.opens {
		prune(record.handle.value, record.expires, func() { delete(w.opens, serial) })
	}
	return removed
}

func (w *Workspace) revoke(value scopedHandle, kind handleKind, capability handleCapability, remove func()) error {
	if w == nil {
		return workspaceError("INVALID_WORKSPACE", "workspace is nil", ErrInvalidHandle)
	}
	if err := w.validateHandle(value, kind, capability, false); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	remove()
	w.recordRevocationLocked(value, revokedExplicitly, w.now())
	return nil
}

func (w *Workspace) RevokeMutationAuthority(handle MutationAuthorityHandle) error {
	return w.revoke(handle.value, handleMutationAuthority, capabilityAuthorize, func() { delete(w.mutationAuthorities, handle.value.serial) })
}
