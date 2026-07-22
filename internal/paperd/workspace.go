// SPDX-License-Identifier: LicenseRef-PaperRune-Health-Sector-Restricted-1.0
// Copyright (c) 2026 cssBruno

// Package paperd provides bounded, in-process authoring operations for Paper
// Studio. Source digests and short-lived handles reject stale edits; paperd
// does not persist or own document history.
package paperd

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cssbruno/paperrune/document"
	"github.com/cssbruno/paperrune/internal/papercompile"
	"github.com/cssbruno/paperrune/internal/paperedit"
	"github.com/cssbruno/paperrune/internal/paperlang"
)

const (
	MaxSourceBytesHard          = paperedit.MaxSourceBytes
	MaxRevisionsHard            = 65536
	MaxCandidatesHard           = 4096
	MaxPlansHard                = 4096
	MaxOpenDocumentsHard        = 4096
	MaxNodesHard                = 100000
	MaxSearchResultsHard        = 256
	MaxQueryBytesHard           = 4096
	MaxRenderBytesHard          = 64 << 20
	MaxFileBytesHard            = 4096
	MaxPlanTTLHard              = 24 * time.Hour
	MaxContextBytesHard         = 4 << 20
	MaxStructuralWorkHard       = 1000000
	MaxRevocationsHard          = 65536
	MaxHandleTTLHard            = 24 * time.Hour
	MaxMutationAuthoritiesHard  = 4096
	MaxAuthorizationEffectsHard = 100000
	MaxAuthorizationAuditHard   = 65536
)

// Limits bounds both retained workspace state and individual operations.
// Zero fields receive conservative defaults in NewWorkspace.
type Limits struct {
	MaxSourceBytes          int
	MaxRevisions            int
	MaxCandidates           int
	MaxPlans                int
	MaxOpenDocuments        int
	MaxNodes                int
	MaxSearchResults        int
	MaxQueryBytes           int
	MaxRenderBytes          int
	MaxFileBytes            int
	MaxOperations           int
	MaxContextBytes         int
	MaxStructuralWork       int
	MaxRevocations          int
	MaxMutationAuthorities  int
	MaxAuthorizationEffects int
	MaxAuthorizationAudit   int
}

func DefaultLimits() Limits {
	return Limits{
		MaxSourceBytes: 1 << 20, MaxRevisions: 1024, MaxCandidates: 128, MaxPlans: 128, MaxOpenDocuments: 128,
		MaxNodes: 20000, MaxSearchResults: 128, MaxQueryBytes: 1024,
		MaxRenderBytes: 16 << 20, MaxFileBytes: 1024, MaxOperations: 128,
		MaxContextBytes: 64 << 10, MaxStructuralWork: 100000, MaxRevocations: 1024,
		MaxMutationAuthorities: 128, MaxAuthorizationEffects: 20000, MaxAuthorizationAudit: 4096,
	}
}

var (
	ErrInvalidLimits     = errors.New("paperd: invalid limits")
	ErrInvalidHandle     = errors.New("paperd: invalid handle")
	ErrWrongWorkspace    = errors.New("paperd: handle belongs to another workspace")
	ErrRevisionNotFound  = errors.New("paperd: revision not found")
	ErrCandidateNotFound = errors.New("paperd: candidate not found")
	ErrPlanNotFound      = errors.New("paperd: plan not found")
	ErrPlanExpired       = errors.New("paperd: plan handle expired")
	ErrRevisionConflict  = errors.New("paperd: revision conflict")
	ErrInvalidSource     = errors.New("paperd: source is invalid")
	ErrLimit             = errors.New("paperd: limit exceeded")
	ErrInvalidQuery      = errors.New("paperd: invalid query")
	ErrHandleExpired     = errors.New("paperd: handle expired")
	ErrHandleRevoked     = errors.New("paperd: handle revoked")
	ErrDisclosureDenied  = errors.New("paperd: disclosure domain denied")
)

// Error is a stable machine-readable workspace error. Message contains no
// process-specific handle values, so equivalent failures are deterministic.
type Error struct {
	Code       string                      `json:"code"`
	Message    string                      `json:"message"`
	Candidates []paperedit.TargetCandidate `json:"candidates,omitempty"`
	cause      error
}

func (e *Error) Error() string { return "paperd: " + e.Code + ": " + e.Message }
func (e *Error) Unwrap() error { return e.cause }

func workspaceError(code, message string, cause error) error {
	return &Error{Code: code, Message: message, cause: cause}
}

type scopedHandle struct {
	scope      uint64
	serial     uint64
	nonce      uint64
	domain     uint64
	kind       handleKind
	capability handleCapability
}

// RevisionHandle is an opaque reference to one immutable source snapshot.
// Its fields are intentionally private; only the creating Workspace can open
// it. The zero value is invalid.
type RevisionHandle struct{ value scopedHandle }

// CandidateHandle is an opaque reference to one mutable head pointer. Source
// revisions referenced by the head remain immutable.
type CandidateHandle struct{ value scopedHandle }

// PlanHandle is an opaque reference to one immutable plan derived from one
// exact retained source revision.
type PlanHandle struct{ value scopedHandle }

// OpenHandle is an opaque, immutable capability pinned to one exact source
// revision. It never follows a candidate head implicitly.
type OpenHandle struct{ value scopedHandle }

// MutationAuthorityHandle is an operation- and node-scoped actor grant. It is
// separate from an edit-capable OpenHandle and cannot edit by itself.
type MutationAuthorityHandle struct{ value scopedHandle }

var nextWorkspaceScope atomic.Uint64

type revisionRecord struct {
	handle     RevisionHandle
	file       string
	source     string
	revision   paperedit.Revision
	parsed     paperlang.ParseResult
	compiled   papercompile.Result
	nodes      int
	expires    time.Time
	disclosure DisclosureDomain
	partition  cachePartition
}

type candidateRecord struct {
	handle      CandidateHandle
	head        RevisionHandle
	idempotency map[string]sourceIdempotencyRecord
	expires     time.Time
	disclosure  DisclosureDomain
	partition   cachePartition
}

type planRecord struct {
	handle     PlanHandle
	revision   RevisionHandle
	digest     paperedit.Revision
	plan       document.PaperPlan
	result     document.PaperPlanResult
	expires    time.Time
	disclosure DisclosureDomain
	partition  cachePartition
}

type openRecord struct {
	handle     OpenHandle
	candidate  CandidateHandle
	revision   RevisionHandle
	digest     paperedit.Revision
	mode       CapabilityMode
	expires    time.Time
	disclosure DisclosureDomain
	partition  cachePartition
}

type mutationAuthorityRecord struct {
	handle         MutationAuthorityHandle
	open           OpenHandle
	candidate      CandidateHandle
	actor          string
	operations     map[MutationOperation]struct{}
	nodeScopes     map[string]struct{}
	protectedNodes map[string]struct{}
	expires        time.Time
	disclosure     DisclosureDomain
	partition      cachePartition
}

// Workspace is safe for concurrent use. Parsing, compilation, editing, and
// rendering are performed without holding its state mutex; candidate commits
// use a compare-and-swap check when they publish a new immutable revision.
type Workspace struct {
	mu                       sync.RWMutex
	scope                    uint64
	limits                   Limits
	nextRevision             uint64
	nextCandidate            uint64
	nextPlan                 uint64
	nextOpen                 uint64
	nextMutationAuthority    uint64
	revisions                map[uint64]*revisionRecord
	candidates               map[uint64]*candidateRecord
	plans                    map[uint64]*planRecord
	opens                    map[uint64]*openRecord
	mutationAuthorities      map[uint64]*mutationAuthorityRecord
	planTTL                  time.Duration
	handleTTL                time.Duration
	now                      func() time.Time
	disclosureDomain         DisclosureDomain
	disclosureTag            uint64
	revocations              map[scopedHandle]revocationRecord
	revocationOrder          []scopedHandle
	projectID                string
	policyRevision           string
	partition                cachePartition
	disclosureAuditSink      func(DisclosureAuditEntry)
	disclosureAudit          []DisclosureAuditEntry
	nextDisclosureAudit      uint64
	requireMutationAuthority bool
	protectedNodeIDs         map[string]struct{}
	authorizationAudit       []AuthorizationAuditEntry
	nextAuthorizationAudit   uint64
	assetCatalog             papercompile.AssetCatalog
	importResolver           papercompile.ImportResolver
}

func NewWorkspace(limits Limits) (*Workspace, error) {
	return NewWorkspaceWithOptions(WorkspaceOptions{Limits: limits})
}

// WorkspaceOptions keeps lifecycle policy separate from deterministic source
// and plan inputs. Now is injectable for tests and embedders; it is never part
// of plan identity or layout output.
type WorkspaceOptions struct {
	Limits           Limits
	PlanTTL          time.Duration
	HandleTTL        time.Duration
	Now              func() time.Time
	DisclosureDomain DisclosureDomain
	ProjectID        string
	PolicyRevision   string
	// DisclosureAuditSink receives detached hash-only denial records. The
	// callback is best-effort and panic-isolated; raw disclosure labels,
	// capabilities, source, and payloads are never supplied to it.
	DisclosureAuditSink      func(DisclosureAuditEntry)
	RequireMutationAuthority bool
	ProtectedNodeIDs         []string
	// AssetResources is an explicit immutable catalog used only for semantic
	// compilation; the workspace never searches paths or the network.
	AssetResources []papercompile.AssetResource
	// ImportResolver is the explicit source boundary for reusable .paper
	// themes and styles. It is never inferred from ambient process state.
	ImportResolver papercompile.ImportResolver
}

func NewWorkspaceWithOptions(options WorkspaceOptions) (*Workspace, error) {
	limits := options.Limits
	normalized, err := normalizeLimits(limits)
	if err != nil {
		return nil, err
	}
	planTTL := options.PlanTTL
	if planTTL == 0 {
		planTTL = 30 * time.Minute
	}
	if planTTL < 0 || planTTL > MaxPlanTTLHard {
		return nil, workspaceError("INVALID_LIMITS", "PlanTTL must be positive and no greater than 24 hours", ErrInvalidLimits)
	}
	handleTTL := options.HandleTTL
	if handleTTL == 0 {
		handleTTL = 30 * time.Minute
	}
	if handleTTL < 0 || handleTTL > MaxHandleTTLHard {
		return nil, workspaceError("INVALID_LIMITS", "HandleTTL must be positive and no greater than 24 hours", ErrInvalidLimits)
	}
	disclosureDomain, disclosureTag, err := normalizeDisclosureDomain(options.DisclosureDomain)
	if err != nil {
		emitDisclosureAudit(options.DisclosureAuditSink, DisclosureAuditEntry{At: time.Now().UTC(), Action: "workspace.create", RequestedHash: disclosureIdentityHash(string(options.DisclosureDomain)), Reason: "invalid_disclosure"})
		return nil, err
	}
	projectID, policyRevision, partition, err := normalizeCachePartition(options.ProjectID, options.PolicyRevision, disclosureDomain)
	if err != nil {
		return nil, err
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	protectedNodeIDs, err := normalizeProtectedNodeIDs(options.ProtectedNodeIDs, normalized.MaxAuthorizationEffects)
	if err != nil {
		return nil, err
	}
	assetCatalog, err := papercompile.NewAssetCatalog(options.AssetResources)
	if err != nil {
		return nil, workspaceError("INVALID_ASSET_CATALOG", "workspace asset catalog is invalid", err)
	}
	scope := nextWorkspaceScope.Add(1)
	return &Workspace{
		scope: scope, limits: normalized, planTTL: planTTL, handleTTL: handleTTL, now: now,
		disclosureDomain: disclosureDomain, disclosureTag: disclosureTag,
		projectID: projectID, policyRevision: policyRevision, partition: partition,
		disclosureAuditSink:      options.DisclosureAuditSink,
		revisions:                make(map[uint64]*revisionRecord),
		candidates:               make(map[uint64]*candidateRecord),
		plans:                    make(map[uint64]*planRecord),
		opens:                    make(map[uint64]*openRecord),
		mutationAuthorities:      make(map[uint64]*mutationAuthorityRecord),
		revocations:              make(map[scopedHandle]revocationRecord),
		requireMutationAuthority: options.RequireMutationAuthority || len(protectedNodeIDs) != 0,
		protectedNodeIDs:         protectedNodeIDs,
		assetCatalog:             assetCatalog,
		importResolver:           options.ImportResolver,
	}, nil
}

func normalizeLimits(limits Limits) (Limits, error) {
	defaults := DefaultLimits()
	values := []*int{
		&limits.MaxSourceBytes, &limits.MaxRevisions, &limits.MaxCandidates, &limits.MaxPlans, &limits.MaxOpenDocuments,
		&limits.MaxNodes, &limits.MaxSearchResults, &limits.MaxQueryBytes,
		&limits.MaxRenderBytes, &limits.MaxFileBytes, &limits.MaxOperations,
		&limits.MaxContextBytes, &limits.MaxStructuralWork, &limits.MaxRevocations,
		&limits.MaxMutationAuthorities, &limits.MaxAuthorizationEffects, &limits.MaxAuthorizationAudit,
	}
	defaultValues := []int{
		defaults.MaxSourceBytes, defaults.MaxRevisions, defaults.MaxCandidates, defaults.MaxPlans, defaults.MaxOpenDocuments,
		defaults.MaxNodes, defaults.MaxSearchResults, defaults.MaxQueryBytes,
		defaults.MaxRenderBytes, defaults.MaxFileBytes, defaults.MaxOperations,
		defaults.MaxContextBytes, defaults.MaxStructuralWork, defaults.MaxRevocations,
		defaults.MaxMutationAuthorities, defaults.MaxAuthorizationEffects, defaults.MaxAuthorizationAudit,
	}
	maximums := []int{
		MaxSourceBytesHard, MaxRevisionsHard, MaxCandidatesHard, MaxPlansHard, MaxOpenDocumentsHard, MaxNodesHard,
		MaxSearchResultsHard, MaxQueryBytesHard, MaxRenderBytesHard,
		MaxFileBytesHard, paperedit.MaxOperations,
		MaxContextBytesHard, MaxStructuralWorkHard, MaxRevocationsHard,
		MaxMutationAuthoritiesHard, MaxAuthorizationEffectsHard, MaxAuthorizationAuditHard,
	}
	names := []string{
		"MaxSourceBytes", "MaxRevisions", "MaxCandidates", "MaxPlans", "MaxOpenDocuments", "MaxNodes",
		"MaxSearchResults", "MaxQueryBytes", "MaxRenderBytes", "MaxFileBytes",
		"MaxOperations", "MaxContextBytes", "MaxStructuralWork", "MaxRevocations",
		"MaxMutationAuthorities", "MaxAuthorizationEffects", "MaxAuthorizationAudit",
	}
	for index, value := range values {
		if *value == 0 {
			*value = defaultValues[index]
		}
		if *value < 1 || *value > maximums[index] {
			return Limits{}, workspaceError("INVALID_LIMITS", fmt.Sprintf("%s must be between 1 and %d", names[index], maximums[index]), ErrInvalidLimits)
		}
	}
	return limits, nil
}

// CreateRevision parses and retains an immutable copy of source. Invalid
// syntax is still a useful agent revision and is retained with diagnostics;
// Compile and Render reject it until an edit produces a valid candidate.
func (w *Workspace) CreateRevision(file, source string) (RevisionSnapshot, error) {
	if w == nil {
		return RevisionSnapshot{}, workspaceError("INVALID_WORKSPACE", "workspace is nil", ErrInvalidHandle)
	}
	record, err := w.prepareRevision(file, source)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pruneExpiredHandlesLocked(w.now())
	if len(w.revisions) >= w.limits.MaxRevisions {
		return RevisionSnapshot{}, workspaceError("REVISION_LIMIT", "workspace revision capacity is exhausted", ErrLimit)
	}
	w.nextRevision++
	record.handle = RevisionHandle{value: w.newHandle(handleRevision, capabilityRead, w.nextRevision)}
	record.expires = w.expiresAt(w.handleTTL)
	record.disclosure = w.disclosureDomain
	record.partition = w.partition
	w.revisions[w.nextRevision] = record
	return snapshotOf(record), nil
}

func (w *Workspace) prepareRevision(file, source string) (*revisionRecord, error) {
	if len(file) == 0 || len(file) > w.limits.MaxFileBytes {
		return nil, workspaceError("FILE_LIMIT", "file name must be non-empty and within the configured byte limit", ErrLimit)
	}
	if len(source) > w.limits.MaxSourceBytes {
		return nil, workspaceError("SOURCE_LIMIT", "source exceeds the configured byte limit", ErrLimit)
	}
	parsed := paperlang.Parse(file, source)
	nodes := countASTNodes(parsed.AST.Root, w.limits.MaxNodes+1)
	if nodes > w.limits.MaxNodes {
		return nil, workspaceError("NODE_LIMIT", "source exceeds the configured syntax-node limit", ErrLimit)
	}
	record := &revisionRecord{
		file: file, source: source, revision: paperedit.SourceRevision(source),
		parsed: parsed, nodes: nodes,
	}
	if parsed.OK() {
		record.compiled = papercompile.CompileWithAssetsAndResolver(parsed.AST, w.assetCatalog, w.importResolver)
	}
	return record, nil
}

// OpenRevision returns a detached snapshot; changing its diagnostic slices
// cannot mutate retained workspace state.
func (w *Workspace) OpenRevision(handle RevisionHandle) (RevisionSnapshot, error) {
	record, err := w.revision(handle)
	if err != nil {
		return RevisionSnapshot{}, err
	}
	return snapshotOf(record), nil
}

func (w *Workspace) NewCandidate(base RevisionHandle) (CandidateSnapshot, error) {
	if _, err := w.revision(base); err != nil {
		return CandidateSnapshot{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pruneExpiredHandlesLocked(w.now())
	if _, err := w.revisionLocked(base); err != nil {
		return CandidateSnapshot{}, err
	}
	if len(w.candidates) >= w.limits.MaxCandidates {
		return CandidateSnapshot{}, workspaceError("CANDIDATE_LIMIT", "workspace candidate capacity is exhausted", ErrLimit)
	}
	w.nextCandidate++
	handle := CandidateHandle{value: w.newHandle(handleCandidate, capabilityEdit, w.nextCandidate)}
	record := &candidateRecord{handle: handle, head: base, idempotency: make(map[string]sourceIdempotencyRecord), expires: w.expiresAt(w.handleTTL), disclosure: w.disclosureDomain, partition: w.partition}
	w.candidates[w.nextCandidate] = record
	return snapshotCandidate(record), nil
}

func (w *Workspace) Candidate(handle CandidateHandle) (CandidateSnapshot, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	record, err := w.candidateLocked(handle)
	if err != nil {
		return CandidateSnapshot{}, err
	}
	if _, err := w.revisionLocked(record.head); err != nil {
		return CandidateSnapshot{}, err
	}
	return snapshotCandidate(record), nil
}

// ApplyRequest requires both the expected opaque head and its exact SHA-256
// source revision. This prevents stale or accidentally cross-document edits.
type ApplyRequest struct {
	Candidate           CandidateHandle
	ExpectedHead        RevisionHandle
	ExpectedRevision    paperedit.Revision
	IdempotencyKey      string
	TargetPreconditions []paperedit.TargetPrecondition
	Operations          []paperedit.Operation
}

type ApplyResult struct {
	Candidate CandidateSnapshot
	Revision  RevisionSnapshot
	Edit      paperedit.Result
}

func (w *Workspace) Apply(request ApplyRequest) (ApplyResult, error) {
	if len(request.Operations) == 0 || len(request.Operations) > w.limits.MaxOperations {
		return ApplyResult{}, workspaceError("OPERATION_LIMIT", "operation count is outside the configured bounds", ErrLimit)
	}
	operations := cloneOperations(request.Operations)
	preconditions := append([]paperedit.TargetPrecondition(nil), request.TargetPreconditions...)
	fingerprint, err := sourceApplyFingerprint(request, operations, preconditions)
	if err != nil {
		return ApplyResult{}, err
	}
	w.mu.RLock()
	candidate, err := w.candidateLocked(request.Candidate)
	if err != nil {
		w.mu.RUnlock()
		return ApplyResult{}, err
	}
	if cached, exists := candidate.idempotency[request.IdempotencyKey]; request.IdempotencyKey != "" && exists {
		w.mu.RUnlock()
		return sourceCachedResult(cached, fingerprint)
	}
	if candidate.head != request.ExpectedHead {
		w.mu.RUnlock()
		return ApplyResult{}, workspaceError("REVISION_CONFLICT", "candidate head changed", ErrRevisionConflict)
	}
	base, err := w.revisionLocked(request.ExpectedHead)
	if err != nil {
		w.mu.RUnlock()
		return ApplyResult{}, err
	}
	file, source, revision := base.file, base.source, base.revision
	w.mu.RUnlock()
	if request.ExpectedRevision != revision {
		return ApplyResult{}, workspaceError("REVISION_CONFLICT", "exact source revision does not match the candidate head", ErrRevisionConflict)
	}

	edit, editErr := paperedit.Apply(paperedit.Transaction{
		File: file, Source: source, ExpectedRevision: revision,
		IdempotencyKey: request.IdempotencyKey, TargetPreconditions: preconditions,
		RequireExactTargets: true, Operations: operations,
	})
	if editErr != nil {
		return ApplyResult{Edit: cloneEditResult(edit)}, wrapEditError(editErr)
	}
	prepared, err := w.prepareRevision(file, edit.Source)
	if err != nil {
		return ApplyResult{Edit: cloneEditResult(edit)}, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.pruneExpiredHandlesLocked(w.now())
	candidate, err = w.candidateLocked(request.Candidate)
	if err != nil {
		return ApplyResult{Edit: cloneEditResult(edit)}, err
	}
	if cached, exists := candidate.idempotency[request.IdempotencyKey]; request.IdempotencyKey != "" && exists {
		return sourceCachedResult(cached, fingerprint)
	}
	if candidate.head != request.ExpectedHead {
		return ApplyResult{Edit: cloneEditResult(edit)}, workspaceError("REVISION_CONFLICT", "candidate head changed", ErrRevisionConflict)
	}
	if !edit.Applied || edit.Diff == nil {
		return ApplyResult{Candidate: snapshotCandidate(candidate), Revision: snapshotOf(base), Edit: cloneEditResult(edit)}, nil
	}
	if len(w.revisions) >= w.limits.MaxRevisions {
		return ApplyResult{Edit: cloneEditResult(edit)}, workspaceError("REVISION_LIMIT", "workspace revision capacity is exhausted", ErrLimit)
	}
	w.nextRevision++
	prepared.handle = RevisionHandle{value: w.newHandle(handleRevision, capabilityRead, w.nextRevision)}
	prepared.expires = w.expiresAt(w.handleTTL)
	prepared.disclosure = w.disclosureDomain
	prepared.partition = w.partition
	w.revisions[w.nextRevision] = prepared
	candidate.head = prepared.handle
	candidate.clearHeadCachesLocked()
	result := ApplyResult{
		Candidate: snapshotCandidate(candidate),
		Revision:  snapshotOf(prepared), Edit: cloneEditResult(edit),
	}
	if request.IdempotencyKey != "" {
		candidate.idempotency[request.IdempotencyKey] = sourceIdempotencyRecord{fingerprint: fingerprint, result: cloneApplyResult(result)}
	}
	return cloneApplyResult(result), nil
}

func (candidate *candidateRecord) clearHeadCachesLocked() {
	candidate.idempotency = make(map[string]sourceIdempotencyRecord)
}

func wrapEditError(err error) error {
	switch {
	case errors.Is(err, paperedit.ErrRevisionConflict):
		return workspaceError("REVISION_CONFLICT", "edit source revision changed", ErrRevisionConflict)
	case errors.Is(err, paperedit.ErrLimit):
		return workspaceError("EDIT_LIMIT", "edit exceeds a transactional limit", ErrLimit)
	case errors.Is(err, paperedit.ErrInvalidSource):
		return workspaceError("INVALID_SOURCE", "candidate source has parse errors", ErrInvalidSource)
	case errors.Is(err, paperedit.ErrCandidateInvalid):
		return workspaceError("INVALID_CANDIDATE", "edit would produce invalid source", ErrInvalidSource)
	default:
		return workspaceError("EDIT_REJECTED", "edit transaction was rejected", err)
	}
}

func (w *Workspace) revision(handle RevisionHandle) (*revisionRecord, error) {
	if w == nil {
		return nil, workspaceError("INVALID_WORKSPACE", "workspace is nil", ErrInvalidHandle)
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.revisionLocked(handle)
}

func (w *Workspace) revisionLocked(handle RevisionHandle) (*revisionRecord, error) {
	if err := w.validateHandle(handle.value, handleRevision, capabilityRead, false); err != nil {
		return nil, err
	}
	record := w.revisions[handle.value.serial]
	if record == nil || record.handle != handle || !w.ownsPartition(record.partition) {
		return nil, w.unavailableHandle(handle.value, ErrRevisionNotFound)
	}
	if err := w.ensureLive(handle.value, record.expires); err != nil {
		return nil, err
	}
	return record, nil
}

func (w *Workspace) candidateLocked(handle CandidateHandle) (*candidateRecord, error) {
	if err := w.validateHandle(handle.value, handleCandidate, capabilityEdit, false); err != nil {
		return nil, err
	}
	record := w.candidates[handle.value.serial]
	if record == nil || record.handle != handle || !w.ownsPartition(record.partition) {
		return nil, w.unavailableHandle(handle.value, ErrCandidateNotFound)
	}
	if err := w.ensureLive(handle.value, record.expires); err != nil {
		return nil, err
	}
	return record, nil
}

func (w *Workspace) plan(handle PlanHandle) (*planRecord, error) {
	if w == nil {
		return nil, workspaceError("INVALID_WORKSPACE", "workspace is nil", ErrInvalidHandle)
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.planLocked(handle)
}

func (w *Workspace) planLocked(handle PlanHandle) (*planRecord, error) {
	if err := w.validateHandle(handle.value, handlePlan, capabilityRender, false); err != nil {
		return nil, err
	}
	record := w.plans[handle.value.serial]
	if record == nil || record.handle != handle || !w.ownsPartition(record.partition) {
		return nil, w.unavailableHandle(handle.value, ErrPlanNotFound)
	}
	if !record.expires.After(w.now()) {
		return nil, workspaceError("PLAN_EXPIRED", "handle is unavailable", ErrPlanExpired)
	}
	return record, nil
}

func countASTNodes(root *paperlang.Node, stop int) int {
	count := 0
	var walk func(*paperlang.Node)
	walk = func(node *paperlang.Node) {
		if node == nil || count >= stop {
			return
		}
		count++
		for _, member := range node.Members {
			if member.Property != nil {
				count++
			}
			walk(member.Node)
			if count >= stop {
				return
			}
		}
	}
	walk(root)
	return count
}

func cloneOperations(operations []paperedit.Operation) []paperedit.Operation {
	cloned := make([]paperedit.Operation, len(operations))
	for index, operation := range operations {
		switch value := operation.(type) {
		case paperedit.SetProperty, paperedit.ReplaceText, paperedit.DeleteNode,
			paperedit.RenameID, paperedit.MoveNode:
			cloned[index] = value
		case paperedit.SetProperties:
			value.Properties = append([]paperedit.PropertySpec(nil), value.Properties...)
			cloned[index] = value
		case paperedit.AppendProperty:
			cloned[index] = value
		case paperedit.InsertNode:
			value.Node = cloneNodeSpec(value.Node)
			cloned[index] = value
		case paperedit.WrapNode:
			value.Wrapper = cloneNodeSpec(value.Wrapper)
			cloned[index] = value
		case paperedit.ReplaceNode:
			value.Node = cloneNodeSpec(value.Node)
			cloned[index] = value
		default:
			cloned[index] = operation
		}
	}
	return cloned
}

func cloneNodeSpec(spec paperedit.NodeSpec) paperedit.NodeSpec {
	cloned := spec
	if spec.Value != nil {
		value := *spec.Value
		cloned.Value = &value
	}
	cloned.Properties = append([]paperedit.PropertySpec(nil), spec.Properties...)
	if len(spec.Children) != 0 {
		cloned.Children = make([]paperedit.NodeSpec, len(spec.Children))
		for index, child := range spec.Children {
			cloned.Children[index] = cloneNodeSpec(child)
		}
	}
	return cloned
}

func cloneEditResult(result paperedit.Result) paperedit.Result {
	result.Diagnostics = append([]paperedit.Diagnostic(nil), result.Diagnostics...)
	if result.Diff != nil {
		diff := *result.Diff
		diff.Patches = append([]paperedit.SourcePatch(nil), result.Diff.Patches...)
		result.Diff = &diff
	}
	if result.Invalidation != nil {
		invalidation := *result.Invalidation
		invalidation.NodeIDs = append([]string(nil), result.Invalidation.NodeIDs...)
		result.Invalidation = &invalidation
	}
	return result
}
